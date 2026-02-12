package api

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ══════════════════════ DLNA / UPnP (P14-01) ══════════════════════

// GET /api/v1/dlna/config
func (s *Server) handleDLNAConfig(w http.ResponseWriter, r *http.Request) {
	var enabled bool
	var name string
	var port int
	err := s.db.QueryRow("SELECT enabled, friendly_name, port FROM dlna_config LIMIT 1").Scan(&enabled, &name, &port)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
			"enabled": false, "friendly_name": "CineVault", "port": 1900,
		}})
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"enabled": enabled, "friendly_name": name, "port": port,
	}})
}

// PUT /api/v1/dlna/config
func (s *Server) handleUpdateDLNAConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled      *bool   `json:"enabled"`
		FriendlyName *string `json:"friendly_name"`
		Port         *int    `json:"port"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Enabled != nil {
		s.db.Exec("UPDATE dlna_config SET enabled = $1, updated_at = NOW()", *req.Enabled)
	}
	if req.FriendlyName != nil {
		s.db.Exec("UPDATE dlna_config SET friendly_name = $1, updated_at = NOW()", *req.FriendlyName)
	}
	if req.Port != nil {
		s.db.Exec("UPDATE dlna_config SET port = $1, updated_at = NOW()", *req.Port)
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// GET /dlna/description.xml — UPnP device description
func (s *Server) handleDLNADescription(w http.ResponseWriter, r *http.Request) {
	var name string
	s.db.QueryRow("SELECT friendly_name FROM dlna_config LIMIT 1").Scan(&name)
	if name == "" {
		name = "CineVault"
	}

	type Service struct {
		XMLName     xml.Name `xml:"service"`
		ServiceType string   `xml:"serviceType"`
		ServiceID   string   `xml:"serviceId"`
		SCPDURL     string   `xml:"SCPDURL"`
		ControlURL  string   `xml:"controlURL"`
		EventSubURL string   `xml:"eventSubURL"`
	}
	type Device struct {
		XMLName      xml.Name  `xml:"device"`
		DeviceType   string    `xml:"deviceType"`
		FriendlyName string    `xml:"friendlyName"`
		Manufacturer string    `xml:"manufacturer"`
		ModelName    string    `xml:"modelName"`
		UDN          string    `xml:"UDN"`
		Services     []Service `xml:"serviceList>service"`
	}
	type Root struct {
		XMLName     xml.Name `xml:"root"`
		XMLNS       string   `xml:"xmlns,attr"`
		SpecVersion struct {
			Major int `xml:"major"`
			Minor int `xml:"minor"`
		} `xml:"specVersion"`
		Device Device `xml:"device"`
	}

	desc := Root{
		XMLNS: "urn:schemas-upnp-org:device-1-0",
		Device: Device{
			DeviceType:   "urn:schemas-upnp-org:device:MediaServer:1",
			FriendlyName: name,
			Manufacturer: "CineVault",
			ModelName:    "CineVault Media Server",
			UDN:          "uuid:cinevault-dlna-001",
			Services: []Service{
				{
					ServiceType: "urn:schemas-upnp-org:service:ContentDirectory:1",
					ServiceID:   "urn:upnp-org:serviceId:ContentDirectory",
					SCPDURL:     "/dlna/ContentDirectory.xml",
					ControlURL:  "/dlna/control/ContentDirectory",
					EventSubURL: "/dlna/event/ContentDirectory",
				},
			},
		},
	}
	desc.SpecVersion.Major = 1
	desc.SpecVersion.Minor = 0

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	xml.NewEncoder(w).Encode(desc)
}

// GET /dlna/content/{id} — DLNA content directory browse
func (s *Server) handleDLNAContent(w http.ResponseWriter, r *http.Request) {
	parentID := r.PathValue("id")

	type Item struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Type     string `json:"type"` // container or item
		MimeType string `json:"mime_type,omitempty"`
		URL      string `json:"url,omitempty"`
	}

	var items []Item

	if parentID == "0" || parentID == "" {
		// Root: list libraries as containers
		rows, err := s.db.Query("SELECT id, name FROM libraries WHERE is_active = TRUE ORDER BY sort_order")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id uuid.UUID
				var name string
				if rows.Scan(&id, &name) == nil {
					items = append(items, Item{ID: id.String(), Title: name, Type: "container"})
				}
			}
		}
	} else {
		// List media items in library
		rows, err := s.db.Query(`SELECT id, title, container, COALESCE(file_path, '') FROM media_items
			WHERE library_id = $1 AND parent_media_id IS NULL ORDER BY sort_title LIMIT 500`, parentID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id uuid.UUID
				var title, container, filePath string
				if rows.Scan(&id, &title, &container, &filePath) == nil {
					mime := "video/mp4"
					if container == "mkv" {
						mime = "video/x-matroska"
					} else if container == "avi" {
						mime = "video/x-msvideo"
					}
					items = append(items, Item{
						ID:       id.String(),
						Title:    title,
						Type:     "item",
						MimeType: mime,
						URL:      fmt.Sprintf("/api/v1/stream/%s/direct", id.String()),
					})
				}
			}
		}
	}

	if items == nil {
		items = []Item{}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: items})
}

// ══════════════════════ Chromecast (P14-02) ══════════════════════

// POST /api/v1/cast/session — Create/update cast session
func (s *Server) handleCastSession(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		MediaItemID string  `json:"media_item_id"`
		DeviceName  string  `json:"device_name"`
		DeviceType  string  `json:"device_type"`
		State       string  `json:"state"`
		CurrentTime float64 `json:"current_time"`
		Duration    float64 `json:"duration"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}

	id := uuid.New()
	_, err := s.db.Exec(`INSERT INTO cast_sessions (id, user_id, media_item_id, device_name, device_type, state, current_time_sec, duration_sec)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, media_item_id) DO UPDATE SET
			device_name = EXCLUDED.device_name, state = EXCLUDED.state,
			current_time_sec = EXCLUDED.current_time_sec, duration_sec = EXCLUDED.duration_sec, updated_at = NOW()`,
		id, userID, req.MediaItemID, req.DeviceName, req.DeviceType, req.State, req.CurrentTime, req.Duration)
	if err != nil {
		// Unique constraint on (user_id, media_item_id) may not exist; try UPDATE
		s.db.Exec(`UPDATE cast_sessions SET device_name=$1, state=$2, current_time_sec=$3, duration_sec=$4, updated_at=NOW()
			WHERE user_id=$5 AND media_item_id=$6`, req.DeviceName, req.State, req.CurrentTime, req.Duration, userID, req.MediaItemID)
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"id": id}})
}

