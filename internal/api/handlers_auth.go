package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"status": "ok"},
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	version := "unknown"
	phase := "?"
	if data, err := os.ReadFile("version.json"); err == nil {
		var v map[string]interface{}
		if json.Unmarshal(data, &v) == nil {
			if ver, ok := v["version"].(string); ok {
				version = ver
			}
			if p, ok := v["phase"].(float64); ok {
				phase = fmt.Sprintf("%d", int(p))
			}
		}
	}
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"version":    version,
			"phase":      phase,
			"ws_clients": s.wsHub.ClientCount(),
		},
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	hashedPassword, err := s.auth.HashPassword(req.Password)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &models.User{
		ID:           uuid.New(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashedPassword,
		Role:         models.RoleUser,
		IsActive:     true,
	}

	if err := s.userRepo.Create(user); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	user.PasswordHash = ""
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: user})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		s.respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := s.auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		s.respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !user.IsActive {
		s.respondError(w, http.StatusForbidden, "account is disabled")
		return
	}

	token, err := s.auth.GenerateToken(user)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	user.PasswordHash = ""
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    LoginResponse{Token: token, User: user},
	})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.userRepo.List()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	for _, user := range users {
		user.PasswordHash = ""
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: users})
}

// ──────────────────── Fast Login ────────────────────

type FastLoginUsersResponse struct {
	ID               uuid.UUID       `json:"id"`
	Username         string          `json:"username"`
	DisplayName      *string         `json:"display_name,omitempty"`
	Role             models.UserRole `json:"role"`
	HasPin           bool            `json:"has_pin"`
	IsKidsProfile    bool            `json:"is_kids_profile"`
	AvatarID         *string         `json:"avatar_id,omitempty"`
	MaxContentRating *string         `json:"max_content_rating,omitempty"`
}

type PinLoginRequest struct {
	UserID uuid.UUID `json:"user_id"`
	Pin    string    `json:"pin"`
}

type SetPinRequest struct {
	Pin string `json:"pin"`
}

// handleFastLoginUsers returns only master users for the fast login screen (public).
func (s *Server) handleFastLoginUsers(w http.ResponseWriter, r *http.Request) {
	// Check if fast login is enabled
	enabled, _ := s.settingsRepo.Get("fast_login_enabled")
	if enabled != "true" {
		s.respondError(w, http.StatusForbidden, "fast login is disabled")
		return
	}

	users, err := s.userRepo.ListMasterUsers()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	var result []FastLoginUsersResponse
	for _, u := range users {
		result = append(result, FastLoginUsersResponse{
			ID:               u.ID,
			Username:         u.Username,
			DisplayName:      u.DisplayName,
			Role:             u.Role,
			HasPin:           u.HasPin,
			IsKidsProfile:    u.IsKidsProfile,
			AvatarID:         u.AvatarID,
			MaxContentRating: u.MaxContentRating,
		})
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: result})
}

// handleFastLoginSettings returns fast login config (public).
func (s *Server) handleFastLoginSettings(w http.ResponseWriter, r *http.Request) {
	enabled, _ := s.settingsRepo.Get("fast_login_enabled")
	pinLen, _ := s.settingsRepo.Get("fast_login_pin_length")
	if pinLen == "" {
		pinLen = "4"
	}
	introEnabled, _ := s.settingsRepo.Get("login_intro_enabled")
	if introEnabled == "" {
		introEnabled = "true"
	}
	introMuted, _ := s.settingsRepo.Get("login_intro_muted")
	if introMuted == "" {
		introMuted = "false"
	}
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]string{
			"fast_login_enabled":    enabled,
			"fast_login_pin_length": pinLen,
			"login_intro_enabled":   introEnabled,
			"login_intro_muted":     introMuted,
		},
	})
}

// handlePinLogin authenticates a user by PIN (public).
func (s *Server) handlePinLogin(w http.ResponseWriter, r *http.Request) {
	enabled, _ := s.settingsRepo.Get("fast_login_enabled")
	if enabled != "true" {
		s.respondError(w, http.StatusForbidden, "fast login is disabled")
		return
	}

	var req PinLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.userRepo.GetByID(req.UserID)
	if err != nil {
		s.respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !user.IsActive {
		s.respondError(w, http.StatusForbidden, "account is disabled")
		return
	}

	if user.PinHash == nil || *user.PinHash == "" {
		s.respondError(w, http.StatusUnauthorized, "no PIN set for this user")
		return
	}

	if err := s.auth.VerifyPassword(*user.PinHash, req.Pin); err != nil {
		s.respondError(w, http.StatusUnauthorized, "invalid PIN")
		return
	}

	token, err := s.auth.GenerateToken(user)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	user.PasswordHash = ""
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    LoginResponse{Token: token, User: user},
	})
}

// handleSetPin sets or updates the current user's PIN.
func (s *Server) handleSetPin(w http.ResponseWriter, r *http.Request) {
	var req SetPinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate PIN is all digits
	for _, c := range req.Pin {
		if !unicode.IsDigit(c) {
			s.respondError(w, http.StatusBadRequest, "PIN must contain only digits")
			return
		}
	}

	// Validate PIN length against setting
	pinLenStr, _ := s.settingsRepo.Get("fast_login_pin_length")
	if pinLenStr == "" {
		pinLenStr = "4"
	}
	requiredLen, _ := strconv.Atoi(pinLenStr)
	if len(req.Pin) != requiredLen {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("PIN must be exactly %d digits", requiredLen))
		return
	}

	userID := s.getUserID(r)
	hashedPin, err := s.auth.HashPassword(req.Pin)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to hash PIN")
		return
	}

	if err := s.userRepo.UpdatePinHash(userID, &hashedPin); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to set PIN")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// handleAdminSetPin allows an admin to set a PIN for any user.
