package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ══════════════════════ TOTP 2FA (P11-01) ══════════════════════

// POST /api/v1/auth/2fa/setup — Generate TOTP secret and QR URL
func (s *Server) handle2FASetup(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)

	// Generate random secret
	secretBytes := make([]byte, 20)
	rand.Read(secretBytes)
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)

	// Get username
	var username string
	s.db.QueryRow("SELECT username FROM users WHERE id = $1", userID).Scan(&username)

	// Store secret (not yet enabled)
	s.db.Exec("UPDATE users SET totp_secret = $1 WHERE id = $2", secret, userID)

	// Build otpauth URL for QR code
	otpURL := fmt.Sprintf("otpauth://totp/CineVault:%s?secret=%s&issuer=CineVault&algorithm=SHA1&digits=6&period=30",
		username, secret)

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"secret":  secret,
		"otp_url": otpURL,
	}})
}

// POST /api/v1/auth/2fa/verify — Verify TOTP code and enable 2FA
func (s *Server) handle2FAVerify(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		Code string `json:"code"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || len(req.Code) != 6 {
		s.respondError(w, http.StatusBadRequest, "6-digit code required")
		return
	}

	var secret string
	err := s.db.QueryRow("SELECT COALESCE(totp_secret, '') FROM users WHERE id = $1", userID).Scan(&secret)
	if err != nil || secret == "" {
		s.respondError(w, http.StatusBadRequest, "2FA setup not started")
		return
	}

	if !validateTOTP(secret, req.Code) {
		s.respondError(w, http.StatusBadRequest, "invalid code")
		return
	}

	// Generate recovery codes
	recoveryCodes := make([]string, 8)
	for i := range recoveryCodes {
		b := make([]byte, 4)
		rand.Read(b)
		recoveryCodes[i] = hex.EncodeToString(b)
	}
	codesJSON, _ := json.Marshal(recoveryCodes)

	// Enable 2FA
	s.db.Exec("UPDATE users SET totp_enabled = TRUE, recovery_codes = $2 WHERE id = $1", userID, string(codesJSON))

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"enabled":        true,
		"recovery_codes": recoveryCodes,
	}})
}

// DELETE /api/v1/auth/2fa — Disable 2FA
func (s *Server) handle2FADisable(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	s.db.Exec("UPDATE users SET totp_enabled = FALSE, totp_secret = NULL, recovery_codes = NULL WHERE id = $1", userID)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// GET /api/v1/auth/2fa/status
func (s *Server) handle2FAStatus(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var enabled bool
	s.db.QueryRow("SELECT totp_enabled FROM users WHERE id = $1", userID).Scan(&enabled)
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"enabled": enabled}})
}

// POST /api/v1/auth/2fa/validate — Validate TOTP during login
func (s *Server) handle2FAValidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Code   string `json:"code"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Code == "" {
		s.respondError(w, http.StatusBadRequest, "code required")
		return
	}

	var secret string
	var enabled bool
	err := s.db.QueryRow("SELECT COALESCE(totp_secret, ''), totp_enabled FROM users WHERE id = $1", req.UserID).
		Scan(&secret, &enabled)
	if err != nil || !enabled {
		s.respondError(w, http.StatusBadRequest, "2FA not enabled")
		return
	}

	// Check TOTP code
	if validateTOTP(secret, req.Code) {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"valid": true}})
		return
	}

	// Check recovery codes
	var codesJSON string
	s.db.QueryRow("SELECT COALESCE(recovery_codes::text, '[]') FROM users WHERE id = $1", req.UserID).Scan(&codesJSON)
	var codes []string
	json.Unmarshal([]byte(codesJSON), &codes)
	for i, code := range codes {
		if code == req.Code {
			// Remove used recovery code
			codes = append(codes[:i], codes[i+1:]...)
			newJSON, _ := json.Marshal(codes)
			s.db.Exec("UPDATE users SET recovery_codes = $2 WHERE id = $1", req.UserID, string(newJSON))
			s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"valid": true, "recovery_code_used": true}})
			return
		}
	}

	s.respondError(w, http.StatusUnauthorized, "invalid code")
}

