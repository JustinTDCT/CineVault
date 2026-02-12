package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ══════════════════════ Scene Markers (P15-02) ══════════════════════

// GET /api/v1/media/{id}/markers
func (s *Server) handleGetMarkers(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("id")
	rows, err := s.db.Query(`SELECT sm.id, sm.title, sm.tag_id, t.name, sm.start_seconds, sm.end_seconds, sm.preview_path
		FROM scene_markers sm LEFT JOIN tags t ON sm.tag_id = t.id
		WHERE sm.media_item_id = $1 ORDER BY sm.start_seconds`, mediaID)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: []interface{}{}})
		return
	}
	defer rows.Close()
	var markers []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var title string
		var tagID *uuid.UUID
		var tagName *string
		var startSec float64
		var endSec *float64
		var previewPath *string
		if rows.Scan(&id, &title, &tagID, &tagName, &startSec, &endSec, &previewPath) == nil {
			m := map[string]interface{}{
				"id": id, "title": title, "start_seconds": startSec,
			}
			if tagID != nil {
				m["tag_id"] = tagID
			}
			if tagName != nil {
				m["tag_name"] = tagName
			}
			if endSec != nil {
				m["end_seconds"] = endSec
			}
			if previewPath != nil {
				m["preview_path"] = previewPath
			}
			markers = append(markers, m)
		}
	}
	if markers == nil {
		markers = []map[string]interface{}{}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: markers})
}

