package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ──────────────────── Watchlist ────────────────────

func (s *Server) handleGetWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	rows, err := s.db.Query(`
		SELECT w.id, w.media_item_id, w.tv_show_id, w.edition_group_id, w.added_at,
			COALESCE(m.title, t.title, e.name, '') AS title,
			COALESCE(m.poster_path, t.poster_path, '') AS poster_path,
			COALESCE(m.year, 0) AS year,
			COALESCE(m.media_type, 'movie') AS media_type
		FROM user_watchlist w
		LEFT JOIN media_items m ON w.media_item_id = m.id
		LEFT JOIN tv_shows t ON w.tv_show_id = t.id
		LEFT JOIN edition_groups e ON w.edition_group_id = e.id
		WHERE w.user_id = $1 ORDER BY w.added_at DESC`, userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var mediaID, showID, editionID *uuid.UUID
		var addedAt time.Time
		var title, posterPath, mediaType string
		var year int
		if err := rows.Scan(&id, &mediaID, &showID, &editionID, &addedAt, &title, &posterPath, &year, &mediaType); err != nil {
			continue
		}
		item := map[string]interface{}{
			"id": id, "added_at": addedAt, "title": title, "poster_path": posterPath, "year": year, "media_type": mediaType,
		}
		if mediaID != nil { item["media_item_id"] = *mediaID }
		if showID != nil { item["tv_show_id"] = *showID }
		if editionID != nil { item["edition_group_id"] = *editionID }
		items = append(items, item)
	}
	if items == nil { items = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: items})
}

func (s *Server) handleAddToWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item id")
		return
	}
	// Determine type by checking which table the ID exists in
	var col string
	var exists bool
	_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM media_items WHERE id=$1)", itemID).Scan(&exists)
	if exists {
		col = "media_item_id"
	} else {
		_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM tv_shows WHERE id=$1)", itemID).Scan(&exists)
		if exists {
			col = "tv_show_id"
		} else {
			_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM edition_groups WHERE id=$1)", itemID).Scan(&exists)
			if exists {
				col = "edition_group_id"
			}
		}
	}
	if col == "" {
		s.respondError(w, http.StatusNotFound, "item not found")
		return
	}
	_, err = s.db.Exec("INSERT INTO user_watchlist (user_id, "+col+") VALUES ($1, $2) ON CONFLICT DO NOTHING", userID, itemID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleRemoveFromWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item id")
		return
	}
	_, _ = s.db.Exec("DELETE FROM user_watchlist WHERE user_id=$1 AND (media_item_id=$2 OR tv_show_id=$2 OR edition_group_id=$2)", userID, itemID)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleCheckWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item id")
		return
	}
	var exists bool
	_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM user_watchlist WHERE user_id=$1 AND (media_item_id=$2 OR tv_show_id=$2 OR edition_group_id=$2))", userID, itemID).Scan(&exists)
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]bool{"in_watchlist": exists}})
}

// ──────────────────── User Ratings ────────────────────

