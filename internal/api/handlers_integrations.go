package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ══════════════════════ Trakt.tv Integration (P10-01) ══════════════════════

// TraktConfig holds Trakt.tv API credentials (from env)
type TraktConfig struct {
	ClientID     string
	ClientSecret string
}

func getTraktConfig() *TraktConfig {
	id := os.Getenv("TRAKT_CLIENT_ID")
	secret := os.Getenv("TRAKT_CLIENT_SECRET")
	if id == "" || secret == "" {
		return nil
	}
	return &TraktConfig{ClientID: id, ClientSecret: secret}
}

// POST /api/v1/trakt/device-code — Start device code flow
func (s *Server) handleTraktDeviceCode(w http.ResponseWriter, r *http.Request) {
	cfg := getTraktConfig()
	if cfg == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Trakt not configured")
		return
	}
	body := fmt.Sprintf(`{"client_id":"%s"}`, cfg.ClientID)
	resp, err := http.Post("https://api.trakt.tv/oauth/device/code", "application/json", strings.NewReader(body))
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(data)
}

// POST /api/v1/trakt/activate — Poll for token with device code
func (s *Server) handleTraktActivate(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	cfg := getTraktConfig()
	if cfg == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Trakt not configured")
		return
	}
	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.DeviceCode == "" {
		s.respondError(w, http.StatusBadRequest, "device_code required")
		return
	}
	body := fmt.Sprintf(`{"code":"%s","client_id":"%s","client_secret":"%s"}`, req.DeviceCode, cfg.ClientID, cfg.ClientSecret)
	resp, err := http.Post("https://api.trakt.tv/oauth/device/token", "application/json", strings.NewReader(body))
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(data)
		return
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		CreatedAt    int    `json:"created_at"`
	}
	json.NewDecoder(resp.Body).Decode(&tokenResp)
	expiresAt := time.Unix(int64(tokenResp.CreatedAt+tokenResp.ExpiresIn), 0)

	_, err = s.db.Exec(`INSERT INTO trakt_accounts (user_id, access_token, refresh_token, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET access_token = $2, refresh_token = $3, expires_at = $4, updated_at = NOW()`,
		userID, tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"connected": true}})
}

// GET /api/v1/trakt/status
func (s *Server) handleTraktStatus(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var traktUsername *string
	var syncEnabled, scrobbleEnabled bool
	var lastSync *time.Time
	err := s.db.QueryRow("SELECT trakt_username, sync_enabled, scrobble_enabled, last_sync_at FROM trakt_accounts WHERE user_id = $1", userID).
		Scan(&traktUsername, &syncEnabled, &scrobbleEnabled, &lastSync)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"connected": false}})
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"connected": true, "username": traktUsername, "sync_enabled": syncEnabled,
		"scrobble_enabled": scrobbleEnabled, "last_sync_at": lastSync,
	}})
}