// POST /api/v1/media/{id}/markers — Create marker(s)
func (s *Server) handleCreateMarker(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("id")
	var req struct {
		Markers []struct {
			Title    string   `json:"title"`
			TagID    *string  `json:"tag_id"`
			Start    float64  `json:"start_seconds"`
			End      *float64 `json:"end_seconds"`
			Preview  *string  `json:"preview_path"`
		} `json:"markers"`
		// Single marker mode
		Title   string   `json:"title"`
		TagID   *string  `json:"tag_id"`
		Start   float64  `json:"start_seconds"`
		End     *float64 `json:"end_seconds"`
		Preview *string  `json:"preview_path"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}

	// Normalize to list
	items := req.Markers
	if len(items) == 0 && req.Title != "" {
		items = append(items, struct {
			Title   string   `json:"title"`
			TagID   *string  `json:"tag_id"`
			Start   float64  `json:"start_seconds"`
			End     *float64 `json:"end_seconds"`
			Preview *string  `json:"preview_path"`
		}{req.Title, req.TagID, req.Start, req.End, req.Preview})
	}

	created := 0
	for _, m := range items {
		_, err := s.db.Exec(`INSERT INTO scene_markers (media_item_id, title, tag_id, start_seconds, end_seconds, preview_path)
			VALUES ($1, $2, $3, $4, $5, $6)`, mediaID, m.Title, m.TagID, m.Start, m.End, m.Preview)
		if err == nil {
			created++
		}
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{"created": created}})
}

// DELETE /api/v1/markers/{id}
func (s *Server) handleDeleteMarker(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.db.Exec("DELETE FROM scene_markers WHERE id = $1", id)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ══════════════════════ Extended Performer Metadata (P15-03) ══════════════════════

// GET /api/v1/performers/{id}/extended
func (s *Server) handleGetPerformerExtended(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var gender, birthPlace, ethnicity, hairColor, eyeColor, measurements sql.NullString
	var heightCM, weightKG sql.NullInt64
	var aliases []string
	var urlsJSON []byte

	err := s.db.QueryRow(`SELECT gender, birth_place, height_cm, weight_kg, ethnicity, hair_color, eye_color,
		measurements, aliases, urls FROM performers WHERE id = $1`, id).
		Scan(&gender, &birthPlace, &heightCM, &weightKG, &ethnicity, &hairColor, &eyeColor,
			&measurements, pq.Array(&aliases), &urlsJSON)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "performer not found")
		return
	}

	data := map[string]interface{}{}
	if gender.Valid {
		data["gender"] = gender.String
	}
	if birthPlace.Valid {
		data["birth_place"] = birthPlace.String
	}
	if heightCM.Valid {
		data["height_cm"] = heightCM.Int64
	}
	if weightKG.Valid {
		data["weight_kg"] = weightKG.Int64
	}
	if ethnicity.Valid {
		data["ethnicity"] = ethnicity.String
	}
	if hairColor.Valid {
		data["hair_color"] = hairColor.String
	}
	if eyeColor.Valid {
		data["eye_color"] = eyeColor.String
	}
	if measurements.Valid {
		data["measurements"] = measurements.String
	}
	if aliases != nil {
		data["aliases"] = aliases
	}
	if urlsJSON != nil {
		var urls interface{}
		json.Unmarshal(urlsJSON, &urls)
		data["urls"] = urls
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: data})
}

// PUT /api/v1/performers/{id}/extended
func (s *Server) handleUpdatePerformerExtended(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Gender       *string  `json:"gender"`
		BirthPlace   *string  `json:"birth_place"`
		HeightCM     *int     `json:"height_cm"`
		WeightKG     *int     `json:"weight_kg"`
		Ethnicity    *string  `json:"ethnicity"`
		HairColor    *string  `json:"hair_color"`
		EyeColor     *string  `json:"eye_color"`
		Measurements *string  `json:"measurements"`
		Aliases      []string `json:"aliases"`
		URLs         *json.RawMessage `json:"urls"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if req.Gender != nil {
		s.db.Exec("UPDATE performers SET gender = $1, updated_at = NOW() WHERE id = $2", *req.Gender, id)
	}
	if req.BirthPlace != nil {
		s.db.Exec("UPDATE performers SET birth_place = $1, updated_at = NOW() WHERE id = $2", *req.BirthPlace, id)
	}
	if req.HeightCM != nil {
		s.db.Exec("UPDATE performers SET height_cm = $1, updated_at = NOW() WHERE id = $2", *req.HeightCM, id)
	}
	if req.WeightKG != nil {
		s.db.Exec("UPDATE performers SET weight_kg = $1, updated_at = NOW() WHERE id = $2", *req.WeightKG, id)
	}
	if req.Ethnicity != nil {
		s.db.Exec("UPDATE performers SET ethnicity = $1, updated_at = NOW() WHERE id = $2", *req.Ethnicity, id)
	}
	if req.HairColor != nil {
		s.db.Exec("UPDATE performers SET hair_color = $1, updated_at = NOW() WHERE id = $2", *req.HairColor, id)
	}
	if req.EyeColor != nil {
		s.db.Exec("UPDATE performers SET eye_color = $1, updated_at = NOW() WHERE id = $2", *req.EyeColor, id)
	}
	if req.Measurements != nil {
		s.db.Exec("UPDATE performers SET measurements = $1, updated_at = NOW() WHERE id = $2", *req.Measurements, id)
	}
	if req.Aliases != nil {
		s.db.Exec("UPDATE performers SET aliases = $1, updated_at = NOW() WHERE id = $2", pq.Array(req.Aliases), id)
	}
	if req.URLs != nil {
		s.db.Exec("UPDATE performers SET urls = $1, updated_at = NOW() WHERE id = $2", []byte(*req.URLs), id)
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ══════════════════════ Per-user Streaming Limits (P15-04) ══════════════════════

// GET /api/v1/admin/users/{id}/stream-limits
func (s *Server) handleGetStreamLimits(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var maxStreams, maxBitrate int
	var remoteCap *string
	err := s.db.QueryRow("SELECT max_simultaneous_streams, max_bitrate_kbps, remote_quality_cap FROM users WHERE id = $1", id).
		Scan(&maxStreams, &maxBitrate, &remoteCap)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "user not found")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"max_simultaneous_streams": maxStreams, "max_bitrate_kbps": maxBitrate, "remote_quality_cap": remoteCap,
	}})
}

// PUT /api/v1/admin/users/{id}/stream-limits
func (s *Server) handleUpdateStreamLimits(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		MaxStreams      *int    `json:"max_simultaneous_streams"`
		MaxBitrate      *int    `json:"max_bitrate_kbps"`
		RemoteQualityCap *string `json:"remote_quality_cap"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.MaxStreams != nil {
		s.db.Exec("UPDATE users SET max_simultaneous_streams = $1 WHERE id = $2", *req.MaxStreams, id)
	}
	if req.MaxBitrate != nil {
		s.db.Exec("UPDATE users SET max_bitrate_kbps = $1 WHERE id = $2", *req.MaxBitrate, id)
	}
	if req.RemoteQualityCap != nil {
		s.db.Exec("UPDATE users SET remote_quality_cap = $1 WHERE id = $2", *req.RemoteQualityCap, id)
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ══════════════════════ Live TV / DVR (P15-05) ══════════════════════

// GET /api/v1/livetv/tuners
func (s *Server) handleListTuners(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, name, device_type, url, is_active, channel_count FROM tuner_devices ORDER BY name")
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: []interface{}{}})
		return
	}
	defer rows.Close()
	var tuners []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, dtype, url string
		var active bool
		var channels int
		if rows.Scan(&id, &name, &dtype, &url, &active, &channels) == nil {
			tuners = append(tuners, map[string]interface{}{
				"id": id, "name": name, "device_type": dtype, "url": url, "is_active": active, "channel_count": channels,
			})
		}
	}
	if tuners == nil {
		tuners = []map[string]interface{}{}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: tuners})
}

// POST /api/v1/livetv/tuners
func (s *Server) handleCreateTuner(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		DeviceType string `json:"device_type"`
		URL        string `json:"url"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.URL == "" {
		s.respondError(w, http.StatusBadRequest, "name and url required")
		return
	}
	if req.DeviceType == "" {
		req.DeviceType = "hdhomerun"
	}
	id := uuid.New()
	s.db.Exec("INSERT INTO tuner_devices (id, name, device_type, url) VALUES ($1, $2, $3, $4)", id, req.Name, req.DeviceType, req.URL)
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{"id": id}})
}

