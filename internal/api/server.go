package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/auth"
	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/db"
	"github.com/JustinTDCT/CineVault/internal/detection"
	"github.com/JustinTDCT/CineVault/internal/jobs"
	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/notifications"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/JustinTDCT/CineVault/internal/scanner"
	"github.com/JustinTDCT/CineVault/internal/stream"
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
	seriesRepo     *repository.SeriesRepository
	watchRepo      *repository.WatchHistoryRepository
	performerRepo  *repository.PerformerRepository
	tagRepo        *repository.TagRepository
	studioRepo     *repository.StudioRepository
	settingsRepo   *repository.SettingsRepository
	jobRepo        *repository.JobRepository
	segmentRepo       *repository.SegmentRepository
	analyticsRepo     *repository.AnalyticsRepository
	notificationRepo  *repository.NotificationRepository
	displayPrefsRepo  *repository.DisplayPreferencesRepository
	tracksRepo        *repository.TracksRepository
	detector         *detection.Detector
	scanner          *scanner.Scanner
	transcoder       *stream.Transcoder
	jobQueue         *jobs.Queue
	wsHub            *WSHub
	webhookSender    *notifications.WebhookSender
	scrapers         []metadata.Scraper
	router           *http.ServeMux
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func NewServer(cfg *config.Config, database *db.DB, jobQueue *jobs.Queue) (*Server, error) {
	authService, err := auth.NewAuth(cfg.JWT.Secret, cfg.JWT.ExpiresIn)
	if err != nil {
		return nil, err
	}

	mediaRepo := repository.NewMediaRepository(database.DB)
	tvRepo := repository.NewTVRepository(database.DB)
	musicRepo := repository.NewMusicRepository(database.DB)
	audiobookRepo := repository.NewAudiobookRepository(database.DB)
	galleryRepo := repository.NewGalleryRepository(database.DB)

	// Initialize metadata scrapers
	var scrapers []metadata.Scraper
	tmdbKey := cfg.TMDBAPIKey
	if tmdbKey != "" {
		scrapers = append(scrapers, metadata.NewTMDBScraper(tmdbKey))
	}
	scrapers = append(scrapers, metadata.NewMusicBrainzScraper())
	scrapers = append(scrapers, metadata.NewOpenLibraryScraper())

	tagRepo := repository.NewTagRepository(database.DB)
	settingsRepo := repository.NewSettingsRepository(database.DB)

	// Initialize TVDB scraper if API key is configured
	tvdbKey, _ := settingsRepo.Get("tvdb_api_key")
	if tvdbKey != "" {
		scrapers = append(scrapers, metadata.NewTVDBScraper(tvdbKey))
	}

	performerRepo := repository.NewPerformerRepository(database.DB)
	sisterRepo := repository.NewSisterRepository(database.DB)
	seriesRepo := repository.NewSeriesRepository(database.DB)
	tracksRepo := repository.NewTracksRepository(database.DB)
	posterDir := cfg.Paths.Preview
	sc := scanner.NewScanner(cfg.FFmpeg.FFprobePath, cfg.FFmpeg.FFmpegPath, mediaRepo, tvRepo, musicRepo, audiobookRepo, galleryRepo, tagRepo, performerRepo, settingsRepo, sisterRepo, seriesRepo, tracksRepo, scrapers, posterDir)
	transcoder := stream.NewTranscoder(cfg.FFmpeg.FFmpegPath, cfg.Paths.Preview)

	wsHub := NewWSHub()

	segmentRepo := repository.NewSegmentRepository(database.DB)
	analyticsRepo := repository.NewAnalyticsRepository(database.DB)
	notificationRepo := repository.NewNotificationRepository(database.DB)
	det := detection.NewDetector(cfg.FFmpeg.FFmpegPath)
	webhookSender := notifications.NewWebhookSender()

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
		seriesRepo:     repository.NewSeriesRepository(database.DB),
		watchRepo:      repository.NewWatchHistoryRepository(database.DB),
		performerRepo:  performerRepo,
		tagRepo:        tagRepo,
		studioRepo:     repository.NewStudioRepository(database.DB),
		settingsRepo:   settingsRepo,
		jobRepo:        repository.NewJobRepository(database.DB),
		segmentRepo:      segmentRepo,
		analyticsRepo:    analyticsRepo,
		notificationRepo: notificationRepo,
		displayPrefsRepo: repository.NewDisplayPreferencesRepository(database.DB),
		tracksRepo:       tracksRepo,
		detector:         det,
		scanner:          sc,
		transcoder:       transcoder,
		jobQueue:         jobQueue,
		wsHub:            wsHub,
		webhookSender:    webhookSender,
		scrapers:         scrapers,
		router:           http.NewServeMux(),
	}

	s.setupRoutes()
	return s, nil
}

func (s *Server) WSHub() *WSHub {
	return s.wsHub
}

func (s *Server) Scanner() *scanner.Scanner {
	return s.scanner
}

