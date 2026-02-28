package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// yearFromFilename extracts a 4-digit year from a filename, preferring the
// year closest to the filename's path (which is most likely the content year).
var identifyYearPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\((\d{4})\)`),            // (2021)
	regexp.MustCompile(`\[(\d{4})\]`),            // [2021]
	regexp.MustCompile(`[.\s_-](\d{4})[.\s_-]`),  // .2021. or -2021-
}

func yearFromFilename(filename string) *int {
	for _, p := range identifyYearPatterns {
		matches := p.FindStringSubmatch(filename)
		if len(matches) >= 2 {
			var y int
			fmt.Sscanf(matches[1], "%d", &y)
			if y >= 1900 && y <= 2100 {
				return &y
			}
		}
	}
	return nil
}

// getCacheClient returns a CacheClient if the cache server is enabled.
// Returns nil if disabled or registration fails.
func (s *Server) getCacheClient() *metadata.CacheClient {
	if s.settingsRepo == nil {
		return nil
	}
	enabled, _ := s.settingsRepo.Get("cache_server_enabled")
	if enabled == "false" {
		return nil
	}
	return metadata.EnsureRegistered(s.settingsRepo)
}

// ──────────────────── Metadata Handlers ────────────────────

func (s *Server) handleIdentifyMedia(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media not found")
		return
	}

	// Read configurable match thresholds
	matchCfg := metadata.ManualMatchConfig(s.settingsRepo)

	// Clean the title and search applicable scrapers
	query := metadata.CleanTitleForSearch(media.Title)
	if query == "" {
		query = media.Title
	}

	// Extract year from the filename (not the DB) to avoid using a previously bad match
	fileYear := yearFromFilename(media.FileName)
	if fileYear == nil {
		fileYear = yearFromFilename(media.FilePath)
	}

	var filtered []*models.MetadataMatch

	cacheClient := s.getCacheClient()
	if cacheClient != nil {
		filtered = cacheClient.Search(query, media.MediaType, fileYear,
			matchCfg.MinConfidence, matchCfg.MaxResults)
		log.Printf("Identify: cache search for %q returned %d results (min=%.2f max=%d)",
			query, len(filtered), matchCfg.MinConfidence, matchCfg.MaxResults)
	}

	if len(filtered) == 0 {
		if cacheClient != nil {
			log.Printf("Identify: cache returned 0 results for %q, falling back to direct scrapers", query)
		}
		scrapers := metadata.ScrapersForMediaType(s.scrapers, media.MediaType)
		if len(scrapers) == 0 {
			scrapers = s.scrapers
		}

		var allMatches []*models.MetadataMatch
		for _, scraper := range scrapers {
			matches, err := scraper.Search(query, media.MediaType, fileYear)
			if err != nil {
				log.Printf("Identify: scraper %q error for %q: %v", scraper.Name(), query, err)
				continue
			}
			allMatches = append(allMatches, matches...)
		}

		if fileYear != nil && *fileYear > 0 {
			for _, m := range allMatches {
				if m.Year != nil {
					diff := *fileYear - *m.Year
					if diff < 0 {
						diff = -diff
					}
					if diff == 0 {
						m.Confidence = min(m.Confidence+0.15, 1.0)
					} else if diff <= 1 {
						m.Confidence = min(m.Confidence+0.05, 1.0)
					} else {
						m.Confidence = max(m.Confidence-0.05, 0.0)
					}
				}
			}
		}

		filtered = make([]*models.MetadataMatch, 0, len(allMatches))
		for _, m := range allMatches {
			if m.Confidence >= matchCfg.MinConfidence {
				filtered = append(filtered, m)
			}
		}

		for i := 0; i < len(filtered); i++ {
			for j := i + 1; j < len(filtered); j++ {
				if filtered[j].Confidence > filtered[i].Confidence {
					filtered[i], filtered[j] = filtered[j], filtered[i]
				}
			}
		}

		if matchCfg.MaxResults > 0 && len(filtered) > matchCfg.MaxResults {
			filtered = filtered[:matchCfg.MaxResults]
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: filtered})
}

func (s *Server) handleApplyMetadata(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media not found")
		return
	}

	var req struct {
		Source      string   `json:"source"`
		ExternalID  string   `json:"external_id"`
		Title       string   `json:"title"`
		Year        *int     `json:"year"`
		Description *string  `json:"description"`
		PosterURL   *string  `json:"poster_url"`
		Rating      *float64 `json:"rating"`
		Genres      []string `json:"genres"`
		IMDBId      string   `json:"imdb_id"`
		ArtistName  string   `json:"artist_name"`
		AlbumTitle  string   `json:"album_title"`
		RecordLabel string   `json:"record_label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Download poster if URL provided
	var posterPath *string
	if req.PosterURL != nil && *req.PosterURL != "" && s.config.Paths.Preview != "" {
		filename := mediaID.String() + ".jpg"
		saved, dlErr := metadata.DownloadPoster(*req.PosterURL, filepath.Join(s.config.Paths.Preview, "posters"), filename)
		if dlErr != nil {
			log.Printf("Apply metadata: poster download failed for %s: %v", mediaID, dlErr)
		} else {
			// Use actual saved filename (may include _alt suffix from dedup)
			webPath := "/previews/posters/" + filepath.Base(saved)
			posterPath = &webPath
			log.Printf("Apply metadata: poster saved to %s", saved)
		}
	}

	// Apply metadata to media item and lock to prevent auto-overwrite
	if posterPath != nil {
		query := `UPDATE media_items SET title = $1, year = $2, description = $3, rating = $4,
			poster_path = $5, generated_poster = false, metadata_locked = true, updated_at = CURRENT_TIMESTAMP WHERE id = $6`
		_, err = s.db.DB.Exec(query, req.Title, req.Year, req.Description, req.Rating, *posterPath, mediaID)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		query := `UPDATE media_items SET title = $1, year = $2, description = $3, rating = $4,
			metadata_locked = true, updated_at = CURRENT_TIMESTAMP WHERE id = $5`
		_, err = s.db.DB.Exec(query, req.Title, req.Year, req.Description, req.Rating, mediaID)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Link genre tags
	if len(req.Genres) > 0 {
		s.linkGenreTags(mediaID, req.Genres)
	}

	// ── Music video metadata: link Artist, Album, and Record Label ──
	if media.MediaType == models.MediaTypeMusicVideos && req.Source == "musicbrainz" {
		s.linkMusicVideoMetadata(mediaID, media.LibraryID, req.ArtistName, req.AlbumTitle, req.RecordLabel, req.Year)
	}

	// ── Supplementary data (IMDB ID, ratings, credits) via cache or direct APIs ──
	imdbID := req.IMDBId
	var cacheResult *metadata.CacheLookupResult

	// ── Supplementary data: cache server is sole source when enabled ──
	cacheClient := s.getCacheClient()
	if cacheClient != nil {
		// Cache enabled — use cache server exclusively for all supplementary data
		cacheResult = cacheClient.Lookup(req.Title, req.Year, media.MediaType)
		if cacheResult != nil && cacheResult.Match != nil {
			if imdbID == "" && cacheResult.Match.IMDBId != "" {
				imdbID = cacheResult.Match.IMDBId
			}
			if cacheResult.Ratings != nil {
				_ = s.mediaRepo.UpdateRatings(mediaID, cacheResult.Ratings.IMDBRating, cacheResult.Ratings.RTScore, cacheResult.Ratings.AudienceScore)
			}
			if cacheResult.CastCrewJSON != nil && *cacheResult.CastCrewJSON != "" {
				credits := metadata.ParseCacheCredits(*cacheResult.CastCrewJSON)
				if credits != nil {
					s.enrichWithCredits(mediaID, credits)
				}
			}
			if len(req.Genres) == 0 && len(cacheResult.Genres) > 0 {
				s.linkGenreTags(mediaID, cacheResult.Genres)
			}
			// Store all unified metadata fields from cache
			if cacheResult.MetacriticScore != nil {
				_ = s.mediaRepo.UpdateMetacriticScore(mediaID, *cacheResult.MetacriticScore)
			}
			if cacheResult.ContentRatingsJSON != nil {
				_ = s.mediaRepo.UpdateContentRatingsJSON(mediaID, *cacheResult.ContentRatingsJSON)
			}
			if cacheResult.ContentRatingsAll != nil {
				resolved := metadata.ResolveContentRating(*cacheResult.ContentRatingsAll, "US")
				if resolved != "" {
					_ = s.mediaRepo.UpdateContentRating(mediaID, resolved)
				}
			}
			if cacheResult.TaglinesJSON != nil {
				_ = s.mediaRepo.UpdateField(mediaID, "taglines_json", *cacheResult.TaglinesJSON)
			}
			if cacheResult.TrailersJSON != nil {
				_ = s.mediaRepo.UpdateField(mediaID, "trailers_json", *cacheResult.TrailersJSON)
			}
			if cacheResult.DescriptionsJSON != nil {
				_ = s.mediaRepo.UpdateField(mediaID, "descriptions_json", *cacheResult.DescriptionsJSON)
			}
			log.Printf("Apply metadata: used cache for all supplementary data on %q", req.Title)
		}
	} else {
		// Cache disabled — fall back to direct APIs
		if imdbID == "" && req.Source == "tmdb" && req.ExternalID != "" {
			for _, sc := range s.scrapers {
				if t, ok := sc.(*metadata.TMDBScraper); ok {
					details, detailErr := t.GetDetails(req.ExternalID)
					if detailErr == nil && details.IMDBId != "" {
						imdbID = details.IMDBId
						if len(req.Genres) == 0 && len(details.Genres) > 0 {
							s.linkGenreTags(mediaID, details.Genres)
						}
					}
					break
				}
			}
		}
		if imdbID != "" {
			omdbKey, keyErr := s.settingsRepo.Get("omdb_api_key")
			if keyErr == nil && omdbKey != "" {
				ratings, omdbErr := metadata.FetchOMDbRatings(imdbID, omdbKey)
				if omdbErr != nil {
					log.Printf("Apply metadata: OMDb fetch failed for %s: %v", imdbID, omdbErr)
				} else {
					_ = s.mediaRepo.UpdateRatings(mediaID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore)
				}
			}
		}
		if req.Source == "tmdb" && req.ExternalID != "" {
			for _, sc := range s.scrapers {
				if t, ok := sc.(*metadata.TMDBScraper); ok {
					var credits *metadata.TMDBCredits
					var credErr error
					if media.MediaType == models.MediaTypeTVShows {
						credits, credErr = t.GetTVCredits(req.ExternalID)
					} else {
						credits, credErr = t.GetMovieCredits(req.ExternalID)
					}
					if credErr == nil && credits != nil {
						s.enrichWithCredits(mediaID, credits)
					}
					break
				}
			}
		}
	}

	// Store external IDs
	usedCache := cacheResult != nil && cacheResult.Match != nil
	if usedCache && cacheResult.ExternalIDsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(mediaID, *cacheResult.ExternalIDsJSON)
	} else {
		idsJSON := metadata.BuildExternalIDsFromMatch(req.Source, req.ExternalID, imdbID, false)
		if idsJSON != nil {
			_ = s.mediaRepo.UpdateExternalIDs(mediaID, *idsJSON)
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// linkGenreTags creates genre tags (if they don't exist) and links them to a media item.
func (s *Server) linkGenreTags(mediaItemID uuid.UUID, genres []string) {
	for _, genre := range genres {
		existing, _ := s.tagRepo.List("genre")
		var tagID uuid.UUID
		found := false
		for _, t := range existing {
			if strings.EqualFold(t.Name, genre) {
				tagID = t.ID
				found = true
				break
			}
		}
		if !found {
			tagID = uuid.New()
			tag := &models.Tag{
				ID:       tagID,
				Name:     genre,
				Category: models.TagCategoryGenre,
			}
			if err := s.tagRepo.Create(tag); err != nil {
				log.Printf("Apply metadata: create genre tag %q failed: %v", genre, err)
				continue
			}
		}
		if err := s.tagRepo.AssignToMedia(mediaItemID, tagID); err != nil {
			log.Printf("Apply metadata: assign genre tag %q to %s failed: %v", genre, mediaItemID, err)
		}
	}
}

// linkMusicVideoMetadata creates or finds Artist, Album, and Record Label records
// and links them to a music video media item.
func (s *Server) linkMusicVideoMetadata(mediaItemID, libraryID uuid.UUID, artistName, albumTitle, recordLabel string, year *int) {
	if artistName == "" {
		return
	}

	// Find or create Artist
	artist, err := s.musicRepo.FindArtistByName(libraryID, artistName)
	if err != nil {
		log.Printf("Music video metadata: find artist %q error: %v", artistName, err)
		return
	}
	if artist == nil {
		artist = &models.Artist{
			ID:        uuid.New(),
			LibraryID: libraryID,
			Name:      artistName,
		}
		if err := s.musicRepo.CreateArtist(artist); err != nil {
			log.Printf("Music video metadata: create artist %q failed: %v", artistName, err)
			return
		}
		log.Printf("Music video metadata: created artist %q", artistName)
	}

	// Link artist to media item
	s.db.DB.Exec(`UPDATE media_items SET artist_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, artist.ID, mediaItemID)

	// Find or create Album (if provided)
	if albumTitle != "" {
		album, err := s.musicRepo.FindAlbumByTitle(artist.ID, albumTitle)
		if err != nil {
			log.Printf("Music video metadata: find album %q error: %v", albumTitle, err)
		}
		if album == nil {
			album = &models.Album{
				ID:        uuid.New(),
				ArtistID:  artist.ID,
				LibraryID: libraryID,
				Title:     albumTitle,
				Year:      year,
			}
			if err := s.musicRepo.CreateAlbum(album); err != nil {
				log.Printf("Music video metadata: create album %q failed: %v", albumTitle, err)
			} else {
				log.Printf("Music video metadata: created album %q", albumTitle)
			}
		}
		if album != nil {
			s.db.DB.Exec(`UPDATE media_items SET album_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, album.ID, mediaItemID)
		}
	}

	// Find or create Record Label as Studio (if provided)
	if recordLabel != "" {
		studio, err := s.studioRepo.FindByNameAndType(recordLabel, models.StudioTypeLabel)
		if err != nil {
			log.Printf("Music video metadata: find label %q error: %v", recordLabel, err)
		}
		if studio == nil {
			studio = &models.Studio{
				ID:         uuid.New(),
				Name:       recordLabel,
				StudioType: models.StudioTypeLabel,
			}
			if err := s.studioRepo.Create(studio); err != nil {
				log.Printf("Music video metadata: create label %q failed: %v", recordLabel, err)
			} else {
				log.Printf("Music video metadata: created label %q", recordLabel)
			}
		}
		if studio != nil {
			_ = s.studioRepo.LinkMedia(mediaItemID, studio.ID, "label")
		}
	}
}