func (s *Server) handleRateMedia(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	var req struct {
		Rating float64 `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Rating < 0 || req.Rating > 10 {
		s.respondError(w, http.StatusBadRequest, "rating must be between 0 and 10")
		return
	}
	_, err = s.db.Exec(`INSERT INTO user_ratings (user_id, media_item_id, rating) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, media_item_id) DO UPDATE SET rating = $3, updated_at = NOW()`, userID, mediaID, req.Rating)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleDeleteRating(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	_, _ = s.db.Exec("DELETE FROM user_ratings WHERE user_id=$1 AND media_item_id=$2", userID, mediaID)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleGetUserRating(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	var rating float64
	err = s.db.QueryRow("SELECT rating FROM user_ratings WHERE user_id=$1 AND media_item_id=$2", userID, mediaID).Scan(&rating)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"rating": nil}})
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"rating": rating}})
}

func (s *Server) handleGetCommunityRating(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	var avg float64
	var count int
	err = s.db.QueryRow("SELECT COALESCE(AVG(rating),0), COUNT(*) FROM user_ratings WHERE media_item_id=$1", mediaID).Scan(&avg, &count)
	if err != nil {
		avg = 0
		count = 0
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"average": avg, "count": count}})
}

// ──────────────────── Favorites ────────────────────

func (s *Server) handleToggleFavorite(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item id")
		return
	}
	// Check which table the ID belongs to
	col := detectItemColumn(s, itemID)
	if col == "" {
		s.respondError(w, http.StatusNotFound, "item not found")
		return
	}
	// Toggle: delete if exists, insert if not
	res, _ := s.db.Exec("DELETE FROM user_favorites WHERE user_id=$1 AND "+col+"=$2", userID, itemID)
	if affected, _ := res.RowsAffected(); affected > 0 {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]bool{"favorited": false}})
		return
	}
	_, err = s.db.Exec("INSERT INTO user_favorites (user_id, "+col+") VALUES ($1, $2)", userID, itemID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]bool{"favorited": true}})
}

func (s *Server) handleCheckFavorite(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item id")
		return
	}
	var exists bool
	_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM user_favorites WHERE user_id=$1 AND (media_item_id=$2 OR tv_show_id=$2 OR performer_id=$2))", userID, itemID).Scan(&exists)
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]bool{"favorited": exists}})
}

func (s *Server) handleGetFavorites(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	rows, err := s.db.Query(`
		SELECT f.id, f.media_item_id, f.tv_show_id, f.performer_id, f.added_at,
			COALESCE(m.title, t.title, p.name, '') AS title,
			COALESCE(m.poster_path, t.poster_path, p.photo_path, '') AS poster_path,
			CASE WHEN m.id IS NOT NULL THEN 'media' WHEN t.id IS NOT NULL THEN 'show' ELSE 'performer' END AS item_type
		FROM user_favorites f
		LEFT JOIN media_items m ON f.media_item_id = m.id
		LEFT JOIN tv_shows t ON f.tv_show_id = t.id
		LEFT JOIN performers p ON f.performer_id = p.id
		WHERE f.user_id = $1 ORDER BY f.added_at DESC`, userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var items []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var mediaID, showID, performerID *uuid.UUID
		var addedAt time.Time
		var title, posterPath, itemType string
		if err := rows.Scan(&id, &mediaID, &showID, &performerID, &addedAt, &title, &posterPath, &itemType); err != nil {
			continue
		}
		item := map[string]interface{}{"id": id, "added_at": addedAt, "title": title, "poster_path": posterPath, "item_type": itemType}
		if mediaID != nil { item["media_item_id"] = *mediaID }
		if showID != nil { item["tv_show_id"] = *showID }
		if performerID != nil { item["performer_id"] = *performerID }
		items = append(items, item)
	}
	if items == nil { items = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: items})
}

// ──────────────────── Playlists ────────────────────

func (s *Server) handleListPlaylists(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.description, p.poster_path, p.is_public, p.shuffle_mode, p.repeat_mode,
			p.created_at, p.updated_at, COUNT(pi.id) AS item_count,
			COALESCE(SUM(COALESCE(m.duration, 0)), 0) AS total_duration
		FROM playlists p
		LEFT JOIN playlist_items pi ON pi.playlist_id = p.id
		LEFT JOIN media_items m ON pi.media_item_id = m.id
		WHERE p.user_id = $1
		GROUP BY p.id ORDER BY p.updated_at DESC`, userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var playlists []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, repeatMode string
		var desc, poster *string
		var isPublic, shuffleMode bool
		var createdAt, updatedAt time.Time
		var itemCount, totalDuration int
		if err := rows.Scan(&id, &name, &desc, &poster, &isPublic, &shuffleMode, &repeatMode, &createdAt, &updatedAt, &itemCount, &totalDuration); err != nil {
			continue
		}
		playlists = append(playlists, map[string]interface{}{
			"id": id, "name": name, "description": desc, "poster_path": poster,
			"is_public": isPublic, "shuffle_mode": shuffleMode, "repeat_mode": repeatMode,
			"created_at": createdAt, "updated_at": updatedAt,
			"item_count": itemCount, "total_duration": totalDuration,
		})
	}
	if playlists == nil { playlists = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: playlists})
}

func (s *Server) handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		s.respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	id := uuid.New()
	var desc *string
	if req.Description != "" { desc = &req.Description }
	_, err := s.db.Exec("INSERT INTO playlists (id, user_id, name, description) VALUES ($1, $2, $3, $4)", id, userID, req.Name, desc)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{"id": id, "name": req.Name}})
}