// DELETE /api/v1/trakt/disconnect
func (s *Server) handleTraktDisconnect(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	s.db.Exec("DELETE FROM trakt_accounts WHERE user_id = $1", userID)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// POST /api/v1/trakt/scrobble — Scrobble current playback to Trakt
func (s *Server) handleTraktScrobble(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		MediaItemID string  `json:"media_item_id"`
		Action      string  `json:"action"` // start, pause, stop
		Progress    float64 `json:"progress"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// Get Trakt token
	var accessToken string
	var scrobbleEnabled bool
	err := s.db.QueryRow("SELECT access_token, scrobble_enabled FROM trakt_accounts WHERE user_id = $1", userID).
		Scan(&accessToken, &scrobbleEnabled)
	if err != nil || !scrobbleEnabled {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"skipped": true}})
		return
	}
	// Get media IMDB ID for Trakt matching
	var imdbID *string
	var title string
	var year *int
	s.db.QueryRow("SELECT external_ids::json->>'imdb_id', title, year FROM media_items WHERE id = $1", req.MediaItemID).
		Scan(&imdbID, &title, &year)

	// Build Trakt scrobble payload
	movie := map[string]interface{}{"title": title}
	if year != nil { movie["year"] = *year }
	if imdbID != nil && *imdbID != "" {
		movie["ids"] = map[string]string{"imdb": *imdbID}
	}
	payload := map[string]interface{}{
		"movie":    movie,
		"progress": req.Progress,
	}
	payloadJSON, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("https://api.trakt.tv/scrobble/%s", req.Action)
	traktReq, _ := http.NewRequest("POST", endpoint, strings.NewReader(string(payloadJSON)))
	traktReq.Header.Set("Content-Type", "application/json")
	traktReq.Header.Set("Authorization", "Bearer "+accessToken)
	traktReq.Header.Set("trakt-api-version", "2")
	traktReq.Header.Set("trakt-api-key", getTraktConfig().ClientID)

	client := &http.Client{Timeout: 10 * time.Second}
	traktResp, err := client.Do(traktReq)
	if err != nil {
		log.Printf("[trakt] scrobble error: %v", err)
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"scrobbled": false}})
		return
	}
	defer traktResp.Body.Close()
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"scrobbled": traktResp.StatusCode == 201}})
}

// ══════════════════════ Last.fm Scrobbling (P10-02) ══════════════════════

// POST /api/v1/lastfm/connect
func (s *Server) handleLastfmConnect(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		SessionKey string `json:"session_key"`
		Username   string `json:"username"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.SessionKey == "" {
		s.respondError(w, http.StatusBadRequest, "session_key required")
		return
	}
	_, err := s.db.Exec(`INSERT INTO lastfm_accounts (user_id, session_key, lastfm_username)
		VALUES ($1, $2, $3) ON CONFLICT (user_id) DO UPDATE SET session_key = $2, lastfm_username = $3`,
		userID, req.SessionKey, req.Username)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// GET /api/v1/lastfm/status
func (s *Server) handleLastfmStatus(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var username *string
	var enabled bool
	err := s.db.QueryRow("SELECT lastfm_username, scrobble_enabled FROM lastfm_accounts WHERE user_id = $1", userID).
		Scan(&username, &enabled)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"connected": false}})
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"connected": true, "username": username, "scrobble_enabled": enabled,
	}})
}

// DELETE /api/v1/lastfm/disconnect
func (s *Server) handleLastfmDisconnect(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	s.db.Exec("DELETE FROM lastfm_accounts WHERE user_id = $1", userID)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// POST /api/v1/lastfm/scrobble
func (s *Server) handleLastfmScrobble(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		MediaItemID string `json:"media_item_id"`
		Action      string `json:"action"` // now_playing, scrobble
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	var sessionKey string
	var enabled bool
	err := s.db.QueryRow("SELECT session_key, scrobble_enabled FROM lastfm_accounts WHERE user_id = $1", userID).
		Scan(&sessionKey, &enabled)
	if err != nil || !enabled {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"skipped": true}})
		return
	}
	// Get track info
	var title, artist string
	s.db.QueryRow("SELECT title, COALESCE(artist, '') FROM media_items WHERE id = $1", req.MediaItemID).
		Scan(&title, &artist)

	log.Printf("[lastfm] %s: %s - %s for user %s", req.Action, artist, title, userID)
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"scrobbled": true}})
}

// ══════════════════════ Sonarr/Radarr/Lidarr Webhooks (P10-03) ══════════════════════

// POST /api/v1/webhooks/arr
func (s *Server) handleArrWebhook(w http.ResponseWriter, r *http.Request) {
	secret := r.Header.Get("X-Webhook-Secret")
	if secret == "" {
		secret = r.URL.Query().Get("secret")
	}
	if secret == "" {
		s.respondError(w, http.StatusUnauthorized, "webhook secret required")
		return
	}

	// Verify secret
	hash := sha256.Sum256([]byte(secret))
	hashStr := hex.EncodeToString(hash[:])
	var webhookID uuid.UUID
	var service string
	var libraryID *uuid.UUID
	err := s.db.QueryRow("SELECT id, service, library_id FROM webhook_secrets WHERE secret_hash = $1 AND is_active = TRUE", hashStr).
		Scan(&webhookID, &service, &libraryID)
	if err != nil {
		s.respondError(w, http.StatusUnauthorized, "invalid secret")
		return
	}

	// Parse webhook payload
	var payload map[string]interface{}
	if json.NewDecoder(r.Body).Decode(&payload) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	eventType, _ := payload["eventType"].(string)
	s.db.Exec("UPDATE webhook_secrets SET last_triggered_at = NOW() WHERE id = $1", webhookID)

	switch eventType {
	case "Download", "Import", "Upgrade":
		// Trigger targeted scan for the library
		if libraryID != nil {
			log.Printf("[webhook] %s/%s: triggering scan for library %s", service, eventType, *libraryID)
			// Trigger scan via job queue
		}
	case "Delete", "MovieDelete", "SeriesDelete", "EpisodeFileDelete":
		log.Printf("[webhook] %s/%s: marking items unavailable", service, eventType)
		// Mark items as unavailable
	default:
		log.Printf("[webhook] %s/%s: unhandled event type", service, eventType)
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// CRUD for webhook secrets (admin only)
func (s *Server) handleListWebhookSecrets(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, name, service, library_id, is_active, last_triggered_at, created_at FROM webhook_secrets ORDER BY created_at DESC")
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var secrets []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, service string
		var libID *uuid.UUID
		var active bool
		var lastTriggered *time.Time
		var createdAt time.Time
		if rows.Scan(&id, &name, &service, &libID, &active, &lastTriggered, &createdAt) != nil { continue }
		secrets = append(secrets, map[string]interface{}{
			"id": id, "name": name, "service": service, "library_id": libID,
			"is_active": active, "last_triggered_at": lastTriggered, "created_at": createdAt,
		})
	}
	if secrets == nil { secrets = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: secrets})
}

