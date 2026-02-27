package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// ──────────────────── Analytics Handlers ────────────────────

// GET /api/v1/analytics/overview
func (s *Server) handleAnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := s.analyticsRepo.GetAnalyticsOverview()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get analytics overview")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: overview})
}

// GET /api/v1/analytics/streams
func (s *Server) handleAnalyticsStreams(w http.ResponseWriter, r *http.Request) {
	active, err := s.analyticsRepo.GetActiveStreams()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get active streams")
		return
	}
	recent, err := s.analyticsRepo.GetRecentStreams(50)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get recent streams")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"active": active,
		"recent": recent,
	}})
}

// GET /api/v1/analytics/streams/breakdown
func (s *Server) handleAnalyticsStreamBreakdown(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	breakdown, err := s.analyticsRepo.GetStreamTypeBreakdown(days)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get stream breakdown")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: breakdown})
}

// GET /api/v1/analytics/watch-activity
func (s *Server) handleAnalyticsWatchActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var userID *uuid.UUID
	if uid := q.Get("user_id"); uid != "" {
		if parsed, err := uuid.Parse(uid); err == nil {
			userID = &parsed
		}
	}
	var from, to *time.Time
	if f := q.Get("from"); f != "" {
		if t, err := time.Parse("2006-01-02", f); err == nil {
			from = &t
		}
	}
	if t := q.Get("to"); t != "" {
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			end := parsed.Add(24*time.Hour - time.Second)
			to = &end
		}
	}
	mediaType := q.Get("media_type")
	limit := 100
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}

	activity, err := s.analyticsRepo.GetWatchActivity(userID, from, to, mediaType, limit)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get watch activity")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: activity})
}

// GET /api/v1/analytics/users/activity
func (s *Server) handleAnalyticsUserActivity(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.analyticsRepo.GetUserActivitySummaries()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get user activity")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: summaries})
}

// GET /api/v1/analytics/transcodes
func (s *Server) handleAnalyticsTranscodes(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	stats, err := s.analyticsRepo.GetTranscodeStats(days)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get transcode stats")
		return
	}
	recent, err := s.analyticsRepo.GetRecentTranscodes(30)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get recent transcodes")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"stats":  stats,
		"recent": recent,
	}})
}

// GET /api/v1/analytics/system
func (s *Server) handleAnalyticsSystem(w http.ResponseWriter, r *http.Request) {
	latest, err := s.analyticsRepo.GetLatestMetrics()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get system metrics")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: latest})
}

// GET /api/v1/analytics/system/history
func (s *Server) handleAnalyticsSystemHistory(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 && v <= 168 {
			hours = v
		}
	}
	history, err := s.analyticsRepo.GetMetricsHistory(hours)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get system history")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: history})
}

// GET /api/v1/analytics/storage
func (s *Server) handleAnalyticsStorage(w http.ResponseWriter, r *http.Request) {
	storage, err := s.analyticsRepo.GetStorageStats()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get storage stats")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: storage})
}

// GET /api/v1/analytics/library-health
func (s *Server) handleAnalyticsLibraryHealth(w http.ResponseWriter, r *http.Request) {
	health, err := s.analyticsRepo.GetLibraryHealth()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get library health")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: health})
}

// GET /api/v1/analytics/trends
func (s *Server) handleAnalyticsTrends(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from := time.Now().AddDate(0, 0, -30)
	to := time.Now()
	if f := q.Get("from"); f != "" {
		if t, err := time.Parse("2006-01-02", f); err == nil {
			from = t
		}
	}
	if t := q.Get("to"); t != "" {
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			to = parsed
		}
	}
	trends, err := s.analyticsRepo.GetDailyTrends(from, to)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get trends")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: trends})
}
