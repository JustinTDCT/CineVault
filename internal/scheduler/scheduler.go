package scheduler

import (
	"log"
	"time"

	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/google/uuid"
)

// OnScanDue is called when a library is due for a scheduled scan.
type OnScanDue func(libraryID uuid.UUID)

// Scheduler checks for libraries due for scans on a regular interval.
type Scheduler struct {
	libRepo  *repository.LibraryRepository
	callback OnScanDue
	interval time.Duration
	stop     chan struct{}
}

// New creates a new scheduled scan checker.
func New(libRepo *repository.LibraryRepository, cb OnScanDue) *Scheduler {
	return &Scheduler{
		libRepo:  libRepo,
		callback: cb,
		interval: 60 * time.Second,
		stop:     make(chan struct{}),
	}
}

// Start begins the ticker loop.
func (s *Scheduler) Start() {
	go s.run()
	log.Println("[scheduler] scheduled scan checker started (60s interval)")
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) run() {
	// Initial check after a short delay
	time.Sleep(10 * time.Second)
	s.check()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.check()
		case <-s.stop:
			log.Println("[scheduler] scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) check() {
	libs, err := s.libRepo.GetDueForScan()
	if err != nil {
		log.Printf("[scheduler] error checking due libraries: %v", err)
		return
	}

	for _, lib := range libs {
		log.Printf("[scheduler] library %q is due for scan", lib.Name)

		// Advance next_scan_at immediately to prevent re-trigger
		if err := s.libRepo.AdvanceNextScan(lib.ID); err != nil {
			log.Printf("[scheduler] error advancing next_scan_at for %s: %v", lib.Name, err)
		}

		s.callback(lib.ID)
	}
}