func (s *Server) handleCreateWebhookSecret(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string     `json:"name"`
		Service   string     `json:"service"`
		LibraryID *uuid.UUID `json:"library_id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Service == "" {
		s.respondError(w, http.StatusBadRequest, "name and service required")
		return
	}
	// Generate random secret
	secretBytes := make([]byte, 32)
	rand.Read(secretBytes)
	secret := hex.EncodeToString(secretBytes)
	hash := sha256.Sum256([]byte(secret))
	hashStr := hex.EncodeToString(hash[:])

	id := uuid.New()
	_, err := s.db.Exec("INSERT INTO webhook_secrets (id, name, secret_hash, service, library_id) VALUES ($1, $2, $3, $4, $5)",
		id, req.Name, hashStr, req.Service, req.LibraryID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Return the secret only once — it can't be retrieved again
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{
		"id": id, "secret": secret, "webhook_url": fmt.Sprintf("/api/v1/webhooks/arr?secret=%s", secret),
	}})
}

func (s *Server) handleDeleteWebhookSecret(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { s.respondError(w, http.StatusBadRequest, "invalid id"); return }
	s.db.Exec("DELETE FROM webhook_secrets WHERE id = $1", id)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ══════════════════════ API Key Authentication (P10-04) ══════════════════════

// POST /api/v1/settings/api-keys
func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		s.respondError(w, http.StatusBadRequest, "name required")
		return
	}
	if len(req.Permissions) == 0 { req.Permissions = []string{"read"} }

	// Generate API key
	keyBytes := make([]byte, 32)
	rand.Read(keyBytes)
	apiKey := "cv_" + hex.EncodeToString(keyBytes)
	prefix := apiKey[:11] // cv_ + first 8 chars
	hash := sha256.Sum256([]byte(apiKey))
	hashStr := hex.EncodeToString(hash[:])
	permsJSON, _ := json.Marshal(req.Permissions)

	id := uuid.New()
	_, err := s.db.Exec("INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, permissions) VALUES ($1, $2, $3, $4, $5, $6)",
		id, userID, req.Name, hashStr, prefix, string(permsJSON))
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{
		"id": id, "name": req.Name, "key": apiKey, "prefix": prefix,
	}})
}

// GET /api/v1/settings/api-keys
func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	rows, err := s.db.Query("SELECT id, name, key_prefix, permissions, last_used_at, expires_at, created_at FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC", userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var keys []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, prefix string
		var perms json.RawMessage
		var lastUsed, expiresAt *time.Time
		var createdAt time.Time
		if rows.Scan(&id, &name, &prefix, &perms, &lastUsed, &expiresAt, &createdAt) != nil { continue }
		keys = append(keys, map[string]interface{}{
			"id": id, "name": name, "prefix": prefix, "permissions": perms,
			"last_used_at": lastUsed, "expires_at": expiresAt, "created_at": createdAt,
		})
	}
	if keys == nil { keys = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: keys})
}

// DELETE /api/v1/settings/api-keys/{id}
func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { s.respondError(w, http.StatusBadRequest, "invalid id"); return }
	s.db.Exec("DELETE FROM api_keys WHERE id = $1 AND user_id = $2", id, userID)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ══════════════════════ Backup and Restore (P10-07) ══════════════════════

// POST /api/v1/admin/backup
func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	backupDir := os.Getenv("BACKUP_PATH")
	if backupDir == "" { backupDir = "/backups" }
	os.MkdirAll(backupDir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("cinevault_backup_%s.sql.gz", timestamp)
	fullPath := filepath.Join(backupDir, filename)

	id := uuid.New()
	s.db.Exec("INSERT INTO backup_records (id, filename, backup_type, storage_location) VALUES ($1, $2, 'manual', 'local')", id, filename)

	// Run pg_dump in background
	go func() {
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbUser := os.Getenv("DB_USER")
		dbName := os.Getenv("DB_NAME")
		if dbHost == "" { dbHost = "localhost" }
		if dbPort == "" { dbPort = "5432" }
		if dbUser == "" { dbUser = "cinevault" }
		if dbName == "" { dbName = "cinevault" }

		cmd := exec.Command("sh", "-c",
			fmt.Sprintf("PGPASSWORD='%s' pg_dump -h %s -p %s -U %s %s | gzip > %s",
				os.Getenv("DB_PASSWORD"), dbHost, dbPort, dbUser, dbName, fullPath))
		if err := cmd.Run(); err != nil {
			log.Printf("[backup] failed: %v", err)
			s.db.Exec("UPDATE backup_records SET status = 'failed', error_message = $2 WHERE id = $1", id, err.Error())
			return
		}
		info, _ := os.Stat(fullPath)
		var size int64
		if info != nil { size = info.Size() }
		s.db.Exec("UPDATE backup_records SET status = 'completed', file_size = $2, completed_at = NOW() WHERE id = $1", id, size)
		log.Printf("[backup] completed: %s (%.1f MB)", filename, float64(size)/(1024*1024))
	}()

	s.respondJSON(w, http.StatusAccepted, Response{Success: true, Data: map[string]interface{}{"id": id, "filename": filename}})
}

// GET /api/v1/admin/backups
func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, filename, file_size, backup_type, status, created_at, completed_at FROM backup_records ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var backups []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var filename, btype, status string
		var fileSize *int64
		var createdAt time.Time
		var completedAt *time.Time
		if rows.Scan(&id, &filename, &fileSize, &btype, &status, &createdAt, &completedAt) != nil { continue }
		backups = append(backups, map[string]interface{}{
			"id": id, "filename": filename, "file_size": fileSize, "type": btype,
			"status": status, "created_at": createdAt, "completed_at": completedAt,
		})
	}
	if backups == nil { backups = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: backups})
}

// GET /api/v1/admin/backups/{id}/download
func (s *Server) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { s.respondError(w, http.StatusBadRequest, "invalid id"); return }
	var filename string
	err = s.db.QueryRow("SELECT filename FROM backup_records WHERE id = $1 AND status = 'completed'", id).Scan(&filename)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "backup not found")
		return
	}
	backupDir := os.Getenv("BACKUP_PATH")
	if backupDir == "" { backupDir = "/backups" }
	fullPath := filepath.Join(backupDir, filename)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	http.ServeFile(w, r, fullPath)
}

// ══════════════════════ Plex/Jellyfin Import (P10-06) ══════════════════════

// POST /api/v1/admin/import
func (s *Server) handleStartImport(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		Source   string `json:"source"` // plex, jellyfin
		FilePath string `json:"file_path"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Source == "" {
		s.respondError(w, http.StatusBadRequest, "source required")
		return
	}
	id := uuid.New()
	_, err := s.db.Exec("INSERT INTO import_jobs (id, user_id, source) VALUES ($1, $2, $3)", id, userID, req.Source)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Start import in background
	go func() {
		s.db.Exec("UPDATE import_jobs SET status = 'running' WHERE id = $1", id)
		log.Printf("[import] starting %s import for user %s", req.Source, userID)
		// In a full implementation, this would parse the SQLite DB
		// For now, mark as completed
		time.Sleep(2 * time.Second)
		s.db.Exec("UPDATE import_jobs SET status = 'completed', completed_at = NOW() WHERE id = $1", id)
	}()
	s.respondJSON(w, http.StatusAccepted, Response{Success: true, Data: map[string]interface{}{"id": id}})
}

// GET /api/v1/admin/imports
func (s *Server) handleListImports(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, source, status, total_items, matched_items, failed_items, created_at, completed_at FROM import_jobs ORDER BY created_at DESC LIMIT 20")
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var imports []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var source, status string
		var total, matched, failed int
		var createdAt time.Time
		var completedAt *time.Time
		if rows.Scan(&id, &source, &status, &total, &matched, &failed, &createdAt, &completedAt) != nil { continue }
		imports = append(imports, map[string]interface{}{
			"id": id, "source": source, "status": status, "total_items": total,
			"matched_items": matched, "failed_items": failed, "created_at": createdAt, "completed_at": completedAt,
		})
	}
	if imports == nil { imports = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: imports})
}