func (s *Server) handleAdminSetPin(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req SetPinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Allow empty pin to clear it
	if req.Pin == "" {
		if err := s.userRepo.UpdatePinHash(userID, nil); err != nil {
			s.respondError(w, http.StatusInternalServerError, "failed to clear PIN")
			return
		}
		s.respondJSON(w, http.StatusOK, Response{Success: true})
		return
	}

	// Validate PIN is all digits
	for _, c := range req.Pin {
		if !unicode.IsDigit(c) {
			s.respondError(w, http.StatusBadRequest, "PIN must contain only digits")
			return
		}
	}

	// Validate PIN length against setting
	pinLenStr, _ := s.settingsRepo.Get("fast_login_pin_length")
	if pinLenStr == "" {
		pinLenStr = "4"
	}
	requiredLen, _ := strconv.Atoi(pinLenStr)
	if len(req.Pin) != requiredLen {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("PIN must be exactly %d digits", requiredLen))
		return
	}

	hashedPin, err := s.auth.HashPassword(req.Pin)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to hash PIN")
		return
	}

	if err := s.userRepo.UpdatePinHash(userID, &hashedPin); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to set PIN")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ──────────────────── Profile ────────────────────

type UpdateProfileRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Email     *string `json:"email"`
	Password  *string `json:"password"`
}

// handleGetProfile returns the current user's profile.
func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "user not found")
		return
	}
	user.PasswordHash = ""
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: user})
}

// handleUpdateProfile updates the current user's profile fields.
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := s.getUserID(r)

	// Update profile fields (first_name, last_name, email)
	if req.FirstName != nil || req.LastName != nil || req.Email != nil {
		if err := s.userRepo.UpdateProfile(userID, req.FirstName, req.LastName, req.Email); err != nil {
			s.respondError(w, http.StatusInternalServerError, "failed to update profile")
			return
		}
	}

	// Update password if provided
	if req.Password != nil && *req.Password != "" {
		hashedPassword, err := s.auth.HashPassword(*req.Password)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		if err := s.userRepo.UpdatePassword(userID, hashedPassword); err != nil {
			s.respondError(w, http.StatusInternalServerError, "failed to update password")
			return
		}
	}

	// Return updated user
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to fetch updated profile")
		return
	}
	user.PasswordHash = ""
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: user})
}

// ──────────────────── First-Run Setup ────────────────────

type SetupRequest struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Pin       string `json:"pin"`
}

// handleSetupCheck returns whether initial setup is needed (no users exist).
func (s *Server) handleSetupCheck(w http.ResponseWriter, r *http.Request) {
	count, err := s.userRepo.Count()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to check setup status")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]bool{"setup_required": count == 0},
	})
}

// handleSetup creates the initial admin user on first run.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	// Only allow setup if no users exist
	count, err := s.userRepo.Count()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to check setup status")
		return
	}
	if count > 0 {
		s.respondError(w, http.StatusForbidden, "setup already completed")
		return
	}

	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	req.Username = strings.TrimSpace(req.Username)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Email = strings.TrimSpace(req.Email)
	req.Pin = strings.TrimSpace(req.Pin)

	if req.Username == "" || req.Email == "" || req.Password == "" {
		s.respondError(w, http.StatusBadRequest, "username, email, and password are required")
		return
	}
	if req.FirstName == "" || req.LastName == "" {
		s.respondError(w, http.StatusBadRequest, "first name and last name are required")
		return
	}

	// Hash password
	hashedPassword, err := s.auth.HashPassword(req.Password)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	// Hash PIN if provided
	var pinHash *string
	if req.Pin != "" {
		for _, c := range req.Pin {
			if !unicode.IsDigit(c) {
				s.respondError(w, http.StatusBadRequest, "PIN must contain only digits")
				return
			}
		}
		pinLenStr, _ := s.settingsRepo.Get("fast_login_pin_length")
		if pinLenStr == "" {
			pinLenStr = "4"
		}
		requiredLen, _ := strconv.Atoi(pinLenStr)
		if len(req.Pin) != requiredLen {
			s.respondError(w, http.StatusBadRequest, fmt.Sprintf("PIN must be exactly %d digits", requiredLen))
			return
		}
		h, err := s.auth.HashPassword(req.Pin)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, "failed to hash PIN")
			return
		}
		pinHash = &h
	}

	displayName := req.FirstName
	user := &models.User{
		ID:           uuid.New(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashedPassword,
		PinHash:      pinHash,
		DisplayName:  &displayName,
		FirstName:    &req.FirstName,
		LastName:     &req.LastName,
		Role:         models.RoleAdmin,
		IsActive:     true,
	}

	if err := s.userRepo.Create(user); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create admin user")
		return
	}

	// Generate token so the user is logged in immediately
	token, err := s.auth.GenerateToken(user)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	user.PasswordHash = ""
	s.respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Data:    LoginResponse{Token: token, User: user},
	})
}

// Ensure imports are used
var _ = strings.TrimSpace
