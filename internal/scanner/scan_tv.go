package scanner

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

func (s *Scanner) handleTVHierarchy(library *models.Library, item *models.MediaItem, path string, basePath ...string) error {
	// Try to parse show name, season, episode from path
	base := library.Path
	if len(basePath) > 0 && basePath[0] != "" {
		base = basePath[0]
	}
	relPath, _ := filepath.Rel(base, path)
	showName, seasonNum, episodeNum := s.parseTVInfo(relPath, base)

	if showName == "" {
		return nil
	}

	log.Printf("TV parse: show=%q season=%d episode=%d (from %s)", showName, seasonNum, episodeNum, relPath)

	// Find or create show — use in-memory folder map to avoid duplicate creation
	// when autoMatchTVShow renames the title in the DB (e.g. "24 - Legacy" → "24").
	folderKey := library.ID.String() + "|" + strings.ToLower(showName)
	show, ok := s.scanShowsByFolder[folderKey]
	if !ok {
		var err error
		show, err = s.tvRepo.FindShowByTitle(library.ID, showName)
		if err != nil {
			return err
		}
		if show == nil {
			show = &models.TVShow{
				ID:        uuid.New(),
				LibraryID: library.ID,
				Title:     showName,
			}
			if err := s.tvRepo.CreateShow(show); err != nil {
				return fmt.Errorf("create show: %w", err)
			}
		}
		s.scanShowsByFolder[folderKey] = show
	}
	item.TVShowID = &show.ID

	if seasonNum > 0 {
		season, err := s.tvRepo.FindSeason(show.ID, seasonNum)
		if err != nil {
			return err
		}
		if season == nil {
			season = &models.TVSeason{
				ID:           uuid.New(),
				TVShowID:     show.ID,
				SeasonNumber: seasonNum,
			}
			if err := s.tvRepo.CreateSeason(season); err != nil {
				return fmt.Errorf("create season: %w", err)
			}
		}
		item.TVSeasonID = &season.ID
	}

	if episodeNum > 0 {
		item.EpisodeNumber = &episodeNum
	}

	return nil
}

// seasonDirPattern matches "Season N" or "Season NN" directory names
var seasonDirPattern = regexp.MustCompile(`(?i)^season\s*(\d+)$`)

func (s *Scanner) parseTVInfo(relPath string, libraryPath string) (showName string, season, episode int) {
	parts := strings.Split(relPath, string(filepath.Separator))
	filename := parts[len(parts)-1]

	// --- Step 1: Extract season and episode from filename using SxxExx patterns ---
	for _, pattern := range tvPatterns {
		matches := pattern.FindStringSubmatch(filename)
		if len(matches) >= 4 {
			season, _ = strconv.Atoi(matches[2])
			episode, _ = strconv.Atoi(matches[3])
			break
		}
	}

	// --- Step 2: Determine show name from directory structure ---
	// Structure A: Show/Season N/file.mkv  → relPath has 3+ parts, show = parts[0]
	// Structure B: Season N/file.mkv        → library IS the show folder, use parent dir name
	// Structure C: file.mkv                 → library IS the show folder, use parent dir name

	if len(parts) >= 3 {
		// Show/Season N/file.mkv
		showName = parts[0]
	} else if len(parts) == 2 {
		// Check if first dir is "Season N" → library path is the show
		if seasonDirPattern.MatchString(parts[0]) {
			showName = filepath.Base(libraryPath)
			// Also extract season from directory if not from filename
			if season == 0 {
				if m := seasonDirPattern.FindStringSubmatch(parts[0]); len(m) >= 2 {
					season, _ = strconv.Atoi(m[1])
				}
			}
		} else {
			// First part is show name
			showName = parts[0]
		}
	} else {
		// Just a file directly in library root → library name is the show
		showName = filepath.Base(libraryPath)
	}

	// --- Step 3: If season still unknown, check directory parts for "Season N" ---
	if season == 0 {
		for _, part := range parts[:len(parts)-1] {
			if m := seasonDirPattern.FindStringSubmatch(part); len(m) >= 2 {
				season, _ = strconv.Atoi(m[1])
				break
			}
		}
	}

	showName = s.cleanShowName(showName)
	return
}