// DELETE /api/v1/livetv/tuners/{id}
func (s *Server) handleDeleteTuner(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.db.Exec("DELETE FROM tuner_devices WHERE id = $1", id)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// GET /api/v1/livetv/epg — Get EPG guide (today or specified date)
func (s *Server) handleGetEPG(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	rows, err := s.db.Query(`SELECT c.id, c.channel_number, c.name, c.icon_url, c.is_favorite,
		p.id, p.title, p.description, p.start_time, p.end_time, p.category
		FROM epg_channels c LEFT JOIN epg_programs p ON c.id = p.channel_id
			AND p.start_time >= $1::date AND p.start_time < ($1::date + interval '1 day')
		ORDER BY c.channel_number, p.start_time`, date)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: []interface{}{}})
		return
	}
	defer rows.Close()

	channelMap := make(map[string]map[string]interface{})
	var order []string
	for rows.Next() {
		var cID uuid.UUID
		var chNum, cName string
		var cIcon *string
		var isFav bool
		var pID *uuid.UUID
		var pTitle, pDesc, pCat *string
		var pStart, pEnd *time.Time
		if rows.Scan(&cID, &chNum, &cName, &cIcon, &isFav, &pID, &pTitle, &pDesc, &pStart, &pEnd, &pCat) != nil {
			continue
		}
		key := cID.String()
		if _, ok := channelMap[key]; !ok {
			channelMap[key] = map[string]interface{}{
				"id": cID, "channel_number": chNum, "name": cName, "icon_url": cIcon,
				"is_favorite": isFav, "programs": []map[string]interface{}{},
			}
			order = append(order, key)
		}
		if pID != nil {
			prog := map[string]interface{}{
				"id": pID, "title": pTitle, "start_time": pStart, "end_time": pEnd,
			}
			if pDesc != nil {
				prog["description"] = *pDesc
			}
			if pCat != nil {
				prog["category"] = *pCat
			}
			progs := channelMap[key]["programs"].([]map[string]interface{})
			channelMap[key]["programs"] = append(progs, prog)
		}
	}

	var result []map[string]interface{}
	for _, k := range order {
		result = append(result, channelMap[k])
	}
	if result == nil {
		result = []map[string]interface{}{}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: result})
}

// POST /api/v1/livetv/recordings — Schedule a DVR recording
func (s *Server) handleScheduleRecording(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		ChannelID string `json:"channel_id"`
		ProgramID string `json:"program_id"`
		Title     string `json:"title"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ChannelID == "" || req.Title == "" {
		s.respondError(w, http.StatusBadRequest, "channel_id and title required")
		return
	}
	start, _ := time.Parse(time.RFC3339, req.StartTime)
	end, _ := time.Parse(time.RFC3339, req.EndTime)
	if start.IsZero() || end.IsZero() {
		s.respondError(w, http.StatusBadRequest, "start_time and end_time required (RFC3339)")
		return
	}
	id := uuid.New()
	s.db.Exec(`INSERT INTO dvr_recordings (id, channel_id, program_id, user_id, title, start_time, end_time)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7)`, id, req.ChannelID, req.ProgramID, userID, req.Title, start, end)
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{"id": id}})
}

// GET /api/v1/livetv/recordings
func (s *Server) handleListRecordings(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, channel_id, title, file_path, state, start_time, end_time FROM dvr_recordings ORDER BY start_time DESC LIMIT 100`)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: []interface{}{}})
		return
	}
	defer rows.Close()
	var recs []map[string]interface{}
	for rows.Next() {
		var id, chID uuid.UUID
		var title string
		var filePath *string
		var state string
		var start, end time.Time
		if rows.Scan(&id, &chID, &title, &filePath, &state, &start, &end) == nil {
			recs = append(recs, map[string]interface{}{
				"id": id, "channel_id": chID, "title": title, "file_path": filePath,
				"state": state, "start_time": start, "end_time": end,
			})
		}
	}
	if recs == nil {
		recs = []map[string]interface{}{}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: recs})
}