// enrichWithCredits creates or finds performers from TMDB credits and links them to a media item.
func (s *Server) enrichWithCredits(mediaItemID uuid.UUID, credits *metadata.TMDBCredits) {
	if credits == nil {
		return
	}

	// Import cast (top 20)
	maxCast := 20
	if len(credits.Cast) < maxCast {
		maxCast = len(credits.Cast)
	}
	for i := 0; i < maxCast; i++ {
		member := credits.Cast[i]
		if member.Name == "" {
			continue
		}
		performer, err := s.findOrCreatePerformer(member.Name, models.PerformerActor, member.ProfilePath, member.ID)
		if err != nil {
			log.Printf("Apply metadata: create performer %q failed: %v", member.Name, err)
			continue
		}
		charName := member.Character
		if err := s.performerRepo.LinkMedia(mediaItemID, performer.ID, "actor", charName, member.Order); err != nil {
			log.Printf("Apply metadata: link performer %q failed: %v", member.Name, err)
		}
	}

	// Import key crew
	importedCrew := 0
	for _, member := range credits.Crew {
		if member.Name == "" {
			continue
		}
		var perfType models.PerformerType
		switch member.Job {
		case "Director":
			perfType = models.PerformerDirector
		case "Producer", "Executive Producer":
			perfType = models.PerformerProducer
		case "Screenplay", "Writer", "Story":
			perfType = models.PerformerOther
		default:
			continue
		}
		performer, err := s.findOrCreatePerformer(member.Name, perfType, member.ProfilePath, member.ID)
		if err != nil {
			log.Printf("Apply metadata: create crew %q failed: %v", member.Name, err)
			continue
		}
		role := strings.ToLower(member.Job)
		if err := s.performerRepo.LinkMedia(mediaItemID, performer.ID, role, "", 100+importedCrew); err != nil {
			log.Printf("Apply metadata: link crew %q failed: %v", member.Name, err)
		}
		importedCrew++
	}
}

