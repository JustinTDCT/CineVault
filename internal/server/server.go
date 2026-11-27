// Package server is CineVault's HTTP API.
package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/JustinTDCT/CineVault/internal/probe"
)

// Capabilities is what the host can do for transcoding, probed at startup.
type Capabilities struct {
	FFmpeg probe.FFmpegInfo `json:"ffmpeg"`
	GPUs   []probe.GPU      `json:"gpus"`
}

// Server serves the CineVault API.
type Server struct {
	version string
	dbAddr  string
	caps    Capabilities
	// dial checks the database is reachable; replaceable in tests.
	dial func(ctx context.Context, addr string) error
}

// New creates a Server.
func New(version, dbAddr string, caps Capabilities) *Server {
	return &Server{version: version, dbAddr: dbAddr, caps: caps, dial: tcpDial}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	return mux
}

type dbStatus struct {
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

type healthResponse struct {
	Status    string       `json:"status"`
	Version   string       `json:"version"`
	Time      time.Time    `json:"time"`
	Database  dbStatus     `json:"database"`
	Transcode Capabilities `json:"transcode"`
}

// health reports "ok" when the database is reachable and FFmpeg was found,
// and "degraded" otherwise. It never exposes credentials.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	resp := healthResponse{
		Status:    "ok",
		Version:   s.version,
		Time:      time.Now().UTC(),
		Transcode: s.caps,
	}
	if err := s.dial(ctx, s.dbAddr); err != nil {
		resp.Database = dbStatus{Error: "cannot connect to " + s.dbAddr}
		resp.Status = "degraded"
	} else {
		resp.Database = dbStatus{Reachable: true}
	}
	if !s.caps.FFmpeg.Found {
		resp.Status = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	if resp.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}

func tcpDial(ctx context.Context, addr string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return conn.Close()
}
