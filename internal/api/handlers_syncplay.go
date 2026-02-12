package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ══════════════════════ Watch Together / SyncPlay (P12-01) ══════════════════════

// POST /api/v1/sync/create — Host creates a sync session
func (s *Server) handleSyncCreate(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		MediaItemID string `json:"media_item_id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.MediaItemID == "" {
		s.respondError(w, http.StatusBadRequest, "media_item_id required")
		return
	}

	// Generate invite code
	codeBytes := make([]byte, 4)
	rand.Read(codeBytes)
	code := hex.EncodeToString(codeBytes)

	id := uuid.New()
	_, err := s.db.Exec(`INSERT INTO sync_sessions (id, host_user_id, invite_code, media_item_id) VALUES ($1, $2, $3, $4)`,
		id, userID, code, req.MediaItemID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Add host as participant
	s.db.Exec("INSERT INTO sync_participants (session_id, user_id) VALUES ($1, $2)", id, userID)

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{
		"session_id": id, "invite_code": code,
	}})
}

// POST /api/v1/sync/join — Join a session by invite code
func (s *Server) handleSyncJoin(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		InviteCode string `json:"invite_code"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.InviteCode == "" {
		s.respondError(w, http.StatusBadRequest, "invite_code required")
		return
	}

	var sessionID uuid.UUID
	var mediaItemID uuid.UUID
	var state string
	var currentTime float64
	var maxParticipants int
	err := s.db.QueryRow(`SELECT id, media_item_id, state, current_time_sec, max_participants
		FROM sync_sessions WHERE invite_code = $1 AND state != 'ended'`, req.InviteCode).
		Scan(&sessionID, &mediaItemID, &state, &currentTime, &maxParticipants)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "session not found or ended")
		return
	}

	// Check participant count
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM sync_participants WHERE session_id = $1", sessionID).Scan(&count)
	if count >= maxParticipants {
		s.respondError(w, http.StatusConflict, "session is full")
		return
	}

	s.db.Exec("INSERT INTO sync_participants (session_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", sessionID, userID)

	// Broadcast join event
	s.wsHub.Broadcast("sync:join", map[string]interface{}{
		"session_id": sessionID, "user_id": userID,
	})

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"session_id": sessionID, "media_item_id": mediaItemID,
		"state": state, "current_time": currentTime,
	}})
}

