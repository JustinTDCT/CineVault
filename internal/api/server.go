package api

import (
	"encoding/json"
	"net/http"
	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/db"
)

type Server struct {
	config *config.Config
	db     *db.DB
	router *http.ServeMux
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func NewServer(cfg *config.Config, database *db.DB) *Server {
	s := &Server{config: cfg, db: database, router: http.NewServeMux()}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("GET /health", s.handleHealth)
	s.router.HandleFunc("GET /api/v1/status", s.handleStatus)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"status": "ok"}})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"version": "0.1.0", "phase": "1"}})
}

func (s *Server) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) Start() error {
	return http.ListenAndServe(s.config.Server.Address(), s.router)
}
