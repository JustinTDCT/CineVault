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
	"github.com/JustinTDCT/CineVault/internal/scanner"
	"github.com/google/uuid"
)

type Server struct {
	config         *config.Config
	db             *db.DB
	auth           *auth.Auth
	userRepo       *repository.UserRepository
	libRepo        *repository.LibraryRepository
	mediaRepo      *repository.MediaRepository
	tvRepo         *repository.TVRepository
	musicRepo      *repository.MusicRepository
	audiobookRepo  *repository.AudiobookRepository
	galleryRepo    *repository.GalleryRepository
	editionRepo    *repository.EditionRepository
	sisterRepo     *repository.SisterRepository
	collectionRepo *repository.CollectionRepository
	watchRepo      *repository.WatchHistoryRepository
	scanner        *scanner.Scanner
	router         *http.ServeMux
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func NewServer(cfg *config.Config, database *db.DB) (*Server, error) {
	authService, err := auth.NewAuth(cfg.JWT.Secret, cfg.JWT.ExpiresIn)
	if err != nil {
		return nil, err
	}

	mediaRepo := repository.NewMediaRepository(database.DB)
	tvRepo := repository.NewTVRepository(database.DB)
	musicRepo := repository.NewMusicRepository(database.DB)
	audiobookRepo := repository.NewAudiobookRepository(database.DB)
	galleryRepo := repository.NewGalleryRepository(database.DB)

	sc := scanner.NewScanner(cfg.FFmpeg.FFprobePath, mediaRepo, tvRepo, musicRepo, audiobookRepo, galleryRepo)

	s := &Server{
		config:         cfg,
		db:             database,
		auth:           authService,
		userRepo:       repository.NewUserRepository(database.DB),
		libRepo:        repository.NewLibraryRepository(database.DB),
		mediaRepo:      mediaRepo,
		tvRepo:         tvRepo,
		musicRepo:      musicRepo,
		audiobookRepo:  audiobookRepo,
		galleryRepo:    galleryRepo,
		editionRepo:    repository.NewEditionRepository(database.DB),
		sisterRepo:     repository.NewSisterRepository(database.DB),
		collectionRepo: repository.NewCollectionRepository(database.DB),
		watchRepo:      repository.NewWatchHistoryRepository(database.DB),
		scanner:        sc,
		router:         http.NewServeMux(),
	}

	s.setupRoutes()
	return s, nil
}

func (s *Server) setupRoutes() {
	// Static files
	fs := http.FileServer(http.Dir("web"))
	s.router.Handle("/", fs)

	// Public
	s.router.HandleFunc("GET /health", s.handleHealth)
	s.router.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.router.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.router.HandleFunc("POST /api/v1/auth/login", s.handleLogin)

	// Users (admin)
	s.router.HandleFunc("GET /api/v1/users", s.authMiddleware(s.handleListUsers, models.RoleAdmin))

	// Libraries
	s.router.HandleFunc("GET /api/v1/libraries", s.authMiddleware(s.handleListLibraries, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/libraries", s.authMiddleware(s.handleCreateLibrary, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/libraries/{id}", s.authMiddleware(s.handleGetLibrary, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/libraries/{id}", s.authMiddleware(s.handleUpdateLibrary, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/libraries/{id}", s.authMiddleware(s.handleDeleteLibrary, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/libraries/{id}/scan", s.authMiddleware(s.handleScanLibrary, models.RoleAdmin))

	// Media
	s.router.HandleFunc("GET /api/v1/libraries/{id}/media", s.authMiddleware(s.handleListMedia, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/media/{id}", s.authMiddleware(s.handleGetMedia, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/media/search", s.authMiddleware(s.handleSearchMedia, models.RoleUser))

	// Edition groups
	s.router.HandleFunc("GET /api/v1/editions", s.authMiddleware(s.handleListEditions, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/editions", s.authMiddleware(s.handleCreateEdition, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/editions/{id}", s.authMiddleware(s.handleGetEdition, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/editions/{id}", s.authMiddleware(s.handleUpdateEdition, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/editions/{id}", s.authMiddleware(s.handleDeleteEdition, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/editions/{id}/items", s.authMiddleware(s.handleAddEditionItem, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/editions/{id}/items/{itemId}", s.authMiddleware(s.handleRemoveEditionItem, models.RoleAdmin))

	// Sister groups
	s.router.HandleFunc("GET /api/v1/sisters", s.authMiddleware(s.handleListSisters, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/sisters", s.authMiddleware(s.handleCreateSister, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/sisters/{id}", s.authMiddleware(s.handleGetSister, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/sisters/{id}/items", s.authMiddleware(s.handleAddSisterItem, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/sisters/{id}/items/{itemId}", s.authMiddleware(s.handleRemoveSisterItem, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/sisters/{id}", s.authMiddleware(s.handleDeleteSister, models.RoleAdmin))

	// Collections
	s.router.HandleFunc("GET /api/v1/collections", s.authMiddleware(s.handleListCollections, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/collections", s.authMiddleware(s.handleCreateCollection, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/collections/{id}", s.authMiddleware(s.handleGetCollection, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/collections/{id}/items", s.authMiddleware(s.handleAddCollectionItem, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/collections/{id}/items/{itemId}", s.authMiddleware(s.handleRemoveCollectionItem, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/collections/{id}", s.authMiddleware(s.handleDeleteCollection, models.RoleUser))

	// Watch history
	s.router.HandleFunc("POST /api/v1/watch/{mediaId}/progress", s.authMiddleware(s.handleUpdateProgress, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/watch/continue", s.authMiddleware(s.handleContinueWatching, models.RoleUser))
}

// ──────────────────── Middleware ────────────────────

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

		r.Header.Set("X-User-ID", claims.UserID.String())
		r.Header.Set("X-User-Role", string(claims.Role))

		next(w, r)
	}
}

// ──────────────────── Helpers ────────────────────

func (s *Server) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) respondError(w http.ResponseWriter, statusCode int, message string) {
	s.respondJSON(w, statusCode, Response{Success: false, Error: message})
}

func (s *Server) getUserID(r *http.Request) uuid.UUID {
	id, _ := uuid.Parse(r.Header.Get("X-User-ID"))
	return id
}

func (s *Server) Start() error {
	return http.ListenAndServe(s.config.Server.Address(), s.router)
}
