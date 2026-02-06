package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"github.com/JustinTDCT/CineVault/internal/auth"
	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/db"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/google/uuid"
)

type Server struct {
	config    *config.Config
	db        *db.DB
	auth      *auth.Auth
	userRepo  *repository.UserRepository
	libRepo   *repository.LibraryRepository
	mediaRepo *repository.MediaRepository
	router    *http.ServeMux
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

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

func NewServer(cfg *config.Config, database *db.DB) (*Server, error) {
	authService, err := auth.NewAuth(cfg.JWT.Secret, cfg.JWT.ExpiresIn)
	if err != nil {
		return nil, err
	}

	s := &Server{
		config:    cfg,
		db:        database,
		auth:      authService,
		userRepo:  repository.NewUserRepository(database.DB),
		libRepo:   repository.NewLibraryRepository(database.DB),
		mediaRepo: repository.NewMediaRepository(database.DB),
		router:    http.NewServeMux(),
	}
	
	s.setupRoutes()
	return s, nil
}

func (s *Server) setupRoutes() {
	// Serve static files from web directory
	fs := http.FileServer(http.Dir("web"))
	s.router.Handle("/", fs)
	
	// Public routes
	s.router.HandleFunc("GET /health", s.handleHealth)
	s.router.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.router.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.router.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	
	// Protected routes (require authentication)
	s.router.HandleFunc("GET /api/v1/users", s.authMiddleware(s.handleListUsers, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/libraries", s.authMiddleware(s.handleListLibraries, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/libraries", s.authMiddleware(s.handleCreateLibrary, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/libraries/{id}", s.authMiddleware(s.handleGetLibrary, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/libraries/{id}", s.authMiddleware(s.handleUpdateLibrary, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/libraries/{id}", s.authMiddleware(s.handleDeleteLibrary, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/libraries/{id}/media", s.authMiddleware(s.handleListMedia, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/media/{id}", s.authMiddleware(s.handleGetMedia, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/media/search", s.authMiddleware(s.handleSearchMedia, models.RoleUser))
}

// Middleware
func (s *Server) authMiddleware(next http.HandlerFunc, requiredRole models.UserRole) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.respondError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := s.auth.ValidateToken(tokenString)
		if err != nil {
			s.respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		if !s.auth.CheckPermission(claims.Role, requiredRole) {
			s.respondError(w, http.StatusForbidden, "insufficient permissions")
			return
		}

		// Store claims in context (simplified - in production use context.Context)
		r.Header.Set("X-User-ID", claims.UserID.String())
		r.Header.Set("X-User-Role", string(claims.Role))
		
		next(w, r)
	}
}

// Health and Status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]string{"status": "ok"},
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]string{"version": "0.1.0", "phase": "1"},
	})
}

// Auth handlers
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

	user.PasswordHash = "" // Don't send password hash in response
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
		Data: LoginResponse{Token: token, User: user},
	})
}

// User handlers
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

// Library handlers
func (s *Server) handleListLibraries(w http.ResponseWriter, r *http.Request) {
	libraries, err := s.libRepo.List()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list libraries")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: libraries})
}

func (s *Server) handleCreateLibrary(w http.ResponseWriter, r *http.Request) {
	var library models.Library
	if err := json.NewDecoder(r.Body).Decode(&library); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	library.ID = uuid.New()
	if err := s.libRepo.Create(&library); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create library")
		return
	}

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: library})
}

func (s *Server) handleGetLibrary(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	library, err := s.libRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "library not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: library})
}

func (s *Server) handleUpdateLibrary(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	var library models.Library
	if err := json.NewDecoder(r.Body).Decode(&library); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	library.ID = id
	if err := s.libRepo.Update(&library); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update library")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: library})
}

func (s *Server) handleDeleteLibrary(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	if err := s.libRepo.Delete(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to delete library")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "library deleted"}})
}

// Media handlers
func (s *Server) handleListMedia(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	libraryID, err := uuid.Parse(idStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	media, err := s.mediaRepo.ListByLibrary(libraryID, 50, 0)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list media")
		return
	}

	count, _ := s.mediaRepo.CountByLibrary(libraryID)
	
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"items": media,
			"total": count,
		},
	})
}

func (s *Server) handleGetMedia(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	media, err := s.mediaRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: media})
}

func (s *Server) handleSearchMedia(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		s.respondError(w, http.StatusBadRequest, "missing search query")
		return
	}

	media, err := s.mediaRepo.Search(query, 50)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "search failed")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: media})
}

// Helper methods
func (s *Server) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) respondError(w http.ResponseWriter, statusCode int, message string) {
	s.respondJSON(w, statusCode, Response{Success: false, Error: message})
}

func (s *Server) Start() error {
	return http.ListenAndServe(s.config.Server.Address(), s.router)
}