// findOrCreatePerformer finds an existing performer by name or creates a new one with optional photo.
// tmdbPersonID is the TMDB person ID used to fetch bio/photo from cache server.
func (s *Server) findOrCreatePerformer(name string, perfType models.PerformerType, profilePath string, tmdbPersonIDs ...int) (*models.Performer, error) {
	existing, err := s.performerRepo.FindByName(name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// Download photo if missing
		if existing.PhotoPath == nil && s.config.Paths.Preview != "" {
			photoURL := s.resolvePerformerPhoto(profilePath, tmdbPersonIDs...)
			if photoURL != "" {
				filename := "performer_" + existing.ID.String() + ".jpg"
				if _, dlErr := metadata.DownloadPoster(photoURL, filepath.Join(s.config.Paths.Preview, "posters"), filename); dlErr == nil {
					webPath := "/previews/posters/" + filename
					existing.PhotoPath = &webPath
					_ = s.performerRepo.Update(existing)
				}
			}
		}
		return existing, nil
	}

	p := &models.Performer{
		ID:            uuid.New(),
		Name:          name,
		PerformerType: perfType,
	}

	// Try cache server for bio data
	if len(tmdbPersonIDs) > 0 && tmdbPersonIDs[0] > 0 {
		if cacheClient := s.getCacheClient(); cacheClient != nil {
			if cp, cpErr := cacheClient.GetPerformer(tmdbPersonIDs[0]); cpErr == nil && cp != nil {
				if cp.Bio != nil { p.Bio = cp.Bio }
				if cp.BirthDate != nil {
					if t, tErr := time.Parse("2006-01-02", *cp.BirthDate); tErr == nil { p.BirthDate = &t }
				}
				if cp.DeathDate != nil {
					if t, tErr := time.Parse("2006-01-02", *cp.DeathDate); tErr == nil { p.DeathDate = &t }
				}
			}
		}
	}

	photoURL := s.resolvePerformerPhoto(profilePath, tmdbPersonIDs...)
	if photoURL != "" && s.config.Paths.Preview != "" {
		filename := "performer_" + p.ID.String() + ".jpg"
		if _, dlErr := metadata.DownloadPoster(photoURL, filepath.Join(s.config.Paths.Preview, "posters"), filename); dlErr == nil {
			webPath := "/previews/posters/" + filename
			p.PhotoPath = &webPath
		}
	}

	if err := s.performerRepo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

// resolvePerformerPhoto tries cache server photo first, falls back to TMDB profile path.
func (s *Server) resolvePerformerPhoto(profilePath string, tmdbPersonIDs ...int) string {
	// Try cache server for performer photo
	if len(tmdbPersonIDs) > 0 && tmdbPersonIDs[0] > 0 {
		if cacheClient := s.getCacheClient(); cacheClient != nil {
			if cp, err := cacheClient.GetPerformer(tmdbPersonIDs[0]); err == nil && cp != nil {
				if cp.PhotoPath != nil && *cp.PhotoPath != "" {
					return metadata.CacheImageURL(*cp.PhotoPath)
				}
				if cp.PhotoURL != nil && *cp.PhotoURL != "" {
					return *cp.PhotoURL
				}
			}
		}
	}
	// Fall back to direct TMDB
	if profilePath != "" {
		return "https://image.tmdb.org/t/p/w185" + profilePath
	}
	return ""
}

func (s *Server) handleRefreshMetadata(w http.ResponseWriter, r *http.Request) {
	libID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library id")
		return
	}

	// Enqueue as async job (deduplicated by library ID)
	if s.jobQueue != nil {
		uniqueID := "metadata-refresh:" + libID.String()
		jobID, err := s.jobQueue.EnqueueUnique("metadata:refresh", map[string]string{
			"library_id": libID.String(),
		}, uniqueID, asynq.Timeout(6*time.Hour), asynq.Retention(1*time.Hour))
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.respondJSON(w, http.StatusAccepted, Response{Success: true, Data: map[string]string{"job_id": jobID}})
		return
	}

	s.respondError(w, http.StatusServiceUnavailable, "job queue not available")
}

