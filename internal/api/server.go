package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/JustinTDCT/CineVault/internal/auth"
	"github.com/JustinTDCT/CineVault/internal/collections"
	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/detection"
	"github.com/JustinTDCT/CineVault/internal/libraries"
	"github.com/JustinTDCT/CineVault/internal/media"
	metadataPkg "github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/player"
	"github.com/JustinTDCT/CineVault/internal/scanner"
	"github.com/JustinTDCT/CineVault/internal/search"
	"github.com/JustinTDCT/CineVault/internal/settings"
	"github.com/JustinTDCT/CineVault/internal/users"
	"github.com/JustinTDCT/CineVault/internal/watchhistory"
)

func NewServer(db *sql.DB, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	authMw := auth.NewMiddleware(db)
	userRepo := users.NewRepository(db)
	libRepo := libraries.NewRepository(db)
	mediaRepo := media.NewRepository(db)
	colRepo := collections.NewRepository(db)
	watchRepo := watchhistory.NewRepository(db)
	settingsRepo := settings.NewRepository(db)

	cacheClient := metadataPkg.NewCacheClient(cfg)
	matcher := metadataPkg.NewMatcher(db, cfg, cacheClient, mediaRepo)
	scan := scanner.New(db, cfg, mediaRepo)
	detector := detection.NewDetector(db, cfg.FFmpegPath)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Mount("/api/auth", auth.NewHandler(db).Router())

	r.Group(func(r chi.Router) {
		r.Use(authMw.RequireAuth)
		r.Mount("/api/users", users.NewHandler(userRepo).Router())
		r.Mount("/api/libraries", libraries.NewHandler(libRepo).Router())
		r.Mount("/api/media", media.NewHandler(mediaRepo).Router())
		r.Mount("/api/scanner", scanner.NewHandler(scan, libRepo).Router())
		r.Mount("/api/metadata", metadataPkg.NewHandler(matcher, mediaRepo, libRepo).Router())
		r.Mount("/api/player", player.NewHandler(mediaRepo, cfg).Router())
		r.Mount("/api/segments", detection.NewHandler(detector, mediaRepo).Router())
		r.Mount("/api/search", search.NewHandler(db, mediaRepo).Router())
		r.Mount("/api/collections", collections.NewHandler(colRepo).Router())
		r.Mount("/api/watch", watchhistory.NewHandler(watchRepo).Router())
		r.Mount("/api/settings", settings.NewHandler(settingsRepo).Router())
	})

	fileServer := http.FileServer(http.Dir("web"))
	r.Handle("/*", fileServer)

	return r
}