func (s *Scanner) cleanShowName(name string) string {
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	// Remove trailing year in parens/brackets: "Show Name (2020)" → "Show Name"
	name = regexp.MustCompile(`\s*[\(\[]\d{4}[\)\]]\s*$`).ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	return name
}

// autoMatchTVShow searches for a TV show and applies metadata to the show record,
// then fetches episode-level metadata from TMDB for each season.
// Only runs once per show per scan.
func (s *Scanner) autoMatchTVShow(showID uuid.UUID) {
	if s.matchedShows[showID] {
		return
	}
	s.matchedShows[showID] = true

	show, err := s.tvRepo.GetShowByID(showID)
	if err != nil {
		return
	}

	// Skip if show already has metadata (description populated)
	if show.Description != nil && *show.Description != "" {
		// Still try to populate episode metadata if episodes lack it
		s.populateEpisodeMetadata(showID, show)
		return
	}

	searchQuery := metadata.CleanTitleForSearch(show.Title)
	if searchQuery == "" {
		return
	}

	// ── Try cache server first ──
	cacheClient := s.getCacheClient()
	if cacheClient != nil {
		result := cacheClient.Lookup(searchQuery, nil, models.MediaTypeTVShows)
		if result != nil && result.Match != nil {
			log.Printf("Auto-match TV: %q → %q (source=cache/%s, confidence=%.2f)",
				show.Title, result.Match.Title, result.Source, result.Confidence)
			s.applyTVShowCacheResult(showID, show, result)
			return
		}
	}

	// ── Fall back to direct TMDB ──
	match := metadata.FindBestMatch(s.scrapers, searchQuery, models.MediaTypeTVShows)
	if match == nil {
		log.Printf("Auto-match: no TV match for %q", searchQuery)
		return
	}

	log.Printf("Auto-match TV: %q → %q (source=%s, id=%s, confidence=%.2f)",
		show.Title, match.Title, match.Source, match.ExternalID, match.Confidence)

	// Download poster for the show
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" {
		filename := "tvshow_" + showID.String() + ".jpg"
		saved, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err != nil {
			log.Printf("Auto-match: TV poster download failed for %s: %v", showID, err)
		} else {
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
			_ = saved
		}
	}

	if err := s.tvRepo.UpdateShowMetadata(showID, match.Title, match.Year,
		match.Description, match.Rating, posterPath); err != nil {
		log.Printf("Auto-match: TV show DB update failed for %s: %v", showID, err)
	}

	// Enrich with genres and OMDb ratings using TMDB TV details
	if match.Source == "tmdb" && match.ExternalID != "" {
		s.enrichTVShowDetails(showID, match.ExternalID)
		// Queue episode-level metadata fetch for after all files are scanned
		s.pendingEpisodeMeta[showID] = match.ExternalID
	}

	// Contribute TV show to cache server in background
	if cacheClient != nil {
		go cacheClient.Contribute(match, metadata.ContributeExtras{
			MediaType: models.MediaTypeTVShows,
		})
	}
}

