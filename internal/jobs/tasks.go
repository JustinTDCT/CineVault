package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/JustinTDCT/CineVault/internal/models"
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

type MetadataPayload struct {
	MediaItemID string `json:"media_item_id"`
	Source      string `json:"source"`
}

// ──────── Handlers ────────

type ScanHandler struct {
	scanner  *scanner.Scanner
	libRepo  *repository.LibraryRepository
	jobRepo  *repository.JobRepository
	notifier EventNotifier
}

type EventNotifier interface {
	Broadcast(event string, data interface{})
}

func NewScanHandler(sc *scanner.Scanner, libRepo *repository.LibraryRepository, jobRepo *repository.JobRepository, notifier EventNotifier) *ScanHandler {
	return &ScanHandler{scanner: sc, libRepo: libRepo, jobRepo: jobRepo, notifier: notifier}
}

func (h *ScanHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p ScanPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	libID, _ := uuid.Parse(p.LibraryID)
	library, err := h.libRepo.GetByID(libID)
	if err != nil {
		return fmt.Errorf("get library: %w", err)
	}

	log.Printf("Job: scanning library %q", library.Name)
	if h.notifier != nil {
		h.notifier.Broadcast("scan:start", map[string]string{"library_id": p.LibraryID, "name": library.Name})
	}

	result, err := h.scanner.ScanLibrary(library)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	_ = h.libRepo.UpdateLastScan(libID)

	log.Printf("Job: scan complete - %d found, %d added", result.FilesFound, result.FilesAdded)
	if h.notifier != nil {
		h.notifier.Broadcast("scan:complete", map[string]interface{}{
			"library_id": p.LibraryID,
			"result":     result,
		})
	}

	return nil
}

// ──────── Fingerprint Handler (stub) ────────

type FingerprintHandler struct {
	mediaRepo *repository.MediaRepository
}

func NewFingerprintHandler(mediaRepo *repository.MediaRepository) *FingerprintHandler {
	return &FingerprintHandler{mediaRepo: mediaRepo}
}

func (h *FingerprintHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p FingerprintPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	log.Printf("Job: fingerprinting media %s", p.MediaItemID)
	// Fingerprinting logic will be wired in the fingerprint package
	return nil
}

// ──────── Preview Handler (stub) ────────

type PreviewHandler struct{}

func NewPreviewHandler() *PreviewHandler {
	return &PreviewHandler{}
}

func (h *PreviewHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p PreviewPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	log.Printf("Job: generating preview for %s", p.MediaItemID)
	return nil
}

// ──────── Register all handlers ────────

func RegisterHandlers(q *Queue, sc *scanner.Scanner, libRepo *repository.LibraryRepository,
	mediaRepo *repository.MediaRepository, jobRepo *repository.JobRepository, notifier EventNotifier) {

	q.RegisterHandler(TaskScanLibrary, NewScanHandler(sc, libRepo, jobRepo, notifier))
	q.RegisterHandler(TaskFingerprint, NewFingerprintHandler(mediaRepo))
	q.RegisterHandler(TaskGeneratePreview, NewPreviewHandler())
	q.RegisterHandler(TaskMetadataScrape, asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		var p MetadataPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		log.Printf("Job: scraping metadata for %s from %s", p.MediaItemID, p.Source)
		return nil
	}))

	// Ignore unused import
	_ = models.JobPending
}
