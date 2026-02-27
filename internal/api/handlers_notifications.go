package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Notification Channel Handlers ────────────────────

// GET /api/v1/notifications/channels
func (s *Server) handleListNotificationChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.notificationRepo.ListChannels()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: channels})
}

// POST /api/v1/notifications/channels
func (s *Server) handleCreateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	var ch models.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if ch.Name == "" || ch.ChannelType == "" || ch.WebhookURL == "" {
		s.respondError(w, http.StatusBadRequest, "name, channel_type, and webhook_url are required")
		return
	}
	if ch.Events == "" {
		ch.Events = `["all"]`
	}
	if err := s.notificationRepo.CreateChannel(&ch); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create channel")
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: ch})
}

// PUT /api/v1/notifications/channels/{id}
func (s *Server) handleUpdateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	var ch models.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ch.ID = id
	if err := s.notificationRepo.UpdateChannel(&ch); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update channel")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: ch})
}

// DELETE /api/v1/notifications/channels/{id}
func (s *Server) handleDeleteNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	if err := s.notificationRepo.DeleteChannel(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to delete channel")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// POST /api/v1/notifications/channels/{id}/test
func (s *Server) handleTestNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	ch, err := s.notificationRepo.GetChannel(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "channel not found")
		return
	}
	if err := s.webhookSender.SendTest(ch); err != nil {
		s.respondError(w, http.StatusBadGateway, "test failed: "+err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"status": "sent"}})
}

// ──────────────────── Alert Rule Handlers ────────────────────

// GET /api/v1/notifications/alerts
func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.notificationRepo.ListAlertRules()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list alert rules")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: rules})
}

// POST /api/v1/notifications/alerts
func (s *Server) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) {
	var rule models.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if rule.Name == "" || rule.ConditionType == "" || rule.ChannelID == uuid.Nil {
		s.respondError(w, http.StatusBadRequest, "name, condition_type, and channel_id are required")
		return
	}
	if rule.CooldownMinutes == 0 {
		rule.CooldownMinutes = 60
	}
	if err := s.notificationRepo.CreateAlertRule(&rule); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create alert rule")
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: rule})
}

// PUT /api/v1/notifications/alerts/{id}
func (s *Server) handleUpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid alert rule id")
		return
	}
	var rule models.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rule.ID = id
	if err := s.notificationRepo.UpdateAlertRule(&rule); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update alert rule")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: rule})
}

// DELETE /api/v1/notifications/alerts/{id}
func (s *Server) handleDeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid alert rule id")
		return
	}
	if err := s.notificationRepo.DeleteAlertRule(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to delete alert rule")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// GET /api/v1/notifications/log
func (s *Server) handleGetAlertLog(w http.ResponseWriter, r *http.Request) {
	entries, err := s.notificationRepo.GetRecentAlerts(100)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get alert log")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: entries})
}