// applyTVShowCacheResult applies a cache server hit to a TV show record,
// including poster, genres, ratings, and queuing episode metadata fetch.
func (s *Scanner) applyTVShowCacheResult(showID uuid.UUID, show *models.TVShow, result *metadata.CacheLookupResult) {
	match := result.Match

	// Download poster if available
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" {
		filename := "tvshow_" + showID.String() + ".jpg"
		_, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err != nil {
			log.Printf("Auto-match: TV poster download failed for %s: %v", showID, err)
		} else {
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
		}
	}

	if err := s.tvRepo.UpdateShowMetadata(showID, match.Title, match.Year,
		match.Description, match.Rating, posterPath); err != nil {
		log.Printf("Auto-match: TV show DB update failed for %s: %v", showID, err)
	}

	// Get all episodes for this show to apply genres/ratings
	episodes, err := s.mediaRepo.ListByTVShow(showID)
	if err != nil {
		log.Printf("Auto-match: failed to list episodes for genre/rating enrichment: %v", err)
		episodes = nil
	}

	// Link genre tags to all episodes
	if s.tagRepo != nil && len(result.Genres) > 0 && episodes != nil {
		for _, ep := range episodes {
			if !ep.IsFieldLocked("genres") {
				s.linkGenreTags(ep.ID, result.Genres)
			}
		}
	}

	// Apply OMDb ratings from cache to all episodes
	if result.Ratings != nil && episodes != nil {
		for _, ep := range episodes {
			_ = s.mediaRepo.UpdateRatingsWithLocks(ep.ID, result.Ratings.IMDBRating, result.Ratings.RTScore, result.Ratings.AudienceScore, ep.LockedFields)
		}
	}

	// Use cast/crew from cache if available
	if s.performerRepo != nil && result.CastCrewJSON != nil && *result.CastCrewJSON != "" && len(episodes) > 0 {
		credits := parseCacheCredits(*result.CastCrewJSON)
		if credits != nil {
			s.enrichWithCredits(episodes[0].ID, credits)
		}
	}

	// Queue episode-level metadata fetch if we have a TMDB external ID
	if match.ExternalID != "" {
		s.pendingEpisodeMeta[showID] = match.ExternalID
	}
}

// enrichTVShowDetails fetches TV show details for genres and OMDb ratings,
// and applies them to all episodes in the show.
func (s *Scanner) enrichTVShowDetails(showID uuid.UUID, tmdbExternalID string) {
	var tmdbScraper *metadata.TMDBScraper
	for _, sc := range s.scrapers {
		if t, ok := sc.(*metadata.TMDBScraper); ok {
			tmdbScraper = t
			break
		}
	}
	if tmdbScraper == nil {
		return
	}

	details, err := tmdbScraper.GetTVDetails(tmdbExternalID)
	if err != nil {
		log.Printf("Auto-match: TMDB TV details failed for %s: %v", tmdbExternalID, err)
		return
	}

	// Get all episodes for this show to apply genres/ratings
	episodes, err := s.mediaRepo.ListByTVShow(showID)
	if err != nil {
		log.Printf("Auto-match: failed to list episodes for genre/rating enrichment: %v", err)
		return
	}

	// Link genre tags to all episodes (respecting per-episode locks)
	if s.tagRepo != nil && len(details.Genres) > 0 {
		for _, ep := range episodes {
			if !ep.IsFieldLocked("genres") {
				s.linkGenreTags(ep.ID, details.Genres)
			}
		}
	}

	// Fetch OMDb ratings and apply to all episodes (respecting per-episode locks)
	if s.settingsRepo != nil && details.IMDBId != "" {
		omdbKey, err := s.settingsRepo.Get("omdb_api_key")
		if err != nil {
			log.Printf("Auto-match: settings lookup failed: %v", err)
		} else if omdbKey != "" {
			ratings, err := metadata.FetchOMDbRatings(details.IMDBId, omdbKey)
			if err != nil {
				log.Printf("Auto-match: OMDb fetch failed for TV %s: %v", details.IMDBId, err)
			} else {
				for _, ep := range episodes {
					if err := s.mediaRepo.UpdateRatingsWithLocks(ep.ID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore, ep.LockedFields); err != nil {
						log.Printf("Auto-match: ratings update failed for episode %s: %v", ep.ID, err)
					}
				}
			}
		}
	}

	// Fetch and link TV show cast to all episodes
	if s.performerRepo != nil {
		credits, err := tmdbScraper.GetTVCredits(tmdbExternalID)
		if err != nil {
			log.Printf("Auto-match: TMDB TV credits failed for %s: %v", tmdbExternalID, err)
		} else if len(episodes) > 0 {
			// Link cast to first episode as representative (avoid massive duplication)
			s.enrichWithCredits(episodes[0].ID, credits)
		}
	}
}