func (s *Server) handleUpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	plID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		ShuffleMode *bool   `json:"shuffle_mode"`
		RepeatMode  *string `json:"repeat_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name != nil {
		s.db.Exec("UPDATE playlists SET name=$1, updated_at=NOW() WHERE id=$2 AND user_id=$3", *req.Name, plID, userID)
	}
	if req.Description != nil {
		s.db.Exec("UPDATE playlists SET description=$1, updated_at=NOW() WHERE id=$2 AND user_id=$3", *req.Description, plID, userID)
	}
	if req.ShuffleMode != nil {
		s.db.Exec("UPDATE playlists SET shuffle_mode=$1, updated_at=NOW() WHERE id=$2 AND user_id=$3", *req.ShuffleMode, plID, userID)
	}
	if req.RepeatMode != nil {
		s.db.Exec("UPDATE playlists SET repeat_mode=$1, updated_at=NOW() WHERE id=$2 AND user_id=$3", *req.RepeatMode, plID, userID)
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleDeletePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	plID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	_, _ = s.db.Exec("DELETE FROM playlists WHERE id=$1 AND user_id=$2", plID, userID)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleGetPlaylistItems(w http.ResponseWriter, r *http.Request) {
	plID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	rows, err := s.db.Query(`
		SELECT pi.id, pi.media_item_id, pi.sort_order, pi.added_at,
			m.title, m.poster_path, m.duration, m.media_type, m.year
		FROM playlist_items pi
		JOIN media_items m ON pi.media_item_id = m.id
		WHERE pi.playlist_id = $1 ORDER BY pi.sort_order`, plID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var items []map[string]interface{}
	for rows.Next() {
		var id, mediaID uuid.UUID
		var sortOrder int
		var addedAt time.Time
		var title string
		var poster *string
		var duration *int
		var mediaType string
		var year *int
		if err := rows.Scan(&id, &mediaID, &sortOrder, &addedAt, &title, &poster, &duration, &mediaType, &year); err != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"id": id, "media_item_id": mediaID, "sort_order": sortOrder, "added_at": addedAt,
			"title": title, "poster_path": poster, "duration": duration, "media_type": mediaType, "year": year,
		})
	}
	if items == nil { items = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: items})
}

func (s *Server) handleAddPlaylistItem(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	plID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	var req struct {
		MediaItemID string `json:"media_item_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	mediaID, err := uuid.Parse(req.MediaItemID)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media_item_id")
		return
	}
	// Verify ownership
	var exists bool
	_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM playlists WHERE id=$1 AND user_id=$2)", plID, userID).Scan(&exists)
	if !exists {
		s.respondError(w, http.StatusForbidden, "not your playlist")
		return
	}
	// Get next sort order
	var maxOrder int
	_ = s.db.QueryRow("SELECT COALESCE(MAX(sort_order),0) FROM playlist_items WHERE playlist_id=$1", plID).Scan(&maxOrder)
	id := uuid.New()
	_, err = s.db.Exec("INSERT INTO playlist_items (id, playlist_id, media_item_id, sort_order) VALUES ($1, $2, $3, $4)", id, plID, mediaID, maxOrder+1)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.db.Exec("UPDATE playlists SET updated_at=NOW() WHERE id=$1", plID)
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{"id": id}})
}

