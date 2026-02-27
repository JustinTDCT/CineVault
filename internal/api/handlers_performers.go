package api

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Performer Handlers ────────────────────

func (s *Server) handleListPerformers(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	performers, err := s.performerRepo.List(search, limit, offset)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: performers})
}

func (s *Server) handleCreatePerformer(w http.ResponseWriter, r *http.Request) {
	var p models.Performer
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.ID = uuid.New()
	if err := s.performerRepo.Create(&p); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: p})
}

func (s *Server) handleGetPerformer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid performer id")
		return
	}

	performer, err := s.performerRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, err.Error())
		return
	}

	// Get linked media
	media, _ := s.performerRepo.GetPerformerMedia(id)

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"performer": performer,
		"media":     media,
	}})
}

func (s *Server) handleUpdatePerformer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid performer id")
		return
	}

	var p models.Performer
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.ID = id

	if err := s.performerRepo.Update(&p); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: p})
}

func (s *Server) handleDeletePerformer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid performer id")
		return
	}
	if err := s.performerRepo.Delete(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleGetMediaCast(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	cast, err := s.performerRepo.GetMediaCast(mediaID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: cast})
}

func (s *Server) handleLinkPerformer(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	var req struct {
		PerformerID   string `json:"performer_id"`
		Role          string `json:"role"`
		CharacterName string `json:"character_name"`
		SortOrder     int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	performerID, _ := uuid.Parse(req.PerformerID)
	if err := s.performerRepo.LinkMedia(mediaID, performerID, req.Role, req.CharacterName, req.SortOrder); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleUnlinkPerformer(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	performerID, err := uuid.Parse(r.PathValue("performerId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid performer id")
		return
	}
	if err := s.performerRepo.UnlinkMedia(mediaID, performerID); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// handlePopulatePerformerPhotos fetches TMDB credits for all media items that
// have performers without photos and downloads the headshots in the background.
func (s *Server) handlePopulatePerformerPhotos(w http.ResponseWriter, r *http.Request) {
	posterDir := s.config.Paths.Preview
	if posterDir == "" {
		s.respondError(w, http.StatusInternalServerError, "preview path not configured")
		return
	}

	var tmdbScraper *metadata.TMDBScraper
	for _, sc := range s.scrapers {
		if t, ok := sc.(*metadata.TMDBScraper); ok {
			tmdbScraper = t
			break
		}
	}
	if tmdbScraper == nil {
		s.respondError(w, http.StatusServiceUnavailable, "TMDB scraper not configured")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: "performer photo population started in background"})

	go s.populatePerformerPhotos(tmdbScraper, posterDir)
}

func (s *Server) populatePerformerPhotos(tmdbScraper *metadata.TMDBScraper, posterDir string) {
	log.Println("[performer-photos] starting bulk photo population")

	type mediaCreditsRow struct {
		MediaID    uuid.UUID
		MediaType  string
		ExternalIDs string
	}

	rows, err := s.mediaRepo.DB().Query(`
		SELECT DISTINCT m.id, m.media_type, m.external_ids::text
		FROM media_items m
		JOIN media_performers mp ON mp.media_item_id = m.id
		JOIN performers p ON p.id = mp.performer_id
		WHERE p.photo_path IS NULL
		  AND m.external_ids IS NOT NULL
		  AND m.external_ids::text LIKE '%tmdb_id%'
		  AND m.media_type IN ('movies', 'tv_shows', 'adult_movies')
	`)
	if err != nil {
		log.Printf("[performer-photos] query failed: %v", err)
		return
	}
	defer rows.Close()

	var items []mediaCreditsRow
	for rows.Next() {
		var r mediaCreditsRow
		if err := rows.Scan(&r.MediaID, &r.MediaType, &r.ExternalIDs); err != nil {
			continue
		}
		items = append(items, r)
	}
	log.Printf("[performer-photos] found %d media items to process", len(items))

	var updated, failed int64

	for i, item := range items {
		var extIDs struct {
			TMDBID string `json:"tmdb_id"`
		}
		if err := json.Unmarshal([]byte(item.ExternalIDs), &extIDs); err != nil || extIDs.TMDBID == "" {
			continue
		}

		var credits *metadata.TMDBCredits
		if item.MediaType == "tv_shows" {
			credits, err = tmdbScraper.GetTVCredits(extIDs.TMDBID)
		} else {
			credits, err = tmdbScraper.GetMovieCredits(extIDs.TMDBID)
		}
		if err != nil {
			atomic.AddInt64(&failed, 1)
			continue
		}

		castMap := make(map[string]string)
		for _, c := range credits.Cast {
			if c.ProfilePath != "" {
				castMap[strings.ToLower(c.Name)] = c.ProfilePath
			}
		}
		for _, c := range credits.Crew {
			if c.ProfilePath != "" {
				if _, exists := castMap[strings.ToLower(c.Name)]; !exists {
					castMap[strings.ToLower(c.Name)] = c.ProfilePath
				}
			}
		}

		performers, _ := s.performerRepo.GetMediaCast(item.MediaID)
		for _, p := range performers {
			if p.PhotoPath != nil {
				continue
			}
			profilePath, found := castMap[strings.ToLower(p.Name)]
			if !found {
				continue
			}
			photoURL := "https://image.tmdb.org/t/p/w185" + profilePath
			filename := "performer_" + p.PerformerID.String() + ".jpg"
			if _, dlErr := metadata.DownloadPoster(photoURL, filepath.Join(posterDir, "posters"), filename); dlErr == nil {
				webPath := "/previews/posters/" + filename
				s.mediaRepo.DB().Exec(
					`UPDATE performers SET photo_path = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2 AND photo_path IS NULL`,
					webPath, p.PerformerID)
				atomic.AddInt64(&updated, 1)
			}
		}

		if (i+1)%100 == 0 {
			log.Printf("[performer-photos] progress: %d/%d items processed, %d photos downloaded", i+1, len(items), atomic.LoadInt64(&updated))
		}
	}

	log.Printf("[performer-photos] complete: %d photos downloaded, %d TMDB failures out of %d items",
		atomic.LoadInt64(&updated), atomic.LoadInt64(&failed), len(items))

	if hub := s.wsHub; hub != nil {
		hub.Broadcast("performer_photos_done", map[string]int64{"updated": atomic.LoadInt64(&updated)})
	}
}
