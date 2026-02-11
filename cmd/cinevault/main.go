package main

import (
	"context"
	"fmt"
	"log"

	"github.com/JustinTDCT/CineVault/internal/api"
	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/db"
	"github.com/JustinTDCT/CineVault/internal/detection"
	"github.com/JustinTDCT/CineVault/internal/fingerprint"
	"github.com/JustinTDCT/CineVault/internal/jobs"
)

const banner = `
   _____ _            __      __          _ _   
  / ____(_)           \ \    / /         | | |  
 | |     _ _ __   ___  \ \  / /_ _ _   _| | |_ 
 | |    | | '_ \ / _ \  \ \/ / _' | | | | | __|
 | |____| | | | |  __/   \  / (_| | |_| | | |_ 
  \_____|_|_| |_|\___|    \/ \__,_|\__,_|_|\__|
                                                
  Self-Hosted Media Server - Phase 3
  Version 0.46.0
`

func main() {
	fmt.Println(banner)
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
	fp := fingerprint.NewFingerprinter(cfg.FFmpeg.FFmpegPath, cfg.Paths.Preview)

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

	addr := cfg.Server.Address()
	log.Printf("Server starting on http://%s\n", addr)
	log.Printf("WebSocket available at ws://%s/api/v1/ws\n", addr)
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
