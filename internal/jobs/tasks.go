package jobs

import (
	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/detection"
	"github.com/JustinTDCT/CineVault/internal/fingerprint"
	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/JustinTDCT/CineVault/internal/scanner"
)

// ──────── Payloads ────────

type ScanPayload struct {
	LibraryID string `json:"library_id"`
}

type FingerprintPayload struct {
	MediaItemID string `json:"media_item_id"`
}

type PreviewPayload struct {
	MediaItemID string `json:"media_item_id"`
}

type PhashLibraryPayload struct {
	LibraryID string `json:"library_id"`
}

type SpritesLibraryPayload struct {
	LibraryID string `json:"library_id"`
}

type PreviewsLibraryPayload struct {
	LibraryID string `json:"library_id"`
	Rebuild   bool   `json:"rebuild,omitempty"`
}

type MetadataPayload struct {
	MediaItemID string `json:"media_item_id"`
	Source      string `json:"source"`
}

type LoudnessLibraryPayload struct {
	LibraryID string `json:"library_id"`
}

type DetectSegmentsPayload struct {
	LibraryID string `json:"library_id"`
}

type EventNotifier interface {
	Broadcast(event string, data interface{})
}

// ──────── Register all handlers ────────

func RegisterHandlers(q *Queue, sc *scanner.Scanner, libRepo *repository.LibraryRepository,
	mediaRepo *repository.MediaRepository, jobRepo *repository.JobRepository,
	fp *fingerprint.Fingerprinter, notifier EventNotifier,
	scrapers []metadata.Scraper, settingsRepo *repository.SettingsRepository, cfg *config.Config,
	det *detection.Detector, segRepo *repository.SegmentRepository) {

	q.RegisterHandler(TaskScanLibrary, NewScanHandler(sc, libRepo, jobRepo, settingsRepo, q, notifier))
	q.RegisterHandler(TaskFingerprint, NewFingerprintHandler(mediaRepo))
	q.RegisterHandler(TaskPhashLibrary, NewPhashLibraryHandler(mediaRepo, settingsRepo, fp, notifier))
	q.RegisterHandler(TaskSpritesLibrary, NewSpritesLibraryHandler(mediaRepo, libRepo, settingsRepo, sc, notifier))
	q.RegisterHandler(TaskPreviewsLibrary, NewPreviewsLibraryHandler(mediaRepo, libRepo, settingsRepo, sc, notifier))
	q.RegisterHandler(TaskGeneratePreview, NewPreviewHandler())
	q.RegisterHandler(TaskMetadataScrape, NewMetadataScrapeHandler(mediaRepo, libRepo, settingsRepo, scrapers, cfg, sc, notifier))
	q.RegisterHandler(TaskMetadataRefresh, NewMetadataRefreshHandler(mediaRepo, libRepo, settingsRepo, scrapers, cfg, sc, notifier))
	q.RegisterHandler(TaskDetectSegments, NewDetectSegmentsHandler(det, segRepo, libRepo, notifier))
	q.RegisterHandler(TaskLoudnessLibrary, NewLoudnessLibraryHandler(mediaRepo, libRepo, notifier, cfg.FFmpeg.FFmpegPath))
}