// POST /api/v1/sync/{id}/action — Host sends play/pause/seek action
func (s *Server) handleSyncAction(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	// Verify host
	var hostID string
	s.db.QueryRow("SELECT host_user_id FROM sync_sessions WHERE id = $1", sessionID).Scan(&hostID)
	if hostID != userID.String() {
		s.respondError(w, http.StatusForbidden, "only host can control playback")
		return
	}

	var req struct {
		Action      string  `json:"action"` // play, pause, seek
		CurrentTime float64 `json:"current_time"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}

	// Update session state
	newState := "playing"
	if req.Action == "pause" {
		newState = "paused"
	}
	s.db.Exec("UPDATE sync_sessions SET state = $1, current_time_sec = $2, updated_at = NOW() WHERE id = $3",
		newState, req.CurrentTime, sessionID)

	// Broadcast to all participants
	s.wsHub.Broadcast("sync:action", map[string]interface{}{
		"session_id":   sessionID,
		"action":       req.Action,
		"current_time": req.CurrentTime,
		"from_user":    userID,
	})

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// POST /api/v1/sync/{id}/chat — Send chat message
func (s *Server) handleSyncChat(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Message == "" {
		s.respondError(w, http.StatusBadRequest, "message required")
		return
	}

	s.db.Exec("INSERT INTO sync_chat (session_id, user_id, message) VALUES ($1, $2, $3)", sessionID, userID, req.Message)

	// Get display name
	var displayName string
	s.db.QueryRow("SELECT COALESCE(display_name, username) FROM users WHERE id = $1", userID).Scan(&displayName)

	s.wsHub.Broadcast("sync:chat", map[string]interface{}{
		"session_id": sessionID, "user": displayName, "message": req.Message,
		"timestamp": time.Now(),
	})

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// GET /api/v1/sync/{id} — Get session info
func (s *Server) handleSyncInfo(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	var hostID uuid.UUID
	var inviteCode, state string
	var mediaItemID uuid.UUID
	var currentTime float64
	err = s.db.QueryRow("SELECT host_user_id, invite_code, media_item_id, state, current_time_sec FROM sync_sessions WHERE id = $1", sessionID).
		Scan(&hostID, &inviteCode, &mediaItemID, &state, &currentTime)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "session not found")
		return
	}

	// Get participants
	rows, _ := s.db.Query(`SELECT u.id, COALESCE(u.display_name, u.username) FROM sync_participants sp
		JOIN users u ON sp.user_id = u.id WHERE sp.session_id = $1`, sessionID)
	var participants []map[string]interface{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var uid uuid.UUID
			var name string
			if rows.Scan(&uid, &name) == nil {
				participants = append(participants, map[string]interface{}{"id": uid, "name": name})
			}
		}
	}

	// Get recent chat
	chatRows, _ := s.db.Query(`SELECT u.username, sc.message, sc.created_at FROM sync_chat sc
		JOIN users u ON sc.user_id = u.id WHERE sc.session_id = $1 ORDER BY sc.created_at DESC LIMIT 50`, sessionID)
	var chat []map[string]interface{}
	if chatRows != nil {
		defer chatRows.Close()
		for chatRows.Next() {
			var user, msg string
			var ts time.Time
			if chatRows.Scan(&user, &msg, &ts) == nil {
				chat = append(chat, map[string]interface{}{"user": user, "message": msg, "timestamp": ts})
			}
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"id": sessionID, "host_id": hostID, "invite_code": inviteCode, "media_item_id": mediaItemID,
		"state": state, "current_time": currentTime, "participants": participants, "chat": chat,
	}})
}

// DELETE /api/v1/sync/{id} — End session
func (s *Server) handleSyncEnd(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	s.db.Exec("UPDATE sync_sessions SET state = 'ended', updated_at = NOW() WHERE id = $1 AND host_user_id = $2", sessionID, userID)
	s.wsHub.Broadcast("sync:ended", map[string]interface{}{"session_id": sessionID})
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ══════════════════════ Cinema Mode (P12-02) ══════════════════════

// GET /api/v1/cinema/pre-rolls
func (s *Server) handleListPreRolls(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, name, file_path, duration_seconds, is_active, sort_order FROM pre_roll_videos ORDER BY sort_order")
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var prerolls []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, filePath string
		var duration *int
		var active bool
		var sortOrder int
		if rows.Scan(&id, &name, &filePath, &duration, &active, &sortOrder) != nil {
			continue
		}
		prerolls = append(prerolls, map[string]interface{}{
			"id": id, "name": name, "file_path": filePath, "duration_seconds": duration,
			"is_active": active, "sort_order": sortOrder,
		})
	}
	if prerolls == nil {
		prerolls = []map[string]interface{}{}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: prerolls})
}

// POST /api/v1/cinema/pre-rolls
func (s *Server) handleCreatePreRoll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		FilePath string `json:"file_path"`
		Duration *int   `json:"duration_seconds"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.FilePath == "" {
		s.respondError(w, http.StatusBadRequest, "name and file_path required")
		return
	}
	id := uuid.New()
	_, err := s.db.Exec("INSERT INTO pre_roll_videos (id, name, file_path, duration_seconds) VALUES ($1, $2, $3, $4)",
		id, req.Name, req.FilePath, req.Duration)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{"id": id}})
}

