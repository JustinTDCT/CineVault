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

	"github.com/JustinTDCT/CineVault/internal/cachesync"
	"github.com/JustinTDCT/CineVault/internal/comic"
	"github.com/JustinTDCT/CineVault/internal/comicvine"
	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/db"
	"github.com/JustinTDCT/CineVault/internal/match"
	"github.com/JustinTDCT/CineVault/internal/mediainfo"
	"github.com/JustinTDCT/CineVault/internal/musicbrainz"
	"github.com/JustinTDCT/CineVault/internal/openlibrary"
	"github.com/JustinTDCT/CineVault/internal/probe"
	"github.com/JustinTDCT/CineVault/internal/scan"
	"github.com/JustinTDCT/CineVault/internal/server"
	"github.com/JustinTDCT/CineVault/internal/thumb"
	"github.com/JustinTDCT/CineVault/internal/tmdb"
	"github.com/JustinTDCT/CineVault/internal/transcode"
	"github.com/JustinTDCT/CineVault/internal/webui"
	"github.com/JustinTDCT/CineVault/pkg/cacheapi"
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

	comics := &comic.Reader{CacheDir: filepath.Join(cfg.CacheDir, "comics")}
	if comics.Available() {
		log.Info("comics", "cbr_cb7", "bsdtar")
	} else {
		log.Warn("comics: CBR and CB7 files need bsdtar; CBZ works without it", "install", "sudo apt install libarchive-tools")
	}
	scanner := scan.Scanner{
		Store:  database,
		Prober: mediainfo.FFprobe{Path: cfg.FFprobePath, Run: probe.ExecRunner},
		Comics: comics,
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
	streamer := &transcode.Manager{
		FFmpeg: cfg.FFmpegPath, WorkDir: cfg.TranscodeDir, Log: log,
		PauseAhead: cfg.PauseAhead, ResumeAhead: cfg.ResumeAhead, KeepBehind: cfg.KeepBehind,
	}
	if err := prepareTranscodeDir(cfg.TranscodeDir); err != nil {
		return err
	}

	cache := &cachesync.Service{
		Store: database,
		Client: &cacheapi.Client{
			BaseURL: cfg.CacheURL, Token: cfg.CacheToken, UserAgent: "CineVault/" + version,
		},
		Log: log,
	}
	initCtx, cancelInit := context.WithTimeout(context.Background(), 5*time.Second)
	err = cache.Init(initCtx)
	cancelInit()
	if err != nil {
		return fmt.Errorf("cache settings: %w", err)
	}
	switch {
	case !cache.Enabled():
		log.Info("cinevault cache", "url", cfg.CacheURL, "enabled", false, "note", "turned off in Settings")
	case cache.Subscribed():
		log.Info("cinevault cache", "url", cfg.CacheURL, "subscribed", true)
	default:
		log.Info("cinevault cache", "url", cfg.CacheURL, "subscribed", false, "note", "no token in Settings or CINEVAULT_CACHE_TOKEN; titles are only shared with the cache, if sharing is on")
	}

	var matchFn func(context.Context, int64) (match.Result, error)
	if cfg.TMDbToken != "" {
		matcher := match.Matcher{Store: database, Source: tmdb.New(cfg.TMDbToken)}
		matchFn = func(ctx context.Context, libraryID int64) (match.Result, error) {
			res, err := matcher.MatchLibrary(ctx, libraryID)
			if err == nil {
				// Look the new titles up in the cache without holding up the scan.
				go func() {
					c, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
					defer cancel()
					cache.LookupNew(c)
				}()
			}
			return res, err
		}
		log.Info("metadata", "source", "TMDb")
	} else {
		log.Warn("metadata: CINEVAULT_TMDB_TOKEN not set; titles won't be matched to TMDb")
	}

	// Music videos are matched to MusicBrainz (no key needed), which gives
	// the CineVault Cache its key for them (cache decision 11).
	mbClient := &musicbrainz.Client{UserAgent: "CineVault/" + version + " ( https://github.com/JustinTDCT/CineVault )"}
	mvMatcher := match.MusicVideoMatcher{Store: database, Source: mbClient}
	lookupAfter := func(res match.Result, err error) (match.Result, error) {
		if err == nil && res.Matched > 0 {
			go func() {
				c, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
				defer cancel()
				cache.LookupNew(c)
			}()
		}
		return res, err
	}
	matchMusicVideos := func(ctx context.Context, libraryID int64) (match.Result, error) {
		return lookupAfter(mvMatcher.MatchLibrary(ctx, libraryID))
	}
	rematchMusicVideos := func(ctx context.Context, libraryID int64) (match.Result, error) {
		return lookupAfter(mvMatcher.RematchLibrary(ctx, libraryID))
	}

	// Albums (MusicBrainz, sharing the music video client's limiter), books
	// and audiobooks (Open Library), comic series (Comic Vine, with a key).
	agent := "CineVault/" + version + " ( https://github.com/JustinTDCT/CineVault )"
	lookups := map[string]server.Lookup{}
	addLookup := func(mediaType string, f match.Finder) {
		m := match.LookupMatcher{Store: database, Finder: f}
		lookups[mediaType] = server.Lookup{
			Match: func(ctx context.Context, id int64) (match.Result, error) {
				return lookupAfter(m.MatchLibrary(ctx, id))
			},
			Rematch: func(ctx context.Context, id int64) (match.Result, error) {
				return lookupAfter(m.RematchLibrary(ctx, id))
			},
		}
	}
	addLookup("music", match.AlbumFinder{MB: mbClient})
	books := match.BookFinder{OL: &openlibrary.Client{UserAgent: agent}}
	addLookup("book", books)
	addLookup("audiobook", books)
	if cfg.ComicVineKey != "" {
		addLookup("comic", match.ComicFinder{CV: &comicvine.Client{APIKey: cfg.ComicVineKey, UserAgent: agent}})
		log.Info("metadata", "source", "Comic Vine")
	} else {
		log.Info("metadata: CINEVAULT_COMICVINE_KEY not set; comic series aren't matched to Comic Vine")
	}

	api := server.New(server.Options{
		Version:            version,
		DBAddr:             dbAddr,
		Caps:               caps,
		SchemaVersion:      mig.Current,
		PingDB:             database.Ping,
		EditionTypes:       database.EditionTypes,
		Libraries:          database,
		Scan:               scanner.Scan,
		Match:              matchFn,
		MatchMusicVideos:   matchMusicVideos,
		RematchMusicVideos: rematchMusicVideos,
		Lookups:            lookups,
		UI:                 webui.Handler(),
		Playback: &server.Playback{
			Versions: database, Streamer: streamer,
			Encoder: enc, EncoderOK: encOK, Note: note, MaxHeight: 1080,
			Thumbs: thumb.New(cfg.FFmpegPath, filepath.Join(cfg.CacheDir, "thumbs")),
			Comics: comics,
		},
		Cache:         cache,
		TitleCacheKey: database.TitleCacheKey,
		Log:           log,
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
	go cache.Run(ctx)
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