// ── TOTP Implementation ──

func validateTOTP(secret, code string) bool {
	// Check current, previous, and next time steps (30-second window)
	now := time.Now().Unix()
	for _, offset := range []int64{-30, 0, 30} {
		if generateTOTP(secret, (now+offset)/30) == code {
			return true
		}
	}
	return false
}

func generateTOTP(secret string, counter int64) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return ""
	}

	// Counter to bytes (big-endian)
	msg := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		msg[i] = byte(counter & 0xff)
		counter >>= 8
	}

	// HMAC-SHA1
	mac := hmac.New(sha256.New, key)
	mac.Write(msg)
	h := mac.Sum(nil)

	// Dynamic truncation
	offset := h[len(h)-1] & 0x0f
	code := int64(h[offset]&0x7f)<<24 |
		int64(h[offset+1])<<16 |
		int64(h[offset+2])<<8 |
		int64(h[offset+3])

	// 6 digits
	otp := code % 1000000
	return fmt.Sprintf("%06d", otp)
}

// ══════════════════════ Rate Limiting (P11-02) ══════════════════════

// In-memory rate limiter with DB-backed IP blocking
var rateLimitCounters = struct {
	mu       sync.Mutex
	counters map[string]*rateBucket
}{counters: make(map[string]*rateBucket)}

type rateBucket struct {
	count   int
	resetAt time.Time
}

// checkRateLimit returns true if the request should be allowed
func (s *Server) checkRateLimit(ip string, bucket string, maxRequests int, window time.Duration) bool {
	key := bucket + ":" + ip
	rateLimitCounters.mu.Lock()
	defer rateLimitCounters.mu.Unlock()

	b, ok := rateLimitCounters.counters[key]
	if !ok || time.Now().After(b.resetAt) {
		rateLimitCounters.counters[key] = &rateBucket{count: 1, resetAt: time.Now().Add(window)}
		return true
	}
	b.count++
	return b.count <= maxRequests
}

// CheckAuthRateLimit for login attempts (5/min per IP)
func (s *Server) checkAuthRateLimit(r *http.Request) bool {
	ip := getClientIP(r)
	// Check if IP is blocked in DB
	var blockedUntil time.Time
	err := s.db.QueryRow("SELECT blocked_until FROM rate_limit_blocks WHERE ip_address = $1 AND blocked_until > NOW()", ip).
		Scan(&blockedUntil)
	if err == nil {
		return false // still blocked
	}
	return s.checkRateLimit(ip, "auth", 5, time.Minute)
}

// RecordAuthFailure tracks failed login attempts
func (s *Server) recordAuthFailure(r *http.Request) {
	ip := getClientIP(r)
	key := "auth_failures:" + ip
	rateLimitCounters.mu.Lock()
	b, ok := rateLimitCounters.counters[key]
	if !ok || time.Now().After(b.resetAt) {
		rateLimitCounters.counters[key] = &rateBucket{count: 1, resetAt: time.Now().Add(15 * time.Minute)}
		rateLimitCounters.mu.Unlock()
		return
	}
	b.count++
	count := b.count
	rateLimitCounters.mu.Unlock()

	if count >= 5 {
		// Block IP for 15 minutes
		s.db.Exec("INSERT INTO rate_limit_blocks (ip_address, blocked_until, failure_count) VALUES ($1, NOW() + INTERVAL '15 minutes', $2) ON CONFLICT DO NOTHING",
			ip, count)
	}
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := make([]string, 0)
		for _, p := range splitComma(xff) {
			parts = append(parts, p)
		}
		if len(parts) > 0 { return parts[0] }
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}
	return addr
}

// splitComma already defined in handlers_media.go

// rateLimitMiddleware wraps a handler with rate limiting (P11-02)
func (s *Server) rateLimitMiddleware(next http.HandlerFunc, maxReqs int, window time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		if !s.checkRateLimit(ip, "api", maxReqs, window) {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
			s.respondError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next(w, r)
	}
}
