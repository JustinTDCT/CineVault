package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/JustinTDCT/CineVault/internal/auth"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// POST /api/v1/auth/reset-token — Admin generates a reset token for a user
func (s *Server) handleCreateResetToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.UserID == "" {
		s.respondError(w, http.StatusBadRequest, "user_id required")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	// Verify user exists
	_, err = s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "user not found")
		return
	}

	// Generate token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	rawToken := hex.EncodeToString(tokenBytes)
	tokenHash := sha256Sum(rawToken)

	// Store hashed token with 24h expiry
	_, err = s.db.Exec(`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, userID, tokenHash, time.Now().Add(24*time.Hour))
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create reset token")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"reset_token": rawToken,
		"expires_in":  "24h",
	}})
}

// POST /api/v1/auth/reset-password — User submits token + new password
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Token == "" || req.Password == "" {
		s.respondError(w, http.StatusBadRequest, "token and password required")
		return
	}

	if err := auth.ValidatePassword(req.Password); err != nil {
		s.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	tokenHash := sha256Sum(req.Token)

	// Look up valid, unused token
	var tokenID uuid.UUID
	var userID uuid.UUID
	err := s.db.QueryRow(`SELECT id, user_id FROM password_reset_tokens
		WHERE token_hash = $1 AND expires_at > NOW() AND used = FALSE`, tokenHash).
		Scan(&tokenID, &userID)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid or expired reset token")
		return
	}

	// Hash new password
	hashedPassword, err := s.auth.HashPassword(req.Password)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	// Update password
	_, err = s.db.Exec("UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2", hashedPassword, userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	// Mark token as used
	s.db.Exec("UPDATE password_reset_tokens SET used = TRUE WHERE id = $1", tokenID)

	// Invalidate all existing sessions for this user
	s.db.Exec("DELETE FROM user_sessions WHERE user_id = $1", userID)

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"status": "password_reset"}})
}

// Ensure the userRepo has GetByID — it should already exist.
// If not, the compiler will tell us.
var _ = models.User{}