// GET /api/v1/cast/sessions — List active cast sessions for user
func (s *Server) handleListCastSessions(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	rows, err := s.db.Query(`SELECT cs.id, cs.media_item_id, mi.title, cs.device_name, cs.device_type, cs.state, cs.current_time_sec, cs.duration_sec, cs.updated_at
		FROM cast_sessions cs JOIN media_items mi ON cs.media_item_id = mi.id
		WHERE cs.user_id = $1 AND cs.state != 'stopped'
		ORDER BY cs.updated_at DESC`, userID)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: []interface{}{}})
		return
	}
	defer rows.Close()

	var sessions []map[string]interface{}
	for rows.Next() {
		var id, mediaID uuid.UUID
		var title, deviceName, deviceType, state string
		var curTime, duration float64
		var updatedAt time.Time
		if rows.Scan(&id, &mediaID, &title, &deviceName, &deviceType, &state, &curTime, &duration, &updatedAt) == nil {
			sessions = append(sessions, map[string]interface{}{
				"id": id, "media_item_id": mediaID, "title": title, "device_name": deviceName,
				"device_type": deviceType, "state": state, "current_time": curTime,
				"duration": duration, "updated_at": updatedAt,
			})
		}
	}
	if sessions == nil {
		sessions = []map[string]interface{}{}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: sessions})
}

// DELETE /api/v1/cast/{id} — End cast session
func (s *Server) handleEndCastSession(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(r.PathValue("id"))
	s.db.Exec("UPDATE cast_sessions SET state = 'stopped', updated_at = NOW() WHERE id = $1", id)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// PUT /api/v1/cast/session/{id}/command — Remote control Chromecast session
func (s *Server) handleCastCommand(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	var req struct {
		Command string  `json:"command"` // play, pause, seek, volume, stop
		Value   float64 `json:"value"`   // seek seconds or volume 0-1
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Command == "" {
		s.respondError(w, http.StatusBadRequest, "command required")
		return
	}

	// Update the session state based on command
	switch req.Command {
	case "play":
		s.db.Exec("UPDATE cast_sessions SET state = 'playing', updated_at = NOW() WHERE id = $1", sessionID)
	case "pause":
		s.db.Exec("UPDATE cast_sessions SET state = 'paused', updated_at = NOW() WHERE id = $1", sessionID)
	case "seek":
		s.db.Exec("UPDATE cast_sessions SET current_time = $2, updated_at = NOW() WHERE id = $1", sessionID, req.Value)
	case "volume":
		// Volume is client-side only, just acknowledge
	case "stop":
		s.db.Exec("UPDATE cast_sessions SET state = 'stopped', updated_at = NOW() WHERE id = $1", sessionID)
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// Suppress unused import
var _ = models.RoleUser
