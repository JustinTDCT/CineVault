package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ──────────────────── Per-User Stats (P9-01) ────────────────────

func (s *Server) handleGetProfileStats(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)

	stats := map[string]interface{}{}

	// Total watch time (seconds)
	var totalWatchSec int
	s.db.QueryRow("SELECT COALESCE(SUM(progress_seconds), 0) FROM watch_history WHERE user_id = $1", userID).Scan(&totalWatchSec)
	stats["total_watch_seconds"] = totalWatchSec
	stats["total_watch_hours"] = totalWatchSec / 3600

	// Items watched (completed)
	var itemsWatched int
	s.db.QueryRow("SELECT COUNT(DISTINCT media_item_id) FROM watch_history WHERE user_id = $1 AND completed = TRUE", userID).Scan(&itemsWatched)
	stats["items_watched"] = itemsWatched

	// Average rating given
	var avgRating float64
	s.db.QueryRow("SELECT COALESCE(AVG(rating), 0) FROM user_ratings WHERE user_id = $1", userID).Scan(&avgRating)
	stats["average_rating"] = avgRating

	// Most-watched genres (top 5)
	genreRows, err := s.db.Query(`
		SELECT t.name, COUNT(*) AS cnt FROM watch_history wh
		JOIN media_item_tags mt ON wh.media_item_id = mt.media_item_id
		JOIN tags t ON mt.tag_id = t.id AND t.category = 'genre'
		WHERE wh.user_id = $1 AND wh.completed = TRUE
		GROUP BY t.name ORDER BY cnt DESC LIMIT 5`, userID)
	if err == nil {
		defer genreRows.Close()
		var genres []map[string]interface{}
		for genreRows.Next() {
			var name string
			var count int
			if genreRows.Scan(&name, &count) == nil {
				genres = append(genres, map[string]interface{}{"name": name, "count": count})
			}
		}
		stats["top_genres"] = genres
	}

	// Most-watched shows (top 5)
	showRows, err := s.db.Query(`
		SELECT ts.title, COUNT(*) AS cnt FROM watch_history wh
		JOIN media_items m ON wh.media_item_id = m.id
		JOIN tv_seasons s ON m.tv_season_id = s.id
		JOIN tv_shows ts ON s.tv_show_id = ts.id
		WHERE wh.user_id = $1 AND wh.completed = TRUE
		GROUP BY ts.title ORDER BY cnt DESC LIMIT 5`, userID)
	if err == nil {
		defer showRows.Close()
		var shows []map[string]interface{}
		for showRows.Next() {
			var title string
			var count int
			if showRows.Scan(&title, &count) == nil {
				shows = append(shows, map[string]interface{}{"title": title, "count": count})
			}
		}
		stats["top_shows"] = shows
	}

	// Watch time heatmap (hour of day)
	heatmapRows, err := s.db.Query(`
		SELECT EXTRACT(DOW FROM last_played_at)::int AS dow, EXTRACT(HOUR FROM last_played_at)::int AS hr,
			COUNT(*) FROM watch_history WHERE user_id = $1
		GROUP BY dow, hr`, userID)
	if err == nil {
		defer heatmapRows.Close()
		var heatmap []map[string]interface{}
		for heatmapRows.Next() {
			var dow, hr, count int
			if heatmapRows.Scan(&dow, &hr, &count) == nil {
				heatmap = append(heatmap, map[string]interface{}{"day": dow, "hour": hr, "count": count})
			}
		}
		stats["heatmap"] = heatmap
	}

	// Favorite performers (top 5 by appearances in watched items)
	perfRows, err := s.db.Query(`
		SELECT p.name, COUNT(*) AS cnt FROM watch_history wh
		JOIN media_performers mp ON wh.media_item_id = mp.media_item_id
		JOIN performers p ON mp.performer_id = p.id
		WHERE wh.user_id = $1 AND wh.completed = TRUE AND mp.role = 'actor'
		GROUP BY p.name ORDER BY cnt DESC LIMIT 5`, userID)
	if err == nil {
		defer perfRows.Close()
		var performers []map[string]interface{}
		for perfRows.Next() {
			var name string
			var count int
			if perfRows.Scan(&name, &count) == nil {
				performers = append(performers, map[string]interface{}{"name": name, "count": count})
			}
		}
		stats["top_performers"] = performers
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: stats})
}

// ──────────────────── Trending / Popular (P9-02) ────────────────────