// fetchEpisodeMetadata uses the cache server (with TMDB fallback) to fetch
// season details and apply episode titles, descriptions, and still images.
func (s *Scanner) fetchEpisodeMetadata(showID uuid.UUID, tmdbShowID string) {
	// Get the TMDB scraper as fallback
	var tmdb *metadata.TMDBScraper
	for _, sc := range s.scrapers {
		if t, ok := sc.(*metadata.TMDBScraper); ok {
			tmdb = t
			break
		}
	}
	if tmdb == nil {
		return
	}

	cacheClient := s.getCacheClient()
	tmdbIDInt, _ := strconv.Atoi(tmdbShowID)

	// Get all seasons for this show
	seasons, err := s.tvRepo.ListSeasonsByShow(showID)
	if err != nil {
		log.Printf("Auto-match: failed to list seasons for show %s: %v", showID, err)
		return
	}

	// Get all episodes for this show
	episodes, err := s.mediaRepo.ListByTVShow(showID)
	if err != nil {
		log.Printf("Auto-match: failed to list episodes for show %s: %v", showID, err)
		return
	}

	for _, season := range seasons {
		// Try cache server first, fall back to direct TMDB
		var seasonName, seasonOverview, seasonPosterPath string
		tmdbMap := make(map[int]metadata.TMDBEpisode)

		if cacheClient != nil && tmdbIDInt > 0 {
			cacheSeason, cacheErr := cacheClient.GetTVSeason(tmdbIDInt, season.SeasonNumber)
			if cacheErr == nil && cacheSeason != nil {
				log.Printf("Auto-match: S%02d fetched from cache server", season.SeasonNumber)
				if cacheSeason.Title != nil { seasonName = *cacheSeason.Title }
				if cacheSeason.Description != nil { seasonOverview = *cacheSeason.Description }
				if cacheSeason.PosterURL != nil { seasonPosterPath = *cacheSeason.PosterURL }
				for _, ep := range cacheSeason.Episodes {
					tmdbEp := metadata.TMDBEpisode{EpisodeNumber: ep.EpisodeNumber}
					if ep.Title != nil { tmdbEp.Name = *ep.Title }
					if ep.Description != nil { tmdbEp.Overview = *ep.Description }
					if ep.StillURL != nil { tmdbEp.StillPath = *ep.StillURL }
					if ep.Rating != nil { tmdbEp.VoteAverage = *ep.Rating }
					tmdbMap[ep.EpisodeNumber] = tmdbEp
				}
			}
		}

		// Fall back to direct TMDB if cache had no data
		if len(tmdbMap) == 0 {
			seasonData, tmdbErr := tmdb.GetTVSeasonDetails(tmdbShowID, season.SeasonNumber)
			if tmdbErr != nil {
				log.Printf("Auto-match: TMDB season %d fetch failed for show %s: %v", season.SeasonNumber, tmdbShowID, tmdbErr)
				continue
			}
			seasonName = seasonData.Name
			seasonOverview = seasonData.Overview
			if seasonData.PosterPath != "" {
				seasonPosterPath = "https://image.tmdb.org/t/p/w500" + seasonData.PosterPath
			}
			for _, ep := range seasonData.Episodes {
				tmdbMap[ep.EpisodeNumber] = ep
			}
		}

		// Download and save season poster
		if seasonPosterPath != "" && s.posterDir != "" {
			filename := "season_" + season.ID.String() + ".jpg"
			if _, dlErr := metadata.DownloadPoster(seasonPosterPath, filepath.Join(s.posterDir, "posters"), filename); dlErr != nil {
				log.Printf("Auto-match: season poster download failed: %v", dlErr)
			} else {
				webPath := "/previews/posters/" + filename
				var title *string
				if seasonName != "" {
					title = &seasonName
				}
				var desc *string
				if seasonOverview != "" {
					desc = &seasonOverview
				}
				if err := s.tvRepo.UpdateSeasonMetadata(season.ID, title, desc, &webPath); err != nil {
					log.Printf("Auto-match: season metadata update failed: %v", err)
				} else {
					log.Printf("Auto-match season: S%02d poster saved", season.SeasonNumber)
				}
			}
		}

		// Match local episodes to TMDB episodes
		for _, ep := range episodes {
			if ep.TVSeasonID == nil || *ep.TVSeasonID != season.ID || ep.EpisodeNumber == nil {
				continue
			}

			// Skip fully locked episodes
			if ep.MetadataLocked || ep.IsFieldLocked("*") {
				continue
			}

			tmdbEp, ok := tmdbMap[*ep.EpisodeNumber]
			if !ok {
				continue
			}

			// Build episode title: "Episode Name" or keep original if TMDB has none
			epTitle := tmdbEp.Name
			if epTitle == "" {
				continue
			}

			var desc *string
			if tmdbEp.Overview != "" {
				desc = &tmdbEp.Overview
			}

			var rating *float64
			if tmdbEp.VoteAverage > 0 {
				rating = &tmdbEp.VoteAverage
			}

			// Download episode still image (respecting poster lock)
			var posterPath *string
			if tmdbEp.StillPath != "" && s.posterDir != "" && !ep.IsFieldLocked("poster_path") {
				stillURL := "https://image.tmdb.org/t/p/w500" + tmdbEp.StillPath
				filename := "ep_" + ep.ID.String() + ".jpg"
				saved, dlErr := metadata.DownloadPoster(stillURL, filepath.Join(s.posterDir, "posters"), filename)
				if dlErr != nil {
					log.Printf("Auto-match: episode still download failed: %v", dlErr)
				} else {
					webPath := "/previews/posters/" + filename
					posterPath = &webPath
					_ = saved
				}
			}

			if err := s.mediaRepo.UpdateMetadataWithLocks(ep.ID, epTitle, nil, desc, rating, posterPath, nil, ep.LockedFields); err != nil {
				log.Printf("Auto-match: episode metadata update failed for %s: %v", ep.ID, err)
			} else {
				log.Printf("Auto-match episode: S%02dE%02d → %q", season.SeasonNumber, *ep.EpisodeNumber, epTitle)
			}
		}
	}
}