// ══════════════════════ Comics & eBooks Reading Progress (P15-06) ══════════════════════

// GET /api/v1/media/{id}/reading-progress
func (s *Server) handleGetReadingProgress(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("id")
	userID := s.getUserID(r)
	var page, total, fontSize int
	var chapter *string
	err := s.db.QueryRow("SELECT current_page, total_pages, current_chapter, font_size FROM reading_progress WHERE user_id = $1 AND media_item_id = $2",
		userID, mediaID).Scan(&page, &total, &chapter, &fontSize)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
			"current_page": 0, "total_pages": 0, "font_size": 16,
		}})
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"current_page": page, "total_pages": total, "current_chapter": chapter, "font_size": fontSize,
	}})
}

// PUT /api/v1/media/{id}/reading-progress
func (s *Server) handleUpdateReadingProgress(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("id")
	userID := s.getUserID(r)
	var req struct {
		CurrentPage    int     `json:"current_page"`
		TotalPages     int     `json:"total_pages"`
		CurrentChapter *string `json:"current_chapter"`
		FontSize       *int    `json:"font_size"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	fs := 16
	if req.FontSize != nil {
		fs = *req.FontSize
	}
	s.db.Exec(`INSERT INTO reading_progress (user_id, media_item_id, current_page, total_pages, current_chapter, font_size, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (user_id, media_item_id) DO UPDATE SET
			current_page = EXCLUDED.current_page, total_pages = EXCLUDED.total_pages,
			current_chapter = EXCLUDED.current_chapter, font_size = EXCLUDED.font_size, updated_at = NOW()`,
		userID, mediaID, req.CurrentPage, req.TotalPages, req.CurrentChapter, fs)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ══════════════════════ Anime Support (P15-01) ══════════════════════

// GET /api/v1/media/{id}/anime-info — Get anime-specific metadata
func (s *Server) handleGetAnimeInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var anilistID, anidbID *int
	var absEpisode *int
	var title string

	// Check external_ids JSONB for anime IDs
	var extIDs []byte
	err := s.db.QueryRow("SELECT title, COALESCE(external_ids, '{}') FROM media_items WHERE id = $1", id).Scan(&title, &extIDs)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "not found")
		return
	}

	var ids map[string]interface{}
	json.Unmarshal(extIDs, &ids)
	if v, ok := ids["anilist_id"]; ok {
		if n, ok := v.(float64); ok {
			i := int(n)
			anilistID = &i
		}
	}
	if v, ok := ids["anidb_id"]; ok {
		if n, ok := v.(float64); ok {
			i := int(n)
			anidbID = &i
		}
	}
	if v, ok := ids["absolute_episode"]; ok {
		if n, ok := v.(float64); ok {
			i := int(n)
			absEpisode = &i
		}
	}

	data := map[string]interface{}{"title": title}
	if anilistID != nil {
		data["anilist_id"] = *anilistID
	}
	if anidbID != nil {
		data["anidb_id"] = *anidbID
	}
	if absEpisode != nil {
		data["absolute_episode"] = *absEpisode
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: data})
}

// PUT /api/v1/media/{id}/anime-info — Set anime-specific IDs
func (s *Server) handleUpdateAnimeInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		AnilistID       *int `json:"anilist_id"`
		AnidbID         *int `json:"anidb_id"`
		AbsoluteEpisode *int `json:"absolute_episode"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}

	// Merge into external_ids JSONB
	var extIDs []byte
	s.db.QueryRow("SELECT COALESCE(external_ids, '{}') FROM media_items WHERE id = $1", id).Scan(&extIDs)
	var ids map[string]interface{}
	json.Unmarshal(extIDs, &ids)
	if ids == nil {
		ids = map[string]interface{}{}
	}
	if req.AnilistID != nil {
		ids["anilist_id"] = *req.AnilistID
	}
	if req.AnidbID != nil {
		ids["anidb_id"] = *req.AnidbID
	}
	if req.AbsoluteEpisode != nil {
		ids["absolute_episode"] = *req.AbsoluteEpisode
	}
	newIDs, _ := json.Marshal(ids)
	s.db.Exec("UPDATE media_items SET external_ids = $1, updated_at = NOW() WHERE id = $2", newIDs, id)

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// Suppress unused imports
var _ = strconv.Itoa
var _ = fmt.Sprint