func (s *Server) handleGetTrending(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT m.id, m.title, m.poster_path, m.media_type, m.year, m.rating, m.updated_at,
			COUNT(DISTINCT wh.user_id) AS unique_viewers,
			COUNT(*) AS total_plays
		FROM watch_history wh
		JOIN media_items m ON wh.media_item_id = m.id
		WHERE wh.last_played_at > NOW() - INTERVAL '7 days'
		GROUP BY m.id
		ORDER BY unique_viewers DESC, total_plays DESC
		LIMIT 20`)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var title, mediaType string
		var poster *string
		var year *int
		var rating *float64
		var updatedAt time.Time
		var viewers, plays int
		if rows.Scan(&id, &title, &poster, &mediaType, &year, &rating, &updatedAt, &viewers, &plays) != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"id": id, "title": title, "poster_path": poster, "media_type": mediaType,
			"year": year, "rating": rating, "updated_at": updatedAt,
			"unique_viewers": viewers, "total_plays": plays,
		})
	}
	if items == nil { items = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: items})
}

// ──────────────────── Home Page Customization (P9-03) ────────────────────

func (s *Server) handleGetHomeLayout(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	rows, err := s.db.Query(`SELECT id, row_type, row_id, sort_position, is_visible
		FROM user_home_layout WHERE user_id = $1 ORDER BY sort_position`, userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var layout []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var rowType string
		var rowID *string
		var sortPos int
		var visible bool
		if rows.Scan(&id, &rowType, &rowID, &sortPos, &visible) != nil { continue }
		item := map[string]interface{}{"id": id, "row_type": rowType, "sort_position": sortPos, "is_visible": visible}
		if rowID != nil { item["row_id"] = *rowID }
		layout = append(layout, item)
	}
	if layout == nil {
		// Return default layout
		layout = []map[string]interface{}{
			{"row_type": "continue", "sort_position": 0, "is_visible": true},
			{"row_type": "on_deck", "sort_position": 1, "is_visible": true},
			{"row_type": "watchlist", "sort_position": 2, "is_visible": true},
			{"row_type": "favorites", "sort_position": 3, "is_visible": true},
			{"row_type": "trending", "sort_position": 4, "is_visible": true},
			{"row_type": "recent", "sort_position": 5, "is_visible": true},
		}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: layout})
}

func (s *Server) handleUpdateHomeLayout(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req []struct {
		RowType  string  `json:"row_type"`
		RowID    *string `json:"row_id"`
		SortPos  int     `json:"sort_position"`
		Visible  bool    `json:"is_visible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// Clear existing layout and replace
	s.db.Exec("DELETE FROM user_home_layout WHERE user_id = $1", userID)
	for _, row := range req {
		s.db.Exec("INSERT INTO user_home_layout (user_id, row_type, row_id, sort_position, is_visible) VALUES ($1, $2, $3, $4, $5)",
			userID, row.RowType, row.RowID, row.SortPos, row.Visible)
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ──────────────────── Genre Hub Pages (P9-04) ────────────────────

func (s *Server) handleGetGenreHub(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		s.respondError(w, http.StatusBadRequest, "slug required")
		return
	}

	// Find tag by slug
	var tagID uuid.UUID
	var tagName string
	err := s.db.QueryRow("SELECT id, name FROM tags WHERE slug = $1 AND category = 'genre'", slug).Scan(&tagID, &tagName)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "genre not found")
		return
	}

	// Get items in this genre (cross-library)
	limit := 50
	rows, err := s.db.Query(`
		SELECT m.id, m.title, m.poster_path, m.media_type, m.year, m.rating, m.updated_at
		FROM media_items m
		JOIN media_item_tags mt ON m.id = mt.media_item_id
		WHERE mt.tag_id = $1
		ORDER BY m.rating DESC NULLS LAST, m.year DESC NULLS LAST
		LIMIT $2`, tagID, limit)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var title, mediaType string
		var poster *string
		var year *int
		var rating *float64
		var updatedAt time.Time
		if rows.Scan(&id, &title, &poster, &mediaType, &year, &rating, &updatedAt) != nil { continue }
		items = append(items, map[string]interface{}{
			"id": id, "title": title, "poster_path": poster, "media_type": mediaType,
			"year": year, "rating": rating, "updated_at": updatedAt,
		})
	}
	if items == nil { items = []map[string]interface{}{} }

	// Get sub-genres (mood tags that commonly co-occur)
	var subGenres []string
	subRows, err := s.db.Query(`
		SELECT t2.name FROM media_item_tags mt1
		JOIN media_item_tags mt2 ON mt1.media_item_id = mt2.media_item_id
		JOIN tags t2 ON mt2.tag_id = t2.id AND t2.category = 'mood'
		WHERE mt1.tag_id = $1
		GROUP BY t2.name ORDER BY COUNT(*) DESC LIMIT 10`, tagID)
	if err == nil {
		defer subRows.Close()
		for subRows.Next() {
			var name string
			if subRows.Scan(&name) == nil { subGenres = append(subGenres, name) }
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"name": tagName, "slug": slug, "items": items, "sub_genres": subGenres, "total": len(items),
	}})
}

