package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// POST /api/v1/auth/logout — Invalidate the current JWT by adding it to session blacklist
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	tokenString := ""
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	} else if t := r.URL.Query().Get("token"); t != "" {
		tokenString = t
	}
	if tokenString == "" {
		s.respondJSON(w, http.StatusOK, Response{Success: true})
		return
	}

	// Hash the token and remove the session
	tokenHash := hashToken(tokenString)
	s.db.Exec("DELETE FROM user_sessions WHERE token_hash = $1", tokenHash)

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"status": "logged_out"}})
}

// GET /api/v1/auth/sessions — List active sessions for the current user
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)

	rows, err := s.db.Query(`SELECT id, device_info, ip_address, created_at, last_used_at
		FROM user_sessions WHERE user_id = $1 ORDER BY last_used_at DESC`, userID)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: []interface{}{}})
		return
	}
	defer rows.Close()

	var sessions []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var deviceInfo, ipAddress *string
		var createdAt, lastUsedAt time.Time
		if rows.Scan(&id, &deviceInfo, &ipAddress, &createdAt, &lastUsedAt) == nil {
			sessions = append(sessions, map[string]interface{}{
				"id":           id,
				"device_info":  deviceInfo,
				"ip_address":   ipAddress,
				"created_at":   createdAt,
				"last_used_at": lastUsedAt,
			})
		}
	}
	if sessions == nil {
		sessions = []map[string]interface{}{}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: sessions})
}

// DELETE /api/v1/auth/sessions/{id} — Revoke a specific session
func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	_, err = s.db.Exec("DELETE FROM user_sessions WHERE id = $1 AND user_id = $2", sessionID, userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to revoke session")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// recordSession creates a session record when a user logs in.
func (s *Server) recordSession(userID uuid.UUID, tokenString string, r *http.Request) {
	tokenHash := hashToken(tokenString)
	deviceInfo := r.Header.Get("User-Agent")
	ipAddress := getClientIP(r)

	s.db.Exec(`INSERT INTO user_sessions (user_id, token_hash, device_info, ip_address)
		VALUES ($1, $2, $3, $4)`, userID, tokenHash, deviceInfo, ipAddress)
}

// isSessionValid checks if a token has an active session (not revoked).
// If no sessions exist at all (pre-session-tracking), all valid JWTs are allowed through.
func (s *Server) isSessionValid(tokenString string) bool {
	// First check if any sessions exist at all — if none, we're in backwards-compatible mode
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM user_sessions").Scan(&total); err != nil {
		return true // table missing or query error — allow
	}
	if total == 0 {
		return true // no sessions tracked yet — allow all valid JWTs
	}

	tokenHash := hashToken(tokenString)
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM user_sessions WHERE token_hash = $1)", tokenHash).Scan(&exists)
	if err != nil {
		return true
	}
	return exists
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Suppress unused import warning
var _ = json.Marshal