func (s *Server) handleAutoMatch(w http.ResponseWriter, r *http.Request) {
	libID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library id")
		return
	}

	// Enqueue as async job (deduplicated by library ID)
	if s.jobQueue != nil {
		uniqueID := "metadata:" + libID.String()
		jobID, err := s.jobQueue.EnqueueUnique("metadata:scrape", map[string]string{
			"library_id": libID.String(),
		}, uniqueID, asynq.Timeout(6*time.Hour), asynq.Retention(1*time.Hour))
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.respondJSON(w, http.StatusAccepted, Response{Success: true, Data: map[string]string{"job_id": jobID}})
		return
	}

	s.respondError(w, http.StatusServiceUnavailable, "job queue not available")
}

// ──────────────────── Per-Field Metadata Locking ────────────────────

func (s *Server) handleGetLockedFields(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"locked_fields":   media.LockedFields,
		"metadata_locked": media.MetadataLocked,
	}})
}

func (s *Server) handleUpdateLockedFields(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	var req struct {
		LockedFields []string `json:"locked_fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Determine if metadata_locked should be set (when "*" is in the list or any fields are locked)
	isLocked := len(req.LockedFields) > 0

	query := `UPDATE media_items SET locked_fields = $1, metadata_locked = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`
	_, err = s.db.DB.Exec(query, req.LockedFields, isLocked, mediaID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ──────────────────── Sort Order Handler ────────────────────

func (s *Server) handleUpdateSortOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntityType string   `json:"entity_type"`
		IDs        []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tableName := ""
	switch req.EntityType {
	case "media_items":
		tableName = "media_items"
	case "performers":
		tableName = "performers"
	case "tags":
		tableName = "tags"
	case "studios":
		tableName = "studios"
	case "edition_items":
		tableName = "edition_items"
	case "collection_items":
		tableName = "collection_items"
	case "tv_shows":
		tableName = "tv_shows"
	case "tv_seasons":
		tableName = "tv_seasons"
	default:
		s.respondError(w, http.StatusBadRequest, "unsupported entity type")
		return
	}

	for i, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		colName := "sort_position"
		if req.EntityType == "edition_items" || req.EntityType == "collection_items" {
			colName = "sort_position"
		}
		query := `UPDATE ` + tableName + ` SET ` + colName + ` = $1 WHERE id = $2`
		s.db.DB.Exec(query, i, id)
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ──────────────────── Job Status Handler ────────────────────

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.jobRepo.ListRecent(50)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: jobs})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	job, err := s.jobRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: job})
}

// ──────────────────── Playback Preferences ────────────────────

func (s *Server) handleGetPlaybackPrefs(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var pref models.UserPlaybackPreference
	query := `SELECT id, user_id, playback_mode, preferred_quality, auto_play_next,
		subtitle_language, audio_language, created_at, updated_at
		FROM user_playback_preferences WHERE user_id = $1`
	err := s.db.DB.QueryRow(query, userID).Scan(&pref.ID, &pref.UserID, &pref.PlaybackMode,
		&pref.PreferredQuality, &pref.AutoPlayNext, &pref.SubtitleLanguage, &pref.AudioLanguage,
		&pref.CreatedAt, &pref.UpdatedAt)
	if err != nil {
		// Return defaults
		pref = models.UserPlaybackPreference{
			UserID:           userID,
			PlaybackMode:     models.PlaybackAlwaysAsk,
			PreferredQuality: "1080p",
			AutoPlayNext:     true,
		}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: pref})
}

func (s *Server) handleUpdatePlaybackPrefs(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var pref models.UserPlaybackPreference
	if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	query := `INSERT INTO user_playback_preferences (id, user_id, playback_mode, preferred_quality, auto_play_next,
		subtitle_language, audio_language) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE SET playback_mode=$3, preferred_quality=$4, auto_play_next=$5,
		subtitle_language=$6, audio_language=$7, updated_at=CURRENT_TIMESTAMP`
	_, err := s.db.DB.Exec(query, uuid.New(), userID, pref.PlaybackMode, pref.PreferredQuality,
		pref.AutoPlayNext, pref.SubtitleLanguage, pref.AudioLanguage)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ──────────────────── System Settings ────────────────────

func (s *Server) handleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.settingsRepo.GetAll()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Mask sensitive keys for display
	for _, sensitiveKey := range []string{"omdb_api_key", "tvdb_api_key", "fanart_api_key"} {
		if val, ok := settings[sensitiveKey]; ok && len(val) > 4 {
			settings[sensitiveKey] = val[:4] + strings.Repeat("*", len(val)-4)
		}
	}
	// For cache_server_api_key, expose only whether it exists (for the "Registered" indicator)
	// but never return the actual value to the frontend
	if val, ok := settings["cache_server_api_key"]; ok && val != "" {
		settings["cache_server_api_key"] = "registered"
	}
	// Never expose the internal cache URL to the frontend
	delete(settings, "cache_server_url")
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: settings})
}

func (s *Server) handleUpdateSystemSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Sensitive keys that get masked – skip update if the value is still masked
	sensitiveKeys := map[string]bool{"omdb_api_key": true, "tvdb_api_key": true, "fanart_api_key": true}
	// Internal keys that the frontend should never overwrite
	internalKeys := map[string]bool{"cache_server_api_key": true, "cache_server_url": true}

	for key, value := range req {
		// Never allow the frontend to overwrite internal keys
		if internalKeys[key] {
			continue
		}
		// Don't overwrite real values with masked placeholders
		if sensitiveKeys[key] && strings.Contains(value, "****") {
			continue
		}
		if err := s.settingsRepo.Set(key, value); err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ──────────────────── Artwork Picker ────────────────────

// handleGetMediaArtwork returns all available poster/backdrop/logo URLs for a media item
// by looking up the item's TMDB ID in the cache server.
func (s *Server) handleGetMediaArtwork(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media item not found")
		return
	}

	cacheClient := s.getCacheClient()
	if cacheClient == nil {
		s.respondError(w, http.StatusServiceUnavailable, "cache server not enabled")
		return
	}

	result := cacheClient.Lookup(media.Title, media.Year, media.MediaType, media.EditionType)
	if result == nil || result.Match == nil {
		log.Printf("[artwork] lookup miss for %q (year=%v type=%s)", media.Title, media.Year, media.MediaType)
		s.respondError(w, http.StatusNotFound, "no cache data found")
		return
	}

	log.Printf("[artwork] %q → posters=%d backdrops=%d logos=%d",
		media.Title, len(result.AllPosterURLs), len(result.AllBackdropURLs), len(result.AllLogoURLs))

	type artworkResponse struct {
		Posters   []string `json:"posters"`
		Backdrops []string `json:"backdrops"`
		Logos     []string `json:"logos"`
	}

	resp := artworkResponse{
		Posters:   result.AllPosterURLs,
		Backdrops: result.AllBackdropURLs,
		Logos:     result.AllLogoURLs,
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: resp})
}

// handleSetMediaArtwork downloads a chosen artwork URL and sets it as the media item's poster or backdrop.
func (s *Server) handleSetMediaArtwork(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	var req struct {
		Type string `json:"type"` // "poster" or "backdrop"
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" || (req.Type != "poster" && req.Type != "backdrop") {
		s.respondError(w, http.StatusBadRequest, "provide type (poster|backdrop) and url")
		return
	}

	if s.config.Paths.Preview == "" {
		s.respondError(w, http.StatusInternalServerError, "preview path not configured")
		return
	}

	var subdir, suffix string
	if req.Type == "poster" {
		subdir = "posters"
		suffix = ".jpg"
	} else {
		subdir = "backdrops"
		suffix = "_backdrop.jpg"
	}

	// Delete old local image before downloading the new one
	media, _ := s.mediaRepo.GetByID(mediaID)
	if media != nil {
		var oldPath string
		if req.Type == "poster" && media.PosterPath != nil {
			oldPath = *media.PosterPath
		} else if req.Type == "backdrop" && media.BackdropPath != nil {
			oldPath = *media.BackdropPath
		}
		if oldPath != "" && strings.HasPrefix(oldPath, "/previews/") {
			diskPath := filepath.Join(s.config.Paths.Preview, strings.TrimPrefix(oldPath, "/previews/"))
			if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
				log.Printf("[artwork] failed to remove old %s: %v", diskPath, err)
			} else if err == nil {
				log.Printf("[artwork] removed old %s image: %s", req.Type, diskPath)
			}
		}
	}

	filename := mediaID.String() + suffix
	dir := filepath.Join(s.config.Paths.Preview, subdir)

	_, dlErr := metadata.DownloadPoster(req.URL, dir, filename)
	if dlErr != nil {
		s.respondError(w, http.StatusInternalServerError, "download failed: "+dlErr.Error())
		return
	}

	webPath := "/previews/" + subdir + "/" + filename
	var updateErr error
	if req.Type == "poster" {
		updateErr = s.mediaRepo.UpdatePosterPath(mediaID, webPath)
	} else {
		updateErr = s.mediaRepo.UpdateBackdropPath(mediaID, webPath)
	}
	if updateErr != nil {
		s.respondError(w, http.StatusInternalServerError, updateErr.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"path": webPath}})
}

// ──────────────────── Collection Artwork Picker ────────────────────

// handleGetCollectionArtwork returns all available poster/backdrop URLs for a collection
// by looking up the collection's TMDB ID in the cache server.
func (s *Server) handleGetCollectionArtwork(w http.ResponseWriter, r *http.Request) {
	collID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	coll, err := s.collectionRepo.GetByID(collID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "collection not found")
		return
	}

	cacheClient := s.getCacheClient()
	if cacheClient == nil {
		s.respondError(w, http.StatusServiceUnavailable, "cache server not enabled")
		return
	}

	// Find the TMDB collection ID linked to this local collection
	tmdbID := s.findCollectionTMDBID(coll)
	if tmdbID == 0 {
		s.respondError(w, http.StatusNotFound, "no TMDB collection linked")
		return
	}

	artwork, err := cacheClient.GetCollectionArtwork(tmdbID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "no artwork found: "+err.Error())
		return
	}

	// Convert to URL-only arrays for the picker UI (use cache image URL if path available, otherwise raw URL)
	type artworkResponse struct {
		Posters   []string `json:"posters"`
		Backdrops []string `json:"backdrops"`
	}
	resp := artworkResponse{}
	for _, p := range artwork.Posters {
		if p.Path != "" {
			resp.Posters = append(resp.Posters, metadata.CacheImageURL(p.Path))
		} else if p.URL != "" {
			resp.Posters = append(resp.Posters, p.URL)
		}
	}
	for _, b := range artwork.Backdrops {
		if b.Path != "" {
			resp.Backdrops = append(resp.Backdrops, metadata.CacheImageURL(b.Path))
		} else if b.URL != "" {
			resp.Backdrops = append(resp.Backdrops, b.URL)
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: resp})
}

// handleSetCollectionArtwork downloads a chosen artwork URL and sets it as the collection's poster or backdrop.
func (s *Server) handleSetCollectionArtwork(w http.ResponseWriter, r *http.Request) {
	collID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	var req struct {
		Type string `json:"type"` // "poster" or "backdrop"
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" || (req.Type != "poster" && req.Type != "backdrop") {
		s.respondError(w, http.StatusBadRequest, "provide type (poster|backdrop) and url")
		return
	}

	if s.config.Paths.Preview == "" {
		s.respondError(w, http.StatusInternalServerError, "preview path not configured")
		return
	}

	coll, err := s.collectionRepo.GetByID(collID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "collection not found")
		return
	}

	// Delete old local image
	if req.Type == "poster" && coll.PosterPath != nil {
		oldPath := *coll.PosterPath
		if strings.HasPrefix(oldPath, "/previews/") {
			diskPath := filepath.Join(s.config.Paths.Preview, strings.TrimPrefix(oldPath, "/previews/"))
			if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
				log.Printf("[artwork] failed to remove old collection %s: %v", diskPath, err)
			} else if err == nil {
				log.Printf("[artwork] removed old collection %s image: %s", req.Type, diskPath)
			}
		}
	}

	var subdir, suffix string
	if req.Type == "poster" {
		subdir = "posters"
		suffix = "_coll.jpg"
	} else {
		subdir = "backdrops"
		suffix = "_coll_backdrop.jpg"
	}

	filename := collID.String() + suffix
	dir := filepath.Join(s.config.Paths.Preview, subdir)

	_, dlErr := metadata.DownloadPoster(req.URL, dir, filename)
	if dlErr != nil {
		s.respondError(w, http.StatusInternalServerError, "download failed: "+dlErr.Error())
		return
	}

	webPath := "/previews/" + subdir + "/" + filename
	if req.Type == "poster" {
		coll.PosterPath = &webPath
		if err := s.collectionRepo.Update(coll); err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	// Note: backdrop_path not currently on client Collection model; poster is the primary use case

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"path": webPath}})
}

// findCollectionTMDBID looks up the TMDB collection ID for a local collection.
// It checks collection items via cache server lookup to find the TMDB collection.
func (s *Server) findCollectionTMDBID(coll *models.Collection) int {
	cacheClient := s.getCacheClient()
	if cacheClient == nil {
		return 0
	}

	// Get collection items and look up the first movie's TMDB collection
	items, err := s.collectionRepo.ListItems(coll.ID, "manual")
	if err != nil || len(items) == 0 {
		return 0
	}

	for _, item := range items {
		if item.MediaItemID == nil {
			continue
		}
		media, err := s.mediaRepo.GetByID(*item.MediaItemID)
		if err != nil || media == nil || media.MediaType != models.MediaTypeMovies {
			continue
		}
		result := cacheClient.Lookup(media.Title, media.Year, media.MediaType, media.EditionType)
		if result != nil && result.Match != nil && result.Match.CollectionID != nil && *result.Match.CollectionID > 0 {
			return *result.Match.CollectionID
		}
	}
	return 0
}