func (s *Server) handleGetDecadeHub(w http.ResponseWriter, r *http.Request) {
	decade := r.PathValue("year")
	if decade == "" {
		s.respondError(w, http.StatusBadRequest, "year required")
		return
	}

	var startYear, endYear int
	fmt.Sscanf(decade, "%d", &startYear)
	endYear = startYear + 9

	rows, err := s.db.Query(`
		SELECT m.id, m.title, m.poster_path, m.media_type, m.year, m.rating, m.updated_at
		FROM media_items m
		WHERE m.year >= $1 AND m.year <= $2
		ORDER BY m.rating DESC NULLS LAST, m.year DESC NULLS LAST
		LIMIT 50`, startYear, endYear)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var title, mediaType string
		var poster *string
		var year *int
		var rating *float64
		var updatedAt time.Time
		if rows.Scan(&id, &title, &poster, &mediaType, &year, &rating, &updatedAt) != nil { continue }
		items = append(items, map[string]interface{}{
			"id": id, "title": title, "poster_path": poster, "media_type": mediaType,
			"year": year, "rating": rating, "updated_at": updatedAt,
		})
	}
	if items == nil { items = []map[string]interface{}{} }

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"decade": fmt.Sprintf("%ds", startYear), "start_year": startYear, "end_year": endYear,
		"items": items, "total": len(items),
	}})
}

// ──────────────────── Year-in-Review / Wrapped (P9-06) ────────────────────

func (s *Server) handleGetWrapped(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	yearStr := r.PathValue("year")
	var year int
	fmt.Sscanf(yearStr, "%d", &year)
	if year < 2020 || year > 2030 {
		s.respondError(w, http.StatusBadRequest, "invalid year")
		return
	}

	startDate := fmt.Sprintf("%d-01-01", year)
	endDate := fmt.Sprintf("%d-12-31", year)

	wrapped := map[string]interface{}{"year": year}

	// Total hours watched
	var totalSec int
	s.db.QueryRow("SELECT COALESCE(SUM(progress_seconds), 0) FROM watch_history WHERE user_id = $1 AND last_played_at >= $2 AND last_played_at <= $3",
		userID, startDate, endDate).Scan(&totalSec)
	wrapped["total_hours"] = totalSec / 3600

	// Items watched
	var items int
	s.db.QueryRow("SELECT COUNT(DISTINCT media_item_id) FROM watch_history WHERE user_id = $1 AND completed = TRUE AND last_played_at >= $2 AND last_played_at <= $3",
		userID, startDate, endDate).Scan(&items)
	wrapped["items_watched"] = items

	// Top 5 genres
	genreRows, _ := s.db.Query(`
		SELECT t.name, COUNT(*) AS cnt FROM watch_history wh
		JOIN media_item_tags mt ON wh.media_item_id = mt.media_item_id
		JOIN tags t ON mt.tag_id = t.id AND t.category = 'genre'
		WHERE wh.user_id = $1 AND wh.completed = TRUE AND wh.last_played_at >= $2 AND wh.last_played_at <= $3
		GROUP BY t.name ORDER BY cnt DESC LIMIT 5`, userID, startDate, endDate)
	if genreRows != nil {
		defer genreRows.Close()
		var genres []string
		for genreRows.Next() {
			var name string; var cnt int
			if genreRows.Scan(&name, &cnt) == nil { genres = append(genres, name) }
		}
		wrapped["top_genres"] = genres
	}

	// Top 5 movies
	movieRows, _ := s.db.Query(`
		SELECT m.title, m.poster_path FROM watch_history wh
		JOIN media_items m ON wh.media_item_id = m.id
		WHERE wh.user_id = $1 AND wh.completed = TRUE AND wh.last_played_at >= $2 AND wh.last_played_at <= $3
		AND m.media_type NOT IN ('tv_shows', 'episodes')
		GROUP BY m.title, m.poster_path ORDER BY COUNT(*) DESC LIMIT 5`, userID, startDate, endDate)
	if movieRows != nil {
		defer movieRows.Close()
		var movies []map[string]interface{}
		for movieRows.Next() {
			var title string; var poster *string
			if movieRows.Scan(&title, &poster) == nil { movies = append(movies, map[string]interface{}{"title": title, "poster_path": poster}) }
		}
		wrapped["top_movies"] = movies
	}

	// Top 5 shows
	showRows, _ := s.db.Query(`
		SELECT ts.title, ts.poster_path, COUNT(*) AS cnt FROM watch_history wh
		JOIN media_items m ON wh.media_item_id = m.id
		JOIN tv_seasons s ON m.tv_season_id = s.id
		JOIN tv_shows ts ON s.tv_show_id = ts.id
		WHERE wh.user_id = $1 AND wh.completed = TRUE AND wh.last_played_at >= $2 AND wh.last_played_at <= $3
		GROUP BY ts.title, ts.poster_path ORDER BY cnt DESC LIMIT 5`, userID, startDate, endDate)
	if showRows != nil {
		defer showRows.Close()
		var shows []map[string]interface{}
		for showRows.Next() {
			var title string; var poster *string; var cnt int
			if showRows.Scan(&title, &poster, &cnt) == nil { shows = append(shows, map[string]interface{}{"title": title, "poster_path": poster, "episodes": cnt}) }
		}
		wrapped["top_shows"] = shows
	}

	// Longest binge (max consecutive watch time in a single day)
	var maxDailySec int
	s.db.QueryRow(`SELECT COALESCE(MAX(daily_sec), 0) FROM (
		SELECT DATE(last_played_at) AS d, SUM(progress_seconds) AS daily_sec
		FROM watch_history WHERE user_id = $1 AND last_played_at >= $2 AND last_played_at <= $3
		GROUP BY d) sub`, userID, startDate, endDate).Scan(&maxDailySec)
	wrapped["longest_binge_hours"] = maxDailySec / 3600

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: wrapped})
}

