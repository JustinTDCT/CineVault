package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/JustinTDCT/CineVault/internal/analytics"
	"github.com/JustinTDCT/CineVault/internal/api"
	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/db"
	"github.com/JustinTDCT/CineVault/internal/detection"
	"github.com/JustinTDCT/CineVault/internal/fingerprint"
	"github.com/JustinTDCT/CineVault/internal/jobs"
	"github.com/JustinTDCT/CineVault/internal/notifications"
	"github.com/JustinTDCT/CineVault/internal/scheduler"
	"github.com/JustinTDCT/CineVault/internal/version"
	"github.com/JustinTDCT/CineVault/internal/watcher"
	"github.com/google/uuid"
)

const bannerArt = `
   _____ _            __      __          _ _   
  / ____(_)           \ \    / /         | | |  
 | |     _ _ __   ___  \ \  / /_ _ _   _| | |_ 
 | |    | | '_ \ / _ \  \ \/ / _' | | | | | __|
 | |____| | | | |  __/   \  / (_| | |_| | | |_ 
  \_____|_|_| |_|\___|    \/ \__,_|\__,_|_|\__|
`

func main() {
	v := version.Get()
	fmt.Println(bannerArt)
	fmt.Printf("  Self-Hosted Media Server - Phase %d\n", v.Phase)
	fmt.Printf("  Version %s\n\n", v.Version)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	database, err := db.Connect(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	log.Println("Database connected")

	// Initialize job queue
	redisAddr := cfg.Redis.Address()
	jobQueue := jobs.NewQueue(redisAddr)
	log.Println("Job queue initialized")

	// Create server
	server, err := api.NewServer(cfg, database, jobQueue)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Initialize fingerprinter for phash computation
	fp := fingerprint.NewFingerprinter(cfg.FFmpeg.FFmpegPath, cfg.Paths.Preview, cfg.FFmpeg.HWAccel)

	// Initialize segment detector
	det := detection.NewDetector(cfg.FFmpeg.FFmpegPath)

	// Register job handlers
	jobs.RegisterHandlers(jobQueue, server.Scanner(), server.LibRepo(),
		server.MediaRepo(), server.JobRepo(), fp, server.WSHub(),
		server.Scrapers(), server.SettingsRepo(), server.Config(),
		det, server.SegmentRepo())

	// Start job queue worker in background
	go func() {
		if err := jobQueue.Start(context.Background()); err != nil {
			log.Printf("Job queue worker error: %v", err)
		}
	}()
	defer jobQueue.Stop()

	// Start transcode session cleanup (every 5m, expire after 30m idle)
	transcodeCleanupStop := make(chan struct{})
	server.Transcoder().RunCleanupLoop(transcodeCleanupStop, 30*time.Minute)
	defer close(transcodeCleanupStop)

	// Start analytics collector (system metrics every 60s)
	collector := analytics.NewCollector(server.AnalyticsRepo(), server.Transcoder(), []string{cfg.Paths.Media})
	go collector.Start()
	defer collector.Stop()

	// Start daily stats rollup scheduler
	rollupStop := make(chan struct{})
	go analytics.StartRollupScheduler(server.AnalyticsRepo(), rollupStop)
	defer close(rollupStop)

	// Start alert evaluator (checks rules every 5m)
	alertEval := notifications.NewAlertEvaluator(server.AnalyticsRepo(), server.NotificationRepo(), server.WebhookSender())
	go alertEval.Start()
	defer alertEval.Stop()

	// Start filesystem watcher for real-time library updates
	fsWatcher, err := watcher.New(server.LibRepo(), func(libraryID uuid.UUID, path string, isCreate bool) {
		if isCreate {
			lib, err := server.LibRepo().GetByID(libraryID)
			if err != nil {
				log.Printf("[watcher] library lookup error: %v", err)
				return
			}
			if err := server.Scanner().ScanSingleFile(lib, path); err != nil {
				log.Printf("[watcher] scan error for %s: %v", path, err)
			}
		} else {
			if err := server.MediaRepo().MarkUnavailable(path); err != nil {
				log.Printf("[watcher] mark unavailable error for %s: %v", path, err)
			}
		}
	})
	if err != nil {
		log.Printf("Filesystem watcher failed to start: %v", err)
	} else {
		fsWatcher.Start()
		defer fsWatcher.Stop()
	}

	// Start scheduled scan checker (every 60s)
	scanScheduler := scheduler.New(server.LibRepo(), func(libraryID uuid.UUID) {
		_, err := jobQueue.EnqueueUnique(jobs.TaskScanLibrary,
			map[string]string{"library_id": libraryID.String()},
			"scheduled-scan-"+libraryID.String())
		if err != nil {
			log.Printf("[scheduler] enqueue scan error: %v", err)
		}
	})
	scanScheduler.Start()
	defer scanScheduler.Stop()

	addr := cfg.Server.Address()
	log.Printf("Server starting on http://%s\n", addr)
	log.Printf("WebSocket available at ws://%s/api/v1/ws\n", addr)
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