func (s *Server) LibRepo() *repository.LibraryRepository {
	return s.libRepo
}

func (s *Server) MediaRepo() *repository.MediaRepository {
	return s.mediaRepo
}

func (s *Server) JobRepo() *repository.JobRepository {
	return s.jobRepo
}

func (s *Server) Scrapers() []metadata.Scraper {
	return s.scrapers
}

func (s *Server) SettingsRepo() *repository.SettingsRepository {
	return s.settingsRepo
}

func (s *Server) Config() *config.Config {
	return s.config
}

func (s *Server) SegmentRepo() *repository.SegmentRepository {
	return s.segmentRepo
}

func (s *Server) Detector() *detection.Detector {
	return s.detector
}

func (s *Server) AnalyticsRepo() *repository.AnalyticsRepository {
	return s.analyticsRepo
}

func (s *Server) NotificationRepo() *repository.NotificationRepository {
	return s.notificationRepo
}

func (s *Server) Transcoder() *stream.Transcoder {
	return s.transcoder
}

func (s *Server) WebhookSender() *notifications.WebhookSender {
	return s.webhookSender
}

func (s *Server) setupRoutes() {
	// Static files
	fs := http.FileServer(http.Dir("web"))
	s.router.Handle("/", fs)

	// Preview files (no-cache so updated posters are always revalidated)
	// Supports ?w= width param for responsive images (P11-03)
	previewFS := http.StripPrefix("/previews/", http.FileServer(http.Dir(s.config.Paths.Preview)))
	s.router.Handle("/previews/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		// WebP negotiation
		if strings.Contains(r.Header.Get("Accept"), "image/webp") {
			w.Header().Set("Vary", "Accept")
		}
		previewFS.ServeHTTP(w, r)
	}))

	// Public
	s.router.HandleFunc("GET /health", s.handleHealth)
	s.router.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.router.HandleFunc("GET /api/v1/setup/check", s.handleSetupCheck)
	s.router.HandleFunc("POST /api/v1/setup", s.handleSetup)
	s.router.HandleFunc("POST /api/v1/auth/register", s.rlAuth(s.handleRegister))
	s.router.HandleFunc("POST /api/v1/auth/login", s.rlAuth(s.handleLogin))

	// Fast Login (public — no auth required)
	s.router.HandleFunc("GET /api/v1/auth/fast-login/settings", s.rlRead(s.handleFastLoginSettings))
	s.router.HandleFunc("GET /api/v1/auth/fast-login/users", s.rlRead(s.handleFastLoginUsers))
	s.router.HandleFunc("POST /api/v1/auth/fast-login", s.rlAuth(s.handlePinLogin))

	// Password reset (admin creates token, user resets with token)
	s.router.HandleFunc("POST /api/v1/auth/reset-token", s.authMiddleware(s.handleCreateResetToken, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/auth/reset-password", s.rlAuth(s.handleResetPassword))

	// Session management
	s.router.HandleFunc("POST /api/v1/auth/logout", s.authMiddleware(s.handleLogout, models.RoleGuest))
	s.router.HandleFunc("GET /api/v1/auth/sessions", s.authMiddleware(s.handleListSessions, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/auth/sessions/{id}", s.authMiddleware(s.handleRevokeSession, models.RoleUser))

	// WebSocket
	s.router.HandleFunc("GET /api/v1/ws", s.handleWebSocket)

	// Users (admin)
	s.router.HandleFunc("GET /api/v1/users", s.authMiddleware(s.handleListUsers, models.RoleAdmin))

	// Profile (authenticated user)
	s.router.HandleFunc("GET /api/v1/profile", s.authMiddleware(s.handleGetProfile, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/profile", s.authMiddleware(s.handleUpdateProfile, models.RoleUser))

	// PIN management
	s.router.HandleFunc("PUT /api/v1/auth/pin", s.authMiddleware(s.handleSetPin, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/users/{id}/pin", s.authMiddleware(s.handleAdminSetPin, models.RoleAdmin))

	// Filesystem browse (admin only)
	s.router.HandleFunc("GET /api/v1/browse", s.authMiddleware(s.handleBrowse, models.RoleAdmin))

	// Libraries
	s.router.HandleFunc("GET /api/v1/libraries", s.authMiddleware(s.handleListLibraries, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/libraries", s.authMiddleware(s.handleCreateLibrary, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/libraries/{id}", s.authMiddleware(s.handleGetLibrary, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/libraries/{id}", s.authMiddleware(s.handleUpdateLibrary, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/libraries/{id}", s.authMiddleware(s.handleDeleteLibrary, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/libraries/{id}/scan", s.authMiddleware(s.handleScanLibrary, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/libraries/{id}/auto-match", s.authMiddleware(s.handleAutoMatch, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/libraries/{id}/refresh-metadata", s.authMiddleware(s.handleRefreshMetadata, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/libraries/{id}/phash", s.authMiddleware(s.handlePhashLibrary, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/libraries/{id}/filters", s.authMiddleware(s.handleLibraryFilters, models.RoleUser))

	// TV Shows
	s.router.HandleFunc("GET /api/v1/libraries/{id}/shows", s.authMiddleware(s.handleListLibraryShows, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/libraries/{id}/missing-episodes", s.authMiddleware(s.handleMissingEpisodes, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/tv/shows/{id}", s.authMiddleware(s.handleGetShow, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/tv/shows/{id}/missing-episodes", s.authMiddleware(s.handleShowMissingEpisodes, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/tv/shows/{id}/seasons", s.authMiddleware(s.handleListShowSeasons, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/tv/seasons/{id}/episodes", s.authMiddleware(s.handleListSeasonEpisodes, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/tv/seasons/{id}/missing", s.authMiddleware(s.handleSeasonMissingEpisodes, models.RoleUser))

	// Media
	s.router.HandleFunc("GET /api/v1/libraries/{id}/media", s.authMiddleware(s.handleListMedia, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/libraries/{id}/media/index", s.authMiddleware(s.handleMediaLetterIndex, models.RoleUser))
	// Media - Bulk operations (must be before /{id} routes)
	s.router.HandleFunc("PUT /api/v1/media/bulk", s.authMiddleware(s.handleBulkUpdateMedia, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/media/bulk-action", s.authMiddleware(s.handleBulkAction, models.RoleUser))

	s.router.HandleFunc("GET /api/v1/media/{id}", s.authMiddleware(s.handleGetMedia, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/media/{id}", s.authMiddleware(s.handleUpdateMedia, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/media/{id}/reset", s.authMiddleware(s.handleResetMediaLock, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/media/{id}/edition", s.authMiddleware(s.handleGetMediaEdition, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/media/{id}/editions", s.authMiddleware(s.handleGetMediaEditions, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/media/{id}/edition-parent", s.authMiddleware(s.handleSetEditionParent, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/media/{id}/edition-parent", s.authMiddleware(s.handleRemoveEditionParent, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/media/search", s.authMiddleware(s.handleSearchMedia, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/media/{id}/identify", s.authMiddleware(s.handleIdentifyMedia, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/media/{id}/apply-meta", s.authMiddleware(s.handleApplyMetadata, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/media/{id}/artwork", s.authMiddleware(s.handleGetMediaArtwork, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/media/{id}/artwork", s.authMiddleware(s.handleSetMediaArtwork, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/media/{id}/locked-fields", s.authMiddleware(s.handleGetLockedFields, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/media/{id}/locked-fields", s.authMiddleware(s.handleUpdateLockedFields, models.RoleAdmin))

	// Media - Performers
	s.router.HandleFunc("GET /api/v1/media/{id}/cast", s.authMiddleware(s.handleGetMediaCast, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/media/{id}/performers", s.authMiddleware(s.handleLinkPerformer, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/media/{id}/performers/{performerId}", s.authMiddleware(s.handleUnlinkPerformer, models.RoleAdmin))

	// Media - Tags
	s.router.HandleFunc("GET /api/v1/media/{id}/tags", s.authMiddleware(s.handleGetMediaTags, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/media/{id}/tags", s.authMiddleware(s.handleAssignTags, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/media/{id}/tags/{tagId}", s.authMiddleware(s.handleRemoveTag, models.RoleAdmin))

	// Media - Studios
	s.router.HandleFunc("POST /api/v1/media/{id}/studios", s.authMiddleware(s.handleLinkStudio, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/media/{id}/studios/{studioId}", s.authMiddleware(s.handleUnlinkStudio, models.RoleAdmin))

	// Streaming
	s.router.HandleFunc("GET /api/v1/stream/{mediaId}/info", s.authMiddleware(s.handleStreamInfo, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/stream/{mediaId}/master.m3u8", s.authMiddleware(s.handleStreamMaster, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/stream/{mediaId}/{quality}/{segment}", s.authMiddleware(s.handleStreamSegment, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/stream/{mediaId}/direct", s.authMiddleware(s.handleStreamDirect, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/stream/{mediaId}/subtitles/{id}", s.authMiddleware(s.handleStreamSubtitle, models.RoleUser))

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
	s.router.HandleFunc("POST /api/v1/collections/templates", s.authMiddleware(s.handleCreateCollectionTemplates, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/collections/{id}", s.authMiddleware(s.handleGetCollection, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/collections/{id}", s.authMiddleware(s.handleUpdateCollection, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/collections/{id}/evaluate", s.authMiddleware(s.handleEvaluateSmartCollection, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/collections/{id}/stats", s.authMiddleware(s.handleGetCollectionStats, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/collections/{id}/children", s.authMiddleware(s.handleListCollectionChildren, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/collections/{id}/items", s.authMiddleware(s.handleAddCollectionItem, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/collections/{id}/items/bulk", s.authMiddleware(s.handleBulkAddCollectionItems, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/collections/{id}/items/{itemId}", s.authMiddleware(s.handleRemoveCollectionItem, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/collections/{id}", s.authMiddleware(s.handleDeleteCollection, models.RoleUser))

	// Movie Series
	s.router.HandleFunc("GET /api/v1/series", s.authMiddleware(s.handleListSeries, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/series", s.authMiddleware(s.handleCreateSeries, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/series/{id}", s.authMiddleware(s.handleGetSeries, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/series/{id}", s.authMiddleware(s.handleUpdateSeries, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/series/{id}", s.authMiddleware(s.handleDeleteSeries, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/series/{id}/items", s.authMiddleware(s.handleAddSeriesItem, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/series/{id}/items/{itemId}", s.authMiddleware(s.handleRemoveSeriesItem, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/media/{id}/series", s.authMiddleware(s.handleGetMediaSeries, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/media/{id}/extras", s.authMiddleware(s.handleGetMediaExtras, models.RoleUser))

	// Watch history
	s.router.HandleFunc("POST /api/v1/watch/{mediaId}/progress", s.authMiddleware(s.handleUpdateProgress, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/watch/continue", s.authMiddleware(s.handleContinueWatching, models.RoleUser))

	// Profile settings (parental controls, kids mode, avatar)
	s.router.HandleFunc("GET /api/v1/profile/settings", s.authMiddleware(s.handleGetProfileSettings, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/profile/settings", s.authMiddleware(s.handleUpdateProfileSettings, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/users/{id}/settings", s.authMiddleware(s.handleAdminUpdateUserSettings, models.RoleAdmin))

	// Recommendations
	s.router.HandleFunc("GET /api/v1/recommendations", s.authMiddleware(s.handleRecommendations, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/recommendations/because-you-watched", s.authMiddleware(s.handleBecauseYouWatched, models.RoleUser))

	// Household profiles
	s.router.HandleFunc("GET /api/v1/household/profiles", s.authMiddleware(s.handleHouseholdProfiles, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/household/switch", s.authMiddleware(s.handleHouseholdSwitch, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/household/profiles", s.authMiddleware(s.handleCreateSubProfile, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/household/profiles/{id}", s.authMiddleware(s.handleUpdateSubProfile, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/household/profiles/{id}", s.authMiddleware(s.handleDeleteSubProfile, models.RoleUser))

	// Performers
	s.router.HandleFunc("GET /api/v1/performers", s.authMiddleware(s.handleListPerformers, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/performers", s.authMiddleware(s.handleCreatePerformer, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/performers/{id}", s.authMiddleware(s.handleGetPerformer, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/performers/{id}", s.authMiddleware(s.handleUpdatePerformer, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/performers/{id}", s.authMiddleware(s.handleDeletePerformer, models.RoleAdmin))

	// Tags
	s.router.HandleFunc("GET /api/v1/tags", s.authMiddleware(s.handleListTags, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/tags", s.authMiddleware(s.handleCreateTag, models.RoleAdmin))
	s.router.HandleFunc("PUT /api/v1/tags/{id}", s.authMiddleware(s.handleUpdateTag, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/tags/{id}", s.authMiddleware(s.handleDeleteTag, models.RoleAdmin))

	// Studios
	s.router.HandleFunc("GET /api/v1/studios", s.authMiddleware(s.handleListStudios, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/studios", s.authMiddleware(s.handleCreateStudio, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/studios/{id}", s.authMiddleware(s.handleGetStudio, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/studios/{id}", s.authMiddleware(s.handleUpdateStudio, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/studios/{id}", s.authMiddleware(s.handleDeleteStudio, models.RoleAdmin))

	// Duplicates
	s.router.HandleFunc("GET /api/v1/duplicates", s.authMiddleware(s.handleListDuplicates, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/duplicates/resolve", s.authMiddleware(s.handleResolveDuplicate, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/duplicates/resolve-bulk", s.authMiddleware(s.handleBulkResolveDuplicates, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/duplicates/count", s.authMiddleware(s.handleGetDuplicateCount, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/duplicates/clear-stale", s.authMiddleware(s.handleClearStaleDuplicates, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/media/{id}/duplicates", s.authMiddleware(s.handleGetMediaDuplicates, models.RoleAdmin))

	// Sort order
	s.router.HandleFunc("PATCH /api/v1/sort", s.authMiddleware(s.handleUpdateSortOrder, models.RoleAdmin))

	// Jobs
	s.router.HandleFunc("GET /api/v1/jobs", s.authMiddleware(s.handleListJobs, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/jobs/{id}", s.authMiddleware(s.handleGetJob, models.RoleAdmin))

	// Playback preferences
	s.router.HandleFunc("GET /api/v1/settings/playback", s.authMiddleware(s.handleGetPlaybackPrefs, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/settings/playback", s.authMiddleware(s.handleUpdatePlaybackPrefs, models.RoleUser))

	// Display preferences (per user — overlay badges)
	s.router.HandleFunc("GET /api/v1/settings/display", s.authMiddleware(s.handleGetDisplayPrefs, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/settings/display", s.authMiddleware(s.handleUpdateDisplayPrefs, models.RoleUser))

	// Skip preferences (per user)
	s.router.HandleFunc("GET /api/v1/settings/skip", s.authMiddleware(s.handleGetSkipPrefs, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/settings/skip", s.authMiddleware(s.handleUpdateSkipPrefs, models.RoleUser))

	// Media segments (skip markers)
	s.router.HandleFunc("GET /api/v1/media/{mediaId}/segments", s.authMiddleware(s.handleGetSegments, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/media/{mediaId}/segments", s.authMiddleware(s.handleUpsertSegment, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/media/{mediaId}/segments/{type}", s.authMiddleware(s.handleDeleteSegment, models.RoleAdmin))

	// Segment detection (admin trigger)
	s.router.HandleFunc("POST /api/v1/libraries/{id}/detect-segments", s.authMiddleware(s.handleDetectSegments, models.RoleAdmin))

	// System settings (admin only)
	s.router.HandleFunc("GET /api/v1/settings/system", s.authMiddleware(s.handleGetSystemSettings, models.RoleAdmin))
	s.router.HandleFunc("PUT /api/v1/settings/system", s.authMiddleware(s.handleUpdateSystemSettings, models.RoleAdmin))

	// Analytics (admin only)
	s.router.HandleFunc("GET /api/v1/analytics/overview", s.authMiddleware(s.handleAnalyticsOverview, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/analytics/streams", s.authMiddleware(s.handleAnalyticsStreams, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/analytics/streams/breakdown", s.authMiddleware(s.handleAnalyticsStreamBreakdown, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/analytics/watch-activity", s.authMiddleware(s.handleAnalyticsWatchActivity, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/analytics/users/activity", s.authMiddleware(s.handleAnalyticsUserActivity, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/analytics/transcodes", s.authMiddleware(s.handleAnalyticsTranscodes, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/analytics/system", s.authMiddleware(s.handleAnalyticsSystem, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/analytics/system/history", s.authMiddleware(s.handleAnalyticsSystemHistory, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/analytics/storage", s.authMiddleware(s.handleAnalyticsStorage, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/analytics/library-health", s.authMiddleware(s.handleAnalyticsLibraryHealth, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/analytics/trends", s.authMiddleware(s.handleAnalyticsTrends, models.RoleAdmin))

	// Profile stats
	s.router.HandleFunc("GET /api/v1/profile/stats", s.authMiddleware(s.handleGetProfileStats, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/profile/wrapped/{year}", s.authMiddleware(s.handleGetWrapped, models.RoleUser))

	// Discovery
	s.router.HandleFunc("GET /api/v1/discover/trending", s.authMiddleware(s.handleGetTrending, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/discover/genre/{slug}", s.authMiddleware(s.handleGetGenreHub, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/discover/decade/{year}", s.authMiddleware(s.handleGetDecadeHub, models.RoleUser))

	// Home page layout customization
	s.router.HandleFunc("GET /api/v1/settings/home-layout", s.authMiddleware(s.handleGetHomeLayout, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/settings/home-layout", s.authMiddleware(s.handleUpdateHomeLayout, models.RoleUser))

	// Content requests
	s.router.HandleFunc("POST /api/v1/requests", s.authMiddleware(s.handleCreateContentRequest, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/requests", s.authMiddleware(s.handleListContentRequests, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/requests/mine", s.authMiddleware(s.handleGetMyContentRequests, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/requests/{id}", s.authMiddleware(s.handleResolveContentRequest, models.RoleAdmin))

	// Watchlist
	s.router.HandleFunc("GET /api/v1/watchlist", s.authMiddleware(s.handleGetWatchlist, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/watchlist/{itemId}", s.authMiddleware(s.handleAddToWatchlist, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/watchlist/{itemId}", s.authMiddleware(s.handleRemoveFromWatchlist, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/watchlist/{itemId}/check", s.authMiddleware(s.handleCheckWatchlist, models.RoleUser))

	// User ratings
	s.router.HandleFunc("POST /api/v1/media/{id}/rating", s.authMiddleware(s.handleRateMedia, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/media/{id}/rating", s.authMiddleware(s.handleDeleteRating, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/media/{id}/rating", s.authMiddleware(s.handleGetUserRating, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/media/{id}/community-rating", s.authMiddleware(s.handleGetCommunityRating, models.RoleUser))

	// Favorites
	s.router.HandleFunc("POST /api/v1/favorites/{itemId}", s.authMiddleware(s.handleToggleFavorite, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/favorites/{itemId}/check", s.authMiddleware(s.handleCheckFavorite, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/favorites", s.authMiddleware(s.handleGetFavorites, models.RoleUser))

	// Playlists
	s.router.HandleFunc("GET /api/v1/playlists", s.authMiddleware(s.handleListPlaylists, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/playlists", s.authMiddleware(s.handleCreatePlaylist, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/playlists/{id}", s.authMiddleware(s.handleUpdatePlaylist, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/playlists/{id}", s.authMiddleware(s.handleDeletePlaylist, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/playlists/{id}/items", s.authMiddleware(s.handleGetPlaylistItems, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/playlists/{id}/items", s.authMiddleware(s.handleAddPlaylistItem, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/playlists/{id}/items/{itemId}", s.authMiddleware(s.handleRemovePlaylistItem, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/playlists/{id}/reorder", s.authMiddleware(s.handleReorderPlaylistItems, models.RoleUser))

	// On Deck
	s.router.HandleFunc("GET /api/v1/watch/on-deck", s.authMiddleware(s.handleOnDeck, models.RoleUser))

	// Saved filter presets
	s.router.HandleFunc("GET /api/v1/filters", s.authMiddleware(s.handleListSavedFilters, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/filters", s.authMiddleware(s.handleCreateSavedFilter, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/filters/{id}", s.authMiddleware(s.handleDeleteSavedFilter, models.RoleUser))

	// ── Phase 10: External Integrations ──

	// Trakt.tv
	s.router.HandleFunc("POST /api/v1/trakt/device-code", s.authMiddleware(s.handleTraktDeviceCode, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/trakt/activate", s.authMiddleware(s.handleTraktActivate, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/trakt/status", s.authMiddleware(s.handleTraktStatus, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/trakt/disconnect", s.authMiddleware(s.handleTraktDisconnect, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/trakt/scrobble", s.authMiddleware(s.handleTraktScrobble, models.RoleUser))

	// Last.fm
	s.router.HandleFunc("POST /api/v1/lastfm/connect", s.authMiddleware(s.handleLastfmConnect, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/lastfm/status", s.authMiddleware(s.handleLastfmStatus, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/lastfm/disconnect", s.authMiddleware(s.handleLastfmDisconnect, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/lastfm/scrobble", s.authMiddleware(s.handleLastfmScrobble, models.RoleUser))

	// Sonarr/Radarr/Lidarr webhooks
	s.router.HandleFunc("POST /api/v1/webhooks/arr", s.handleArrWebhook) // no auth — uses shared secret
	s.router.HandleFunc("GET /api/v1/admin/webhooks", s.authMiddleware(s.handleListWebhookSecrets, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/admin/webhooks", s.authMiddleware(s.handleCreateWebhookSecret, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/admin/webhooks/{id}", s.authMiddleware(s.handleDeleteWebhookSecret, models.RoleAdmin))

	// API keys
	s.router.HandleFunc("GET /api/v1/settings/api-keys", s.authMiddleware(s.handleListAPIKeys, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/settings/api-keys", s.authMiddleware(s.handleCreateAPIKey, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/settings/api-keys/{id}", s.authMiddleware(s.handleDeleteAPIKey, models.RoleUser))

	// Backup and restore
	s.router.HandleFunc("POST /api/v1/admin/backup", s.authMiddleware(s.handleCreateBackup, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/admin/backups", s.authMiddleware(s.handleListBackups, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/admin/backups/{id}/download", s.authMiddleware(s.handleDownloadBackup, models.RoleAdmin))

	// Plex/Jellyfin import
	s.router.HandleFunc("POST /api/v1/admin/import", s.authMiddleware(s.handleStartImport, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/admin/imports", s.authMiddleware(s.handleListImports, models.RoleAdmin))

	// ── Phase 11: Security ──

	// 2FA / TOTP
	s.router.HandleFunc("POST /api/v1/auth/2fa/setup", s.authMiddleware(s.handle2FASetup, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/auth/2fa/verify", s.authMiddleware(s.handle2FAVerify, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/auth/2fa", s.authMiddleware(s.handle2FADisable, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/auth/2fa/status", s.authMiddleware(s.handle2FAStatus, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/auth/2fa/validate", s.rlAuth(s.handle2FAValidate)) // no auth — called during login

	// OpenAPI spec & docs (P10-05)
	s.router.HandleFunc("GET /api/v1/openapi.json", s.handleOpenAPISpec)

	// Watch Together / SyncPlay (P12-01)
	s.router.HandleFunc("POST /api/v1/sync/create", s.authMiddleware(s.handleSyncCreate, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/sync/join", s.authMiddleware(s.handleSyncJoin, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/sync/{id}/action", s.authMiddleware(s.handleSyncAction, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/sync/{id}/chat", s.authMiddleware(s.handleSyncChat, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/sync/{id}", s.authMiddleware(s.handleSyncInfo, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/sync/{id}", s.authMiddleware(s.handleSyncEnd, models.RoleUser))

	// Cinema mode (P12-02)
	s.router.HandleFunc("GET /api/v1/cinema/pre-rolls", s.authMiddleware(s.handleListPreRolls, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/cinema/pre-rolls", s.authMiddleware(s.handleCreatePreRoll, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/cinema/pre-rolls/{id}", s.authMiddleware(s.handleDeletePreRoll, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/cinema/queue/{mediaId}", s.authMiddleware(s.handleCinemaQueue, models.RoleUser))

	// DASH streaming (P12-04)
	s.router.HandleFunc("GET /api/v1/stream/{mediaId}/manifest.mpd", s.authMiddleware(s.handleStreamDASH, models.RoleUser))

	// Lyrics (P13-03)
	s.router.HandleFunc("GET /api/v1/media/{id}/lyrics", s.authMiddleware(s.handleGetLyrics, models.RoleUser))

	// DLNA (P14-01)
	s.router.HandleFunc("GET /api/v1/dlna/config", s.authMiddleware(s.handleDLNAConfig, models.RoleAdmin))
	s.router.HandleFunc("PUT /api/v1/dlna/config", s.authMiddleware(s.handleUpdateDLNAConfig, models.RoleAdmin))
	s.router.HandleFunc("GET /dlna/description.xml", s.handleDLNADescription)
	s.router.HandleFunc("GET /dlna/content/{id}", s.handleDLNAContent)

	// Chromecast (P14-02)
	s.router.HandleFunc("POST /api/v1/cast/session", s.authMiddleware(s.handleCastSession, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/cast/sessions", s.authMiddleware(s.handleListCastSessions, models.RoleUser))
	s.router.HandleFunc("DELETE /api/v1/cast/{id}", s.authMiddleware(s.handleEndCastSession, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/cast/session/{id}/command", s.authMiddleware(s.handleCastCommand, models.RoleUser))

	// Scene markers (P15-02)
	s.router.HandleFunc("GET /api/v1/media/{id}/markers", s.authMiddleware(s.handleGetMarkers, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/media/{id}/markers", s.authMiddleware(s.handleCreateMarker, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/markers/{id}", s.authMiddleware(s.handleDeleteMarker, models.RoleAdmin))

	// Extended performer metadata (P15-03)
	s.router.HandleFunc("GET /api/v1/performers/{id}/extended", s.authMiddleware(s.handleGetPerformerExtended, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/performers/{id}/extended", s.authMiddleware(s.handleUpdatePerformerExtended, models.RoleAdmin))

	// Per-user streaming limits (P15-04)
	s.router.HandleFunc("GET /api/v1/admin/users/{id}/stream-limits", s.authMiddleware(s.handleGetStreamLimits, models.RoleAdmin))
	s.router.HandleFunc("PUT /api/v1/admin/users/{id}/stream-limits", s.authMiddleware(s.handleUpdateStreamLimits, models.RoleAdmin))

	// Live TV / DVR (P15-05)
	s.router.HandleFunc("GET /api/v1/livetv/tuners", s.authMiddleware(s.handleListTuners, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/livetv/tuners", s.authMiddleware(s.handleCreateTuner, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/livetv/tuners/{id}", s.authMiddleware(s.handleDeleteTuner, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/livetv/epg", s.authMiddleware(s.handleGetEPG, models.RoleUser))
	s.router.HandleFunc("POST /api/v1/livetv/recordings", s.authMiddleware(s.handleScheduleRecording, models.RoleUser))
	s.router.HandleFunc("GET /api/v1/livetv/recordings", s.authMiddleware(s.handleListRecordings, models.RoleUser))

	// Anime info (P15-01)
	s.router.HandleFunc("GET /api/v1/media/{id}/anime-info", s.authMiddleware(s.handleGetAnimeInfo, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/media/{id}/anime-info", s.authMiddleware(s.handleUpdateAnimeInfo, models.RoleAdmin))

	// Comics / eBooks reading progress (P15-06)
	s.router.HandleFunc("GET /api/v1/media/{id}/reading-progress", s.authMiddleware(s.handleGetReadingProgress, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/media/{id}/reading-progress", s.authMiddleware(s.handleUpdateReadingProgress, models.RoleUser))

	// Notifications (admin only)
	s.router.HandleFunc("GET /api/v1/notifications/channels", s.authMiddleware(s.handleListNotificationChannels, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/notifications/channels", s.authMiddleware(s.handleCreateNotificationChannel, models.RoleAdmin))
	s.router.HandleFunc("PUT /api/v1/notifications/channels/{id}", s.authMiddleware(s.handleUpdateNotificationChannel, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/notifications/channels/{id}", s.authMiddleware(s.handleDeleteNotificationChannel, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/notifications/channels/{id}/test", s.authMiddleware(s.handleTestNotificationChannel, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/notifications/alerts", s.authMiddleware(s.handleListAlertRules, models.RoleAdmin))
	s.router.HandleFunc("POST /api/v1/notifications/alerts", s.authMiddleware(s.handleCreateAlertRule, models.RoleAdmin))
	s.router.HandleFunc("PUT /api/v1/notifications/alerts/{id}", s.authMiddleware(s.handleUpdateAlertRule, models.RoleAdmin))
	s.router.HandleFunc("DELETE /api/v1/notifications/alerts/{id}", s.authMiddleware(s.handleDeleteAlertRule, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/notifications/log", s.authMiddleware(s.handleGetAlertLog, models.RoleAdmin))
}

// ──────────────────── Middleware ────────────────────

func (s *Server) authMiddleware(next http.HandlerFunc, requiredRole models.UserRole) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check X-API-Key first (P10-04)
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			if userID, role := s.validateAPIKey(apiKey); userID != "" {
				if !s.auth.CheckPermission(models.UserRole(role), requiredRole) {
					s.respondError(w, http.StatusForbidden, "insufficient permissions")
					return
				}
				r.Header.Set("X-User-ID", userID)
				r.Header.Set("X-User-Role", role)
				next(w, r)
				return
			}
			s.respondError(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		tokenString := ""
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		} else if t := r.URL.Query().Get("token"); t != "" {
			// Allow token via query param for streaming endpoints (video elements can't set headers)
			tokenString = t
		} else {
			s.respondError(w, http.StatusUnauthorized, "missing authorization")
			return
		}

		claims, err := s.auth.ValidateToken(tokenString)
		if err != nil {
			s.respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		// Check if session has been revoked (logout/admin revoke)
		if !s.isSessionValid(tokenString) {
			s.respondError(w, http.StatusUnauthorized, "session revoked")
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

// validateAPIKey checks an API key and returns (userID, role) if valid.
func (s *Server) validateAPIKey(apiKey string) (string, string) {
	hash := sha256Sum(apiKey)
	var userID, role string
	err := s.db.QueryRow(`SELECT ak.user_id, u.role FROM api_keys ak
		JOIN users u ON ak.user_id = u.id
		WHERE ak.key_hash = $1 AND (ak.expires_at IS NULL OR ak.expires_at > NOW())`, hash).
		Scan(&userID, &role)
	if err != nil {
		return "", ""
	}
	// Update last_used_at
	go s.db.Exec("UPDATE api_keys SET last_used_at = NOW() WHERE key_hash = $1", hash)
	return userID, role
}

func sha256Sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
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

// respondPaginated sends a JSON response with pagination headers (X-Total-Count, Link).
func (s *Server) respondPaginated(w http.ResponseWriter, statusCode int, data interface{}, page, pageSize, total int, baseURL string) {
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	lastPage := (total + pageSize - 1) / pageSize
	if lastPage < 1 {
		lastPage = 1
	}
	sep := "?"
	if strings.Contains(baseURL, "?") {
		sep = "&"
	}
	pageSizeStr := strconv.Itoa(pageSize)
	var links []string
	links = append(links, fmt.Sprintf(`<%s%spage=1&page_size=%s>; rel="first"`, baseURL, sep, pageSizeStr))
	links = append(links, fmt.Sprintf(`<%s%spage=%d&page_size=%s>; rel="last"`, baseURL, sep, lastPage, pageSizeStr))
	if page < lastPage {
		links = append(links, fmt.Sprintf(`<%s%spage=%d&page_size=%s>; rel="next"`, baseURL, sep, page+1, pageSizeStr))
	}
	if page > 1 {
		links = append(links, fmt.Sprintf(`<%s%spage=%d&page_size=%s>; rel="prev"`, baseURL, sep, page-1, pageSizeStr))
	}
	w.Header().Set("Link", strings.Join(links, ", "))
	s.respondJSON(w, statusCode, data)
}

// respondWithETag sends a JSON response with ETag support; returns 304 if If-None-Match matches.
func (s *Server) respondWithETag(w http.ResponseWriter, r *http.Request, statusCode int, data interface{}) {
	body, err := json.Marshal(data)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to marshal response")
		return
	}
	h := sha256.Sum256(body)
	etag := hex.EncodeToString(h[:])[:16]
	quotedETag := fmt.Sprintf(`"%s"`, etag)
	w.Header().Set("ETag", quotedETag)
	if match := r.Header.Get("If-None-Match"); match != "" {
		for _, v := range strings.Split(match, ",") {
			if strings.TrimSpace(v) == quotedETag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

func (s *Server) getUserID(r *http.Request) uuid.UUID {
	id, _ := uuid.Parse(r.Header.Get("X-User-ID"))
	return id
}

func (s *Server) Start() error {
	// Wrap router with global middleware: security headers → CORS → handler
	handler := s.securityHeadersMiddleware(s.corsMiddleware(s.router))
	return http.ListenAndServe(s.config.Server.Address(), handler)
}

// securityHeadersMiddleware adds standard security headers to all responses.
func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("X-XSS-Protection", "0")
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware handles CORS preflight and response headers globally.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Requested-With")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Set("Vary", "Origin")
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