// ──────────────────── Content Requests (P9-07) ────────────────────

func (s *Server) handleCreateContentRequest(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		TMDBID      *int    `json:"tmdb_id"`
		Title       string  `json:"title"`
		Year        *int    `json:"year"`
		MediaType   string  `json:"media_type"`
		PosterURL   *string `json:"poster_url"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		s.respondError(w, http.StatusBadRequest, "title required")
		return
	}
	if req.MediaType == "" { req.MediaType = "movie" }

	id := uuid.New()
	_, err := s.db.Exec(`INSERT INTO content_requests (id, user_id, tmdb_id, title, year, media_type, poster_url, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, userID, req.TMDBID, req.Title, req.Year, req.MediaType, req.PosterURL, req.Description)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{"id": id}})
}

func (s *Server) handleListContentRequests(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	query := `SELECT cr.id, cr.user_id, u.display_name, cr.tmdb_id, cr.title, cr.year, cr.media_type,
		cr.poster_url, cr.description, cr.status, cr.admin_note, cr.created_at
		FROM content_requests cr JOIN users u ON cr.user_id = u.id`
	var args []interface{}
	if statusFilter != "" {
		query += " WHERE cr.status = $1"
		args = append(args, statusFilter)
	}
	query += " ORDER BY cr.created_at DESC LIMIT 100"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var requests []map[string]interface{}
	for rows.Next() {
		var id, uid uuid.UUID
		var displayName, title, mediaType, status string
		var tmdbID, year *int
		var posterURL, desc, adminNote *string
		var createdAt time.Time
		if rows.Scan(&id, &uid, &displayName, &tmdbID, &title, &year, &mediaType, &posterURL, &desc, &status, &adminNote, &createdAt) != nil {
			continue
		}
		requests = append(requests, map[string]interface{}{
			"id": id, "user_id": uid, "user_name": displayName, "tmdb_id": tmdbID,
			"title": title, "year": year, "media_type": mediaType, "poster_url": posterURL,
			"description": desc, "status": status, "admin_note": adminNote, "created_at": createdAt,
		})
	}
	if requests == nil { requests = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: requests})
}

func (s *Server) handleResolveContentRequest(w http.ResponseWriter, r *http.Request) {
	adminID := s.getUserID(r)
	reqID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request id")
		return
	}
	var body struct {
		Status    string  `json:"status"` // approved, denied, fulfilled
		AdminNote *string `json:"admin_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	_, err = s.db.Exec(`UPDATE content_requests SET status = $1, admin_note = $2, resolved_by = $3,
		resolved_at = NOW(), updated_at = NOW() WHERE id = $4`, body.Status, body.AdminNote, adminID, reqID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleGetMyContentRequests(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	rows, err := s.db.Query(`SELECT id, tmdb_id, title, year, media_type, poster_url, status, admin_note, created_at
		FROM content_requests WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var requests []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var title, mediaType, status string
		var tmdbID, year *int
		var posterURL, adminNote *string
		var createdAt time.Time
		if rows.Scan(&id, &tmdbID, &title, &year, &mediaType, &posterURL, &status, &adminNote, &createdAt) != nil { continue }
		requests = append(requests, map[string]interface{}{
			"id": id, "tmdb_id": tmdbID, "title": title, "year": year, "media_type": mediaType,
			"poster_url": posterURL, "status": status, "admin_note": adminNote, "created_at": createdAt,
		})
	}
	if requests == nil { requests = []map[string]interface{}{} }
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: requests})
}