// DELETE /api/v1/cinema/pre-rolls/{id}
func (s *Server) handleDeletePreRoll(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	s.db.Exec("DELETE FROM pre_roll_videos WHERE id = $1", id)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// GET /api/v1/cinema/queue/{mediaId} — Build cinema queue (pre-rolls + trailers + main)
func (s *Server) handleCinemaQueue(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("mediaId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	var queue []map[string]interface{}

	// Get pre-roll count from settings
	prerollCount := 1
	var countStr string
	if s.db.QueryRow("SELECT value FROM settings WHERE key = 'cinema_trailer_count'").Scan(&countStr) == nil {
		var c int
		if _, err := fmt.Sscanf(countStr, "%d", &c); err == nil && c >= 0 && c <= 5 {
			prerollCount = c
		}
	}

	// Get active pre-rolls
	if prerollCount > 0 {
		randomize := false
		var rStr string
		if s.db.QueryRow("SELECT value FROM settings WHERE key = 'cinema_randomize'").Scan(&rStr) == nil {
			randomize = rStr == "true"
		}
		orderClause := "sort_order"
		if randomize {
			orderClause = "RANDOM()"
		}
		prRows, err := s.db.Query(fmt.Sprintf("SELECT id, name, file_path, duration_seconds FROM pre_roll_videos WHERE is_active = TRUE ORDER BY %s LIMIT $1", orderClause), prerollCount)
		if err == nil {
			defer prRows.Close()
			for prRows.Next() {
				var pid uuid.UUID
				var name, filePath string
				var dur *int
				if prRows.Scan(&pid, &name, &filePath, &dur) == nil {
					queue = append(queue, map[string]interface{}{
						"type": "pre_roll", "id": pid, "title": name, "file_path": filePath, "duration": dur,
					})
				}
			}
		}
	}

	// Get library trailers (extras of type 'trailer' for this item)
	trRows, _ := s.db.Query(`SELECT id, title, file_path, duration_seconds FROM media_items
		WHERE parent_media_id = $1 AND extra_type = 'trailer' ORDER BY title LIMIT 3`, mediaID)
	if trRows != nil {
		defer trRows.Close()
		for trRows.Next() {
			var tid uuid.UUID
			var title, filePath string
			var dur *int
			if trRows.Scan(&tid, &title, &filePath, &dur) == nil {
				queue = append(queue, map[string]interface{}{
					"type": "trailer", "id": tid, "title": title, "file_path": filePath, "duration": dur,
				})
			}
		}
	}

	// Add main feature
	var title, filePath string
	var dur *int
	s.db.QueryRow("SELECT title, file_path, duration_seconds FROM media_items WHERE id = $1", mediaID).
		Scan(&title, &filePath, &dur)
	queue = append(queue, map[string]interface{}{
		"type": "feature", "id": mediaID, "title": title, "file_path": filePath, "duration": dur,
	})

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: queue})
}

// ══════════════════════ DASH Manifest (P12-04) ══════════════════════

// GET /api/v1/stream/{mediaId}/manifest.mpd — Generate DASH manifest
func (s *Server) handleStreamDASH(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("mediaId")
	token := r.URL.Query().Get("token")

	var height, duration int
	s.db.QueryRow("SELECT COALESCE(height, 0), COALESCE(duration_seconds, 0) FROM media_items WHERE id = $1", mediaID).
		Scan(&height, &duration)

	qualities := buildDASHQualities(height)

	// Build MPD XML
	mpd := `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
	mpd += fmt.Sprintf(`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-live:2011" type="static" mediaPresentationDuration="PT%dS" minBufferTime="PT2S">`, duration) + "\n"
	mpd += `  <Period>` + "\n"
	mpd += `    <AdaptationSet mimeType="video/mp4" segmentAlignment="true">` + "\n"
	for _, q := range qualities {
		mpd += fmt.Sprintf(`      <Representation id="%s" bandwidth="%d" width="%d" height="%d">`, q.name, q.bitrate, q.width, q.height) + "\n"
		mpd += fmt.Sprintf(`        <BaseURL>/api/v1/stream/%s/%s/init.mp4?token=%s</BaseURL>`, mediaID, q.name, token) + "\n"
		mpd += `      </Representation>` + "\n"
	}
	mpd += `    </AdaptationSet>` + "\n"
	mpd += `  </Period>` + "\n"
	mpd += `</MPD>`

	w.Header().Set("Content-Type", "application/dash+xml")
	w.Write([]byte(mpd))
}

type dashQuality struct {
	name    string
	width   int
	height  int
	bitrate int
}

func buildDASHQualities(sourceHeight int) []dashQuality {
	all := []dashQuality{
		{"360p", 640, 360, 800000},
		{"480p", 854, 480, 1400000},
		{"720p", 1280, 720, 2800000},
		{"1080p", 1920, 1080, 5000000},
		{"4K", 3840, 2160, 14000000},
	}
	var result []dashQuality
	for _, q := range all {
		if q.height <= sourceHeight+100 {
			result = append(result, q)
		}
	}
	if len(result) == 0 {
		result = append(result, all[0])
	}
	return result
}

// ══════════════════════ Lyrics (P13-03) ══════════════════════

// GET /api/v1/media/{id}/lyrics
func (s *Server) handleGetLyrics(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("id")
	var source, lyricsType, content string
	err := s.db.QueryRow("SELECT source, lyrics_type, content FROM media_lyrics WHERE media_item_id = $1 ORDER BY CASE WHEN lyrics_type = 'synced' THEN 0 ELSE 1 END LIMIT 1", mediaID).
		Scan(&source, &lyricsType, &content)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: nil})
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"source": source, "type": lyricsType, "content": content,
	}})
}

