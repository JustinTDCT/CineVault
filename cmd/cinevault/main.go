package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/JustinTDCT/CineVault/internal/api"
	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/db"
	"github.com/JustinTDCT/CineVault/internal/version"
)

func main() {
	ver := version.Load()
	log.Printf("CineVault %s starting...", ver.Version)

	cfg := config.Load()

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database, "migrations"); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	cfg.MergeFromDB(database)
	log.Printf("cache server enabled=%v, key_len=%d, url=%s", cfg.CacheServerEnabled(), len(cfg.CacheServerKey), config.CacheServerURL)

	srv := api.NewServer(database, cfg)

	httpServer := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      srv,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("listening on :%d", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
}
