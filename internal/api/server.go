package api

import (
	"encoding/json"
	"net/http"
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
	segmentRepo      *repository.SegmentRepository
	analyticsRepo    *repository.AnalyticsRepository
	notificationRepo *repository.NotificationRepository
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
	posterDir := cfg.Paths.Preview
	sc := scanner.NewScanner(cfg.FFmpeg.FFprobePath, cfg.FFmpeg.FFmpegPath, mediaRepo, tvRepo, musicRepo, audiobookRepo, galleryRepo, tagRepo, performerRepo, settingsRepo, sisterRepo, seriesRepo, scrapers, posterDir)
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
	previewFS := http.StripPrefix("/previews/", http.FileServer(http.Dir(s.config.Paths.Preview)))
	s.router.Handle("/previews/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		previewFS.ServeHTTP(w, r)
	}))

	// Public
	s.router.HandleFunc("GET /health", s.handleHealth)
	s.router.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.router.HandleFunc("GET /api/v1/setup/check", s.handleSetupCheck)
	s.router.HandleFunc("POST /api/v1/setup", s.handleSetup)
	s.router.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.router.HandleFunc("POST /api/v1/auth/login", s.handleLogin)

	// Fast Login (public — no auth required)
	s.router.HandleFunc("GET /api/v1/auth/fast-login/settings", s.handleFastLoginSettings)
	s.router.HandleFunc("GET /api/v1/auth/fast-login/users", s.handleFastLoginUsers)
	s.router.HandleFunc("POST /api/v1/auth/fast-login", s.handlePinLogin)

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
	s.router.HandleFunc("GET /api/v1/duplicates/count", s.authMiddleware(s.handleGetDuplicateCount, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/media/{id}/duplicates", s.authMiddleware(s.handleGetMediaDuplicates, models.RoleAdmin))

	// Sort order
	s.router.HandleFunc("PATCH /api/v1/sort", s.authMiddleware(s.handleUpdateSortOrder, models.RoleAdmin))

	// Jobs
	s.router.HandleFunc("GET /api/v1/jobs", s.authMiddleware(s.handleListJobs, models.RoleAdmin))
	s.router.HandleFunc("GET /api/v1/jobs/{id}", s.authMiddleware(s.handleGetJob, models.RoleAdmin))

	// Playback preferences
	s.router.HandleFunc("GET /api/v1/settings/playback", s.authMiddleware(s.handleGetPlaybackPrefs, models.RoleUser))
	s.router.HandleFunc("PUT /api/v1/settings/playback", s.authMiddleware(s.handleUpdatePlaybackPrefs, models.RoleUser))

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
