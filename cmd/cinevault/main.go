// Command cinevault runs the CineVault media server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/db"
	"github.com/JustinTDCT/CineVault/internal/match"
	"github.com/JustinTDCT/CineVault/internal/mediainfo"
	"github.com/JustinTDCT/CineVault/internal/probe"
	"github.com/JustinTDCT/CineVault/internal/scan"
	"github.com/JustinTDCT/CineVault/internal/server"
	"github.com/JustinTDCT/CineVault/internal/tmdb"
	"github.com/JustinTDCT/CineVault/internal/transcode"
	"github.com/JustinTDCT/CineVault/internal/webui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("cinevault stopped", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dbAddr, _ := config.DatabaseAddress(cfg.DatabaseURL)

	log.Info("starting cinevault", "version", version, "listen", cfg.Listen)

	startCtx, cancelStart := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStart()
	database, err := db.Open(startCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database at %s: %w", dbAddr, err)
	}
	defer database.Close()

	mig, err := database.Migrate(startCtx)
	if err != nil {
		return err
	}
	for _, m := range mig.Applied {
		log.Info("applied database migration", "version", m.Version, "name", m.Name)
	}
	log.Info("database ready", "address", dbAddr, "schema_version", mig.Current)

	caps := probeCapabilities(cfg)
	logCapabilities(log, caps)

	scanner := scan.Scanner{
		Store:  database,
		Prober: mediainfo.FFprobe{Path: cfg.FFprobePath, Run: probe.ExecRunner},
	}

	pref, err := transcode.ParsePreference(cfg.TranscodeMode, cfg.TranscodeGPU)
	if err != nil {
		return err
	}
	enc, note, encOK := transcode.SelectH264(caps.FFmpeg.Encoders, pref)
	switch {
	case !encOK:
		log.Warn("playback: no working H.264 encoder; only files the player supports will play", "reason", note)
	case note != "":
		log.Warn("playback", "encoder", enc.Name, "device", enc.DeviceName, "note", note)
	default:
		log.Info("playback", "encoder", enc.Name, "device", enc.DeviceName, "mode", pref.Mode)
	}
	streamer := &transcode.Manager{FFmpeg: cfg.FFmpegPath, WorkDir: cfg.TranscodeDir, Log: log}
	if err := prepareTranscodeDir(cfg.TranscodeDir); err != nil {
		return err
	}

	var matchFn func(context.Context, int64) (match.Result, error)
	if cfg.TMDbToken != "" {
		matchFn = match.Matcher{Store: database, Source: tmdb.New(cfg.TMDbToken)}.MatchLibrary
		log.Info("metadata", "source", "TMDb")
	} else {
		log.Warn("metadata: CINEVAULT_TMDB_TOKEN not set; titles won't be matched to TMDb")
	}

	api := server.New(server.Options{
		Version:       version,
		DBAddr:        dbAddr,
		Caps:          caps,
		SchemaVersion: mig.Current,
		PingDB:        database.Ping,
		EditionTypes:  database.EditionTypes,
		Libraries:     database,
		Scan:          scanner.Scan,
		Match:         matchFn,
		UI:            webui.Handler(),
		Playback: &server.Playback{
			Versions: database, Streamer: streamer,
			Encoder: enc, EncoderOK: encOK, Note: note, MaxHeight: 1080,
		},
		Log: log,
	})
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	reaperDone := make(chan struct{})
	go func() { streamer.RunReaper(ctx); close(reaperDone) }()
	defer func() { stop(); <-reaperDone }() // stops FFmpeg and removes session files

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "url", "http://"+displayAddr(cfg.Listen)+"/")

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}

func probeCapabilities(cfg config.Config) server.Capabilities {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ProbeTimeout)
	defer cancel()
	gpus := probe.NvidiaGPUs(ctx, probe.ExecRunner)
	return server.Capabilities{
		FFmpeg: probe.FFmpeg(ctx, probe.ExecRunner, cfg.FFmpegPath, gpus),
		GPUs:   gpus,
	}
}

func logCapabilities(log *slog.Logger, caps server.Capabilities) {
	for _, g := range caps.GPUs {
		log.Info("gpu", "index", g.Index, "name", g.Name, "driver", g.Driver, "memory_mib", g.MemoryMiB)
	}
	if !caps.FFmpeg.Found {
		log.Warn("ffmpeg unavailable; transcoding disabled", "error", caps.FFmpeg.Error)
		return
	}
	var usable, unusable []string
	for _, e := range caps.FFmpeg.Encoders {
		name := e.Name
		if e.Kind == probe.KindNVENC && e.GPU >= 0 {
			name = fmt.Sprintf("%s@gpu%d", e.Name, e.GPU)
		}
		if e.Usable {
			usable = append(usable, name)
		} else {
			unusable = append(unusable, name)
		}
	}
	log.Info("ffmpeg", "version", caps.FFmpeg.Version,
		"encoders", strings.Join(usable, ","),
		"listed_but_not_working", strings.Join(unusable, ","))
}

func displayAddr(listen string) string {
	if strings.HasPrefix(listen, ":") {
		return "localhost" + listen
	}
	return listen
}

// prepareTranscodeDir creates the folder for streaming segments and removes
// session folders left by a previous run. Only folders named like session IDs
// (32 hex characters) are removed, so a misconfigured path can't lose data.
func prepareTranscodeDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("transcode folder %s: %w", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("transcode folder %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() && sessionDirName.MatchString(e.Name()) {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}

var sessionDirName = regexp.MustCompile(`^[0-9a-f]{32}$`)
