package api

import (
	"encoding/json"
	"net/http"

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
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"version":    "0.3.0",
			"phase":      "3",
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
