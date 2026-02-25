package auth

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/httputil"
	"github.com/JustinTDCT/CineVault/internal/version"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/login/pin", h.loginPIN)
	r.Post("/logout", h.logout)
	return r
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		PIN      string `json:"pin,omitempty"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if req.FullName == "" || req.Email == "" || req.Password == "" {
		httputil.WriteError(w, http.StatusBadRequest, "MISSING_FIELDS", "full_name, email, and password are required")
		return
	}

	req.Email = NormalizeEmail(req.Email)

	if err := ValidatePassword(req.Password, 8, false); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "WEAK_PASSWORD", err.Error())
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to hash password")
		return
	}

	var count int
	h.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	isAdmin := count == 0
	accountType := "shared"
	if isAdmin {
		accountType = "owner"
	}

	var userID string
	var pin *string
	if req.PIN != "" {
		pin = &req.PIN
	}

	defaults, _ := json.Marshal(map[string]interface{}{
		"resolution_audio": map[string]interface{}{"enabled": true, "position": "top_left"},
		"edition":          map[string]interface{}{"enabled": false, "position": "top_right"},
		"ratings":          map[string]interface{}{"enabled": true, "position": "bottom_left"},
		"content_rating":   map[string]interface{}{"enabled": false, "position": "bottom_right"},
		"source_type":      map[string]interface{}{"enabled": false, "position": "top"},
		"hide_theatrical":  false,
	})

	err = h.db.QueryRow(`
		INSERT INTO users (account_type, full_name, email, password_hash, pin, is_admin)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		accountType, req.FullName, req.Email, hash, pin, isAdmin,
	).Scan(&userID)
	if err != nil {
		httputil.WriteError(w, http.StatusConflict, "EMAIL_EXISTS", "email already registered")
		return
	}

	h.db.Exec("INSERT INTO user_profiles (user_id, overlay_settings) VALUES ($1, $2)", userID, defaults)

	token, _ := GenerateToken()
	exp := time.Now().Add(30 * 24 * time.Hour).Unix()
	h.db.Exec(
		"INSERT INTO sessions (token, user_id, is_admin, expires_at) VALUES ($1, $2, $3, $4)",
		token, userID, isAdmin, exp,
	)

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 3600,
	})

	cacheStatus := ""
	if isAdmin {
		cacheStatus = h.registerWithCacheServer(req.FullName, req.Email)
	}

	resp := map[string]interface{}{
		"user_id":  userID,
		"is_admin": isAdmin,
		"token":    token,
	}
	if cacheStatus != "" {
		resp["cache_registration"] = cacheStatus
	}

	httputil.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	req.Email = NormalizeEmail(req.Email)

	var userID, passwordHash string
	var isAdmin bool
	err := h.db.QueryRow(
		"SELECT id, password_hash, is_admin FROM users WHERE email=$1", req.Email,
	).Scan(&userID, &passwordHash, &isAdmin)
	if err != nil || !CheckPassword(passwordHash, req.Password) {
		httputil.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
		return
	}

	token, _ := GenerateToken()
	exp := time.Now().Add(30 * 24 * time.Hour).Unix()
	h.db.Exec(
		"INSERT INTO sessions (token, user_id, is_admin, expires_at) VALUES ($1, $2, $3, $4)",
		token, userID, isAdmin, exp,
	)

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 3600,
	})

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":  userID,
		"is_admin": isAdmin,
		"token":    token,
	})
}

func (h *Handler) loginPIN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		PIN    string `json:"pin"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	var storedPIN sql.NullString
	var isAdmin bool
	err := h.db.QueryRow(
		"SELECT pin, is_admin FROM users WHERE id=$1", req.UserID,
	).Scan(&storedPIN, &isAdmin)
	if err != nil || !storedPIN.Valid || storedPIN.String != req.PIN {
		httputil.WriteError(w, http.StatusUnauthorized, "INVALID_PIN", "invalid PIN")
		return
	}

	token, _ := GenerateToken()
	exp := time.Now().Add(30 * 24 * time.Hour).Unix()
	h.db.Exec(
		"INSERT INTO sessions (token, user_id, is_admin, expires_at) VALUES ($1, $2, $3, $4)",
		token, req.UserID, isAdmin, exp,
	)

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 3600,
	})

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":  req.UserID,
		"is_admin": isAdmin,
		"token":    token,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token != "" {
		h.db.Exec("DELETE FROM sessions WHERE token=$1", token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (h *Handler) registerWithCacheServer(fullName, email string) string {
	ver := version.Load()
	wanIP := getWanIP()

	payload, _ := json.Marshal(map[string]string{
		"app_name":    "CineVault",
		"app_version": ver.Version,
		"owner_name":  fullName,
		"owner_email": email,
		"wan_ip":      wanIP,
	})

	url := fmt.Sprintf("%s/api/v1/register", config.CacheServerURL)
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[cache-register] failed to reach cache server: %v", err)
		return "failed"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ClientID string `json:"client_id"`
			APIKey   string `json:"api_key"`
			Status   string `json:"status"`
		} `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[cache-register] invalid response from cache server: %v", err)
		return "failed"
	}

	if resp.StatusCode != http.StatusCreated || result.Data.APIKey == "" {
		reason := "unknown"
		if result.Error != nil {
			reason = result.Error.Message
		}
		log.Printf("[cache-register] registration rejected: %s", reason)
		return "failed"
	}

	h.db.Exec(`INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = $2`,
		"cache_server_api_key", result.Data.APIKey)
	h.db.Exec(`INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = $2`,
		"cache_server_enabled", "true")

	log.Printf("[cache-register] registered with cache server, client_id=%s", result.Data.ClientID)
	return "ok"
}

func getWanIP() string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		log.Printf("[cache-register] failed to resolve WAN IP: %v", err)
		return ""
	}
	defer resp.Body.Close()
	ip, _ := io.ReadAll(resp.Body)
	return string(ip)
}