// populateEpisodeMetadata is called when the show already has metadata but episodes may not.
// It re-searches TMDB to get the show ID and queues episode metadata for post-scan.
func (s *Scanner) populateEpisodeMetadata(showID uuid.UUID, show *models.TVShow) {
	// Search TMDB to get the show's external ID
	searchQuery := metadata.CleanTitleForSearch(show.Title)
	if searchQuery == "" {
		return
	}
	match := metadata.FindBestMatch(s.scrapers, searchQuery, models.MediaTypeTVShows)
	if match == nil || match.Source != "tmdb" || match.ExternalID == "" {
		return
	}

	s.pendingEpisodeMeta[showID] = match.ExternalID
}

// extractYear tries to find a 4-digit year in a filename using the improved patterns.
func (s *Scanner) extractYear(filename string) *int {
	// Try parens/brackets first: (2020) or [2020]
	if m := yearInParensRx.FindStringSubmatch(filename); len(m) >= 2 {
		year, err := strconv.Atoi(m[1])
		if err == nil && year >= 1900 && year <= 2100 {
			return &year
		}
	}
	// Try delimited: .2020. -2020-
	if m := yearRx.FindStringSubmatch(filename); len(m) >= 2 {
		year, err := strconv.Atoi(m[1])
		if err == nil && year >= 1900 && year <= 2100 {
			return &year
		}
	}
	return nil
}