func (s *Server) handleRemovePlaylistItem(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	plID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item id")
		return
	}
	var exists bool
	_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM playlists WHERE id=$1 AND user_id=$2)", plID, userID).Scan(&exists)
	if !exists {
		s.respondError(w, http.StatusForbidden, "not your playlist")
		return
	}
	_, _ = s.db.Exec("DELETE FROM playlist_items WHERE id=$1 AND playlist_id=$2", itemID, plID)
	s.db.Exec("UPDATE playlists SET updated_at=NOW() WHERE id=$1", plID)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleReorderPlaylistItems(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	plID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	var exists bool
	_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM playlists WHERE id=$1 AND user_id=$2)", plID, userID).Scan(&exists)
	if !exists {
		s.respondError(w, http.StatusForbidden, "not your playlist")
		return
	}
	var req struct {
		ItemIDs []string `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	for i, idStr := range req.ItemIDs {
		id, err := uuid.Parse(idStr)
		if err != nil { continue }
		s.db.Exec("UPDATE playlist_items SET sort_order=$1 WHERE id=$2 AND playlist_id=$3", i, id, plID)
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ──────────────────── On Deck ────────────────────

func (s *Server) handleOnDeck(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	rows, err := s.db.Query(`
		WITH last_watched AS (
			SELECT DISTINCT ON (ts.id) ts.id AS show_id, ts.title AS show_title, ts.poster_path AS show_poster,
				wh.media_item_id, wh.last_played_at, m.episode_number, m.tv_season_id
			FROM watch_history wh
			JOIN media_items m ON wh.media_item_id = m.id
			JOIN tv_seasons s ON m.tv_season_id = s.id
			JOIN tv_shows ts ON s.tv_show_id = ts.id
			WHERE wh.user_id = $1 AND wh.completed = TRUE
			ORDER BY ts.id, wh.last_played_at DESC
		)
		SELECT lw.show_id, lw.show_title, lw.show_poster,
			next_ep.id AS next_episode_id, next_ep.title AS next_title,
			next_ep.poster_path AS next_poster, next_ep.episode_number,
			ns.season_number, next_ep.duration
		FROM last_watched lw
		JOIN tv_seasons cur_s ON lw.tv_season_id = cur_s.id
		LEFT JOIN LATERAL (
			SELECT m2.id, m2.title, m2.poster_path, m2.episode_number, m2.tv_season_id, m2.duration
			FROM media_items m2
			JOIN tv_seasons s2 ON m2.tv_season_id = s2.id
			WHERE s2.tv_show_id = lw.show_id
			AND (s2.season_number > cur_s.season_number
				OR (s2.season_number = cur_s.season_number AND m2.episode_number > lw.episode_number))
			AND NOT EXISTS (
				SELECT 1 FROM watch_history wh2
				WHERE wh2.user_id = $1 AND wh2.media_item_id = m2.id AND wh2.completed = TRUE
			)
			ORDER BY s2.season_number, m2.episode_number
			LIMIT 1
		) next_ep ON TRUE
		LEFT JOIN tv_seasons ns ON next_ep.tv_season_id = ns.id
		WHERE next_ep.id IS NOT NULL
		ORDER BY lw.last_played_at DESC
		LIMIT 20`, userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var items []map[string]interface{}
	for rows.Next() {
		var showID, nextEpID uuid.UUID
		var showTitle, nextTitle string
		var showPoster, nextPoster *string
		var epNum, seasonNum *int
		var duration *int
		if err := rows.Scan(&showID, &showTitle, &showPoster, &nextEpID, &nextTitle, &nextPoster, &epNum, &seasonNum, &duration); err != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"show_id": showID, "show_title": showTitle, "show_poster": showPoster,
			"episode_id": nextEpID, "episode_title": nextTitle, "episode_poster": nextPoster,
			"episode_number": epNum, "season_number": seasonNum, "duration": duration,
		})
	}
	if items == nil { items = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: items})
}

// ──────────────────── Saved Filter Presets ────────────────────

func (s *Server) handleListSavedFilters(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	libraryID := r.URL.Query().Get("library_id")
	var rows interface{ Next() bool; Scan(...interface{}) error; Close() error }
	var err error
	if libraryID != "" {
		lid, perr := uuid.Parse(libraryID)
		if perr != nil {
			s.respondError(w, http.StatusBadRequest, "invalid library_id")
			return
		}
		rows, err = s.db.Query("SELECT id, name, library_id, filters, created_at FROM user_saved_filters WHERE user_id=$1 AND (library_id=$2 OR library_id IS NULL) ORDER BY name", userID, lid)
	} else {
		rows, err = s.db.Query("SELECT id, name, library_id, filters, created_at FROM user_saved_filters WHERE user_id=$1 ORDER BY name", userID)
	}
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var filters []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name string
		var libID *uuid.UUID
		var filtersJSON string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &libID, &filtersJSON, &createdAt); err != nil {
			continue
		}
		item := map[string]interface{}{"id": id, "name": name, "filters": json.RawMessage(filtersJSON), "created_at": createdAt}
		if libID != nil { item["library_id"] = *libID }
		filters = append(filters, item)
	}
	if filters == nil { filters = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: filters})
}

func (s *Server) handleCreateSavedFilter(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		Name      string           `json:"name"`
		LibraryID *string          `json:"library_id"`
		Filters   json.RawMessage  `json:"filters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		s.respondError(w, http.StatusBadRequest, "name and filters required")
		return
	}
	id := uuid.New()
	var libID *uuid.UUID
	if req.LibraryID != nil {
		if lid, err := uuid.Parse(*req.LibraryID); err == nil { libID = &lid }
	}
	_, err := s.db.Exec("INSERT INTO user_saved_filters (id, user_id, library_id, name, filters) VALUES ($1, $2, $3, $4, $5)",
		id, userID, libID, req.Name, string(req.Filters))
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{"id": id}})
}

func (s *Server) handleDeleteSavedFilter(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	filterID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid filter id")
		return
	}
	_, _ = s.db.Exec("DELETE FROM user_saved_filters WHERE id=$1 AND user_id=$2", filterID, userID)
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ──────────────────── Helpers ────────────────────

func detectItemColumn(s *Server, itemID uuid.UUID) string {
	var exists bool
	_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM media_items WHERE id=$1)", itemID).Scan(&exists)
	if exists { return "media_item_id" }
	_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM tv_shows WHERE id=$1)", itemID).Scan(&exists)
	if exists { return "tv_show_id" }
	_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM performers WHERE id=$1)", itemID).Scan(&exists)
	if exists { return "performer_id" }
	return ""
}
