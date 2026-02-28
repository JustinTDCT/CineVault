package scanner

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// flushPendingMeta sends all deferred items to the cache server in batches of 50,
// then applies results. Much faster than per-file individual HTTP calls.
func (s *Scanner) flushPendingMeta() {
	if len(s.pendingMeta) == 0 {
		return
	}

	cacheClient := s.getCacheClient()
	if cacheClient == nil {
		return
	}

	items := s.pendingMeta
	s.pendingMeta = nil
	log.Printf("Batch metadata: looking up %d items from cache server...", len(items))

	lookupItems := make([]metadata.BatchLookupItem, len(items))
	for i, pm := range items {
		lookupItems[i] = metadata.BatchLookupItem{
			Title:     pm.Query,
			Year:      pm.Item.Year,
			MediaType: pm.Item.MediaType,
		}
	}

	results := cacheClient.BatchLookup(lookupItems)

	hits := 0
	for i, result := range results {
		if result == nil || result.Match == nil {
			continue
		}
		hits++
		pm := items[i]
		log.Printf("Auto-match: %q → %q (source=cache/%s, confidence=%.2f)",
			pm.Item.Title, result.Match.Title, result.Source, result.Confidence)
		s.applyCacheResult(pm.Item, result)
	}
	log.Printf("Batch metadata: %d/%d hits from cache server", hits, len(items))
}

// but are missing OMDb ratings or cast/crew, and re-enriches them using concurrent workers.
func (s *Scanner) reEnrichExistingItems(library *models.Library, onProgress ProgressFunc) {
	items, err := s.mediaRepo.ListItemsNeedingEnrichment(library.ID)
	if err != nil {
		log.Printf("Re-enrich: failed to list items: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}

	// Pre-fetch and cache the OMDb API key once
	var omdbKey string
	if s.settingsRepo != nil {
		omdbKey, _ = s.settingsRepo.Get("omdb_api_key")
	}

	// Find the TMDB scraper once
	var tmdbScraper *metadata.TMDBScraper
	for _, sc := range s.scrapers {
		if t, ok := sc.(*metadata.TMDBScraper); ok {
			tmdbScraper = t
			break
		}
	}
	if tmdbScraper == nil {
		log.Printf("Re-enrich: no TMDB scraper available")
		return
	}

	// Filter items upfront (skip fully locked items)
	var enrichItems []*models.MediaItem
	for _, item := range items {
		if item.MetadataLocked || item.IsFieldLocked("*") {
			continue
		}
		if item.MediaType == models.MediaTypeTVShows && item.TVShowID != nil && library.SeasonGrouping {
			continue
		}
		enrichItems = append(enrichItems, item)
	}
	if len(enrichItems) == 0 {
		return
	}

	total := len(enrichItems)
	log.Printf("Re-enrich: %d items need OMDb ratings or cast enrichment (using 5 workers)", total)
	if onProgress != nil {
		onProgress(0, total, 0, "Enriching metadata...")
	}

	// Concurrent worker pool
	const numWorkers = 5
	itemCh := make(chan *models.MediaItem, numWorkers*2)
	var wg sync.WaitGroup
	var processed int64

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range itemCh {
				s.enrichItemFast(item, tmdbScraper, omdbKey)
				cur := atomic.AddInt64(&processed, 1)
				if onProgress != nil && (cur%10 == 0 || int(cur) == total) {
					onProgress(int(cur), total, 0, item.Title)
				}
			}
		}()
	}

	for _, item := range enrichItems {
		itemCh <- item
	}
	close(itemCh)
	wg.Wait()

	log.Printf("Re-enrich: completed %d items", total)
}

// enrichItemFast enriches a single item using the cache server or the combined
// TMDB details+credits endpoint, plus OMDb for ratings. Thread-safe for concurrent use.
func (s *Scanner) enrichItemFast(item *models.MediaItem, tmdbScraper *metadata.TMDBScraper, omdbKey string) {
	searchQuery := metadata.CleanTitleForSearch(item.Title)
	if searchQuery == "" {
		return
	}

	// ── Cache server is sole source when enabled ──
	cacheClient := s.getCacheClient()
	if cacheClient != nil {
		result := cacheClient.Lookup(searchQuery, item.Year, item.MediaType, item.EditionType)
		if result != nil && result.Match != nil {
			log.Printf("Re-enrich: %q → %q (source=cache/%s)", item.Title, result.Match.Title, result.Source)

			// Download poster if cache provides one and current poster is a generated screenshot
			if item.GeneratedPoster && result.Match.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
				filename := item.ID.String() + ".jpg"
				posterDir := filepath.Join(s.posterDir, "posters")
				// Remove generated screenshot so dedup doesn't save TMDB poster as _alt
				_ = os.Remove(filepath.Join(posterDir, filename))
				_, err := metadata.DownloadPoster(*result.Match.PosterURL, posterDir, filename)
				if err != nil {
					log.Printf("Re-enrich: poster download failed for %s: %v", item.ID, err)
				} else {
					webPath := "/previews/posters/" + filename
					_ = s.mediaRepo.UpdatePosterPath(item.ID, webPath)
				}
			}

			// Link genre tags from cache
			if s.tagRepo != nil && len(result.Genres) > 0 && !item.IsFieldLocked("genres") {
				s.linkGenreTags(item.ID, result.Genres)
			}

			// Link mood tags and store keywords from cache
			if len(result.Keywords) > 0 {
				s.linkMoodTags(item.ID, result.Keywords)
				s.storeKeywords(item.ID, result.Keywords)
			}

			// Apply OMDb ratings from cache
			if result.Ratings != nil {
				_ = s.mediaRepo.UpdateRatingsWithLocks(item.ID, result.Ratings.IMDBRating, result.Ratings.RTScore, result.Ratings.AudienceScore, item.LockedFields)
			}

			// Use cast/crew from cache if available, otherwise fall back to TMDB
			if s.performerRepo != nil && !item.IsFieldLocked("cast") {
				if result.CastCrewJSON != nil && *result.CastCrewJSON != "" {
					credits := parseCacheCredits(*result.CastCrewJSON)
					if credits != nil {
						s.enrichWithCredits(item.ID, credits)
					}
				} else if result.Match.ExternalID != "" && tmdbScraper != nil {
					combined, err := tmdbScraper.GetDetailsWithCredits(result.Match.ExternalID)
					if err == nil && combined.Credits != nil {
						s.enrichWithCredits(item.ID, combined.Credits)
					}
				}
			}

			// Store external IDs from cache
			if result.ExternalIDsJSON != nil {
				_ = s.mediaRepo.UpdateExternalIDs(item.ID, *result.ExternalIDsJSON)
			}

			// Apply extended metadata from cache (respecting per-field locks)
			tagline, origLang, country, trailerURL, logoURL := filterLockedExtended(item.LockedFields,
				result.Match.Tagline, result.Match.OriginalLanguage, result.Match.Country, result.Match.TrailerURL, result.LogoURL)
			extUpdate := &repository.ExtendedMetadataUpdate{
				Tagline:          tagline,
				OriginalLanguage: origLang,
				Country:          country,
				TrailerURL:       trailerURL,
				LogoPath:         logoURL,
			}
			if result.OriginalTitle != nil && !isFieldLocked(item.LockedFields, "title") {
				extUpdate.OriginalTitle = result.OriginalTitle
			}
			if result.SortTitle != nil && !isFieldLocked(item.LockedFields, "title") {
				extUpdate.SortTitle = result.SortTitle
			}
			if result.ReleaseDate != nil && !isFieldLocked(item.LockedFields, "year") {
				extUpdate.ReleaseDate = result.ReleaseDate
			}
			_ = s.mediaRepo.UpdateExtendedMetadataFull(item.ID, extUpdate)

			// Auto-create collection from cache
			if result.Match.CollectionID != nil && result.Match.CollectionName != nil {
				s.autoCreateCollection(item, result.Match)
			}

			// Store unified metadata fields from cache
			s.storeUnifiedCacheFields(item.ID, result)
			return
		}
		// Cache enabled = sole source. No fallback.
		log.Printf("Re-enrich: cache miss for %q (no fallback, cache is sole source)", searchQuery)
		return
	}

	match := metadata.FindBestMatch(s.scrapers, searchQuery, item.MediaType, item.Year)
	if match == nil || match.Source != "tmdb" || match.ExternalID == "" {
		return
	}

	log.Printf("Re-enrich: %q → TMDB ID %s", item.Title, match.ExternalID)

	// Use combined details+credits call (1 TMDB request instead of 2)
	combined, err := tmdbScraper.GetDetailsWithCredits(match.ExternalID)
	if err != nil {
		log.Printf("Re-enrich: TMDB details+credits failed for %s: %v", match.ExternalID, err)
		return
	}

	// Link genre tags (respecting per-field lock)
	if s.tagRepo != nil && len(combined.Details.Genres) > 0 && !item.IsFieldLocked("genres") {
		s.linkGenreTags(item.ID, combined.Details.Genres)
	}

	// Link mood tags and store keywords from TMDB
	if len(combined.Details.Keywords) > 0 {
		s.linkMoodTags(item.ID, combined.Details.Keywords)
		s.storeKeywords(item.ID, combined.Details.Keywords)
	}

	// Fetch OMDb ratings (respecting per-field locks)
	if omdbKey != "" && combined.Details.IMDBId != "" {
		ratings, err := metadata.FetchOMDbRatings(combined.Details.IMDBId, omdbKey)
		if err != nil {
			log.Printf("Re-enrich: OMDb fetch failed for %s: %v", combined.Details.IMDBId, err)
		} else {
			if err := s.mediaRepo.UpdateRatingsWithLocks(item.ID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore, item.LockedFields); err != nil {
				log.Printf("Re-enrich: ratings update failed for %s: %v", item.ID, err)
			}
		}
	}

	// Replace generated screenshot poster with TMDB poster (respecting poster lock)
	if item.GeneratedPoster && combined.Details.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
		filename := item.ID.String() + ".jpg"
		pDir := filepath.Join(s.posterDir, "posters")
		_ = os.Remove(filepath.Join(pDir, filename))
		_, dlErr := metadata.DownloadPoster(*combined.Details.PosterURL, pDir, filename)
		if dlErr != nil {
			log.Printf("Re-enrich: poster download failed for %s: %v", item.ID, dlErr)
		} else {
			webPath := "/previews/posters/" + filename
			_ = s.mediaRepo.UpdatePosterPath(item.ID, webPath)
			log.Printf("Re-enrich: replaced generated poster for %q", item.Title)
		}
	}

	// Populate cast/crew from the credits already fetched (respecting cast lock)
	if s.performerRepo != nil && combined.Credits != nil && !item.IsFieldLocked("cast") {
		s.enrichWithCredits(item.ID, combined.Credits)
	}

	// Store external IDs from direct TMDB match
	idsJSON := metadata.BuildExternalIDsFromMatch("tmdb", match.ExternalID, combined.Details.IMDBId, false)
	if idsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(item.ID, *idsJSON)
	}

	// Apply extended metadata from TMDB details (respecting per-field locks)
	d := combined.Details
	eTagline, eLang, eCountry, eTrailer, eLogo := filterLockedExtended(item.LockedFields,
		d.Tagline, d.OriginalLanguage, d.Country, d.TrailerURL, nil)
	extUpdate := &repository.ExtendedMetadataUpdate{
		Tagline:          eTagline,
		OriginalLanguage: eLang,
		Country:          eCountry,
		TrailerURL:       eTrailer,
		LogoPath:         eLogo,
	}
	if d.OriginalTitle != nil && !isFieldLocked(item.LockedFields, "title") {
		extUpdate.OriginalTitle = d.OriginalTitle
	}
	if d.ReleaseDate != nil && !isFieldLocked(item.LockedFields, "year") {
		extUpdate.ReleaseDate = d.ReleaseDate
	}
	_ = s.mediaRepo.UpdateExtendedMetadataFull(item.ID, extUpdate)

	// Auto-create movie collection from TMDB belongs_to_collection
	if combined.Details.CollectionID != nil && combined.Details.CollectionName != nil {
		s.autoCreateCollection(item, combined.Details)
	}

	// Contribute to cache server with cast/crew, ratings, and extended metadata
	if cacheClient != nil {
		extras := metadata.ContributeExtras{
			Tagline:          d.Tagline,
			OriginalLanguage: d.OriginalLanguage,
			Country:          d.Country,
			TrailerURL:       d.TrailerURL,
			BackdropURL:      d.BackdropURL,
			CollectionID:     d.CollectionID,
			CollectionName:   d.CollectionName,
			OriginalTitle:    d.OriginalTitle,
			ReleaseDate:      d.ReleaseDate,
		}
		if combined.Credits != nil {
			creditsJSON, err := json.Marshal(combined.Credits)
			if err == nil {
				s := string(creditsJSON)
				extras.CastCrewJSON = &s
			}
		}
		go cacheClient.Contribute(combined.Details, extras)
	}
}

// getCacheClient returns a CacheClient if the cache server is enabled,
// auto-registering if no API key exists yet.
// Returns nil if the cache is disabled or registration fails.
func (s *Scanner) getCacheClient() *metadata.CacheClient {
	if s.settingsRepo == nil {
		return nil
	}
	enabled, _ := s.settingsRepo.Get("cache_server_enabled")
	if enabled == "false" {
		return nil
	}
	return metadata.EnsureRegistered(s.settingsRepo)
}

// applyDirectMatch applies metadata from a direct TMDB lookup (by ID) to a media item.
func (s *Scanner) applyDirectMatch(item *models.MediaItem, match *models.MetadataMatch) {
	// Download poster if available (respecting poster lock)
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
		filename := item.ID.String() + ".jpg"
		_, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err == nil {
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
		}
	}

	if err := s.mediaRepo.UpdateMetadataWithLocks(item.ID, match.Title, match.Year,
		match.Description, match.Rating, posterPath, match.ContentRating, item.LockedFields); err != nil {
		log.Printf("Direct match: DB update failed for %s: %v", item.ID, err)
	}
	if posterPath != nil {
		item.PosterPath = posterPath
	}

	// Apply extended metadata (respecting per-field locks)
	eTagline, eLang, eCountry, eTrailer, eLogo := filterLockedExtended(item.LockedFields,
		match.Tagline, match.OriginalLanguage, match.Country, match.TrailerURL, nil)
	extUpdate := &repository.ExtendedMetadataUpdate{
		Tagline:          eTagline,
		OriginalLanguage: eLang,
		Country:          eCountry,
		TrailerURL:       eTrailer,
		LogoPath:         eLogo,
	}
	if match.OriginalTitle != nil && !isFieldLocked(item.LockedFields, "title") {
		extUpdate.OriginalTitle = match.OriginalTitle
	}
	if match.ReleaseDate != nil && !isFieldLocked(item.LockedFields, "year") {
		extUpdate.ReleaseDate = match.ReleaseDate
	}
	_ = s.mediaRepo.UpdateExtendedMetadataFull(item.ID, extUpdate)

	// Enrich with genres, ratings, etc.
	if match.Source == "tmdb" {
		s.enrichWithDetails(item.ID, match.ExternalID, item.MediaType, item.LockedFields)
	}

	// Store external IDs
	idsJSON := metadata.BuildExternalIDsFromMatch(match.Source, match.ExternalID, match.IMDBId, false)
	if idsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(item.ID, *idsJSON)
	}

	log.Printf("Direct match: %q → %q (ID=%s)", item.Title, match.Title, match.ExternalID)
}

// applyDirectMatchWithCredits applies metadata + credits from a combined TMDB lookup.
func (s *Scanner) applyDirectMatchWithCredits(item *models.MediaItem, combined *metadata.DetailsWithCredits) {
	match := combined.Details
	s.applyDirectMatch(item, match)

	// Also populate cast/crew from the included credits (respecting cast lock)
	if s.performerRepo != nil && combined.Credits != nil && !item.IsFieldLocked("cast") {
		s.enrichWithCredits(item.ID, combined.Credits)
	}

	// Auto-create movie collection from TMDB belongs_to_collection
	if match.CollectionID != nil && match.CollectionName != nil {
		s.autoCreateCollection(item, match)
	}
}

// autoCreateCollection finds or creates a movie_series from TMDB collection data
// and links the media item to it. This auto-populates the movie series tables.
func (s *Scanner) autoCreateCollection(item *models.MediaItem, match *models.MetadataMatch) {
	if s.seriesRepo == nil || match.CollectionID == nil || match.CollectionName == nil {
		return
	}

	collectionIDStr := fmt.Sprintf("%d", *match.CollectionID)

	// First check if a series with this TMDB collection ID already exists
	series, err := s.seriesRepo.FindByExternalID(item.LibraryID, collectionIDStr)
	if err != nil {
		log.Printf("Auto-collection: lookup failed: %v", err)
		return
	}

	if series == nil {
		// Also check by name (user may have manually created it)
		series, err = s.seriesRepo.FindByName(item.LibraryID, *match.CollectionName)
		if err != nil {
			log.Printf("Auto-collection: name lookup failed: %v", err)
			return
		}
	}

	if series == nil {
		// Create a new movie series — enrich from cache server if available
		externalIDs := fmt.Sprintf(`{"tmdb_collection_id":"%s"}`, collectionIDStr)
		series = &models.MovieSeries{
			ID:          uuid.New(),
			LibraryID:   item.LibraryID,
			Name:        *match.CollectionName,
			ExternalIDs: &externalIDs,
		}

		// Try cache server for collection details (poster, description, backdrop)
		if cacheClient := s.getCacheClient(); cacheClient != nil && *match.CollectionID > 0 {
			if coll, collErr := cacheClient.GetCollection(*match.CollectionID); collErr == nil && coll != nil {
				if coll.Description != nil { series.Description = coll.Description }
				// Download poster if available
				posterURL := ""
				if coll.PosterURL != nil { posterURL = *coll.PosterURL }
				if posterURL != "" && s.posterDir != "" {
					filename := "series_" + series.ID.String() + ".jpg"
					if _, dlErr := metadata.DownloadPoster(posterURL, filepath.Join(s.posterDir, "posters"), filename); dlErr == nil {
						webPath := "/previews/posters/" + filename
						series.PosterPath = &webPath
					}
				}
				// Download backdrop
				backdropURL := ""
				if coll.BackdropURL != nil { backdropURL = *coll.BackdropURL }
				if backdropURL != "" && s.posterDir != "" {
					filename := "series_bg_" + series.ID.String() + ".jpg"
					if _, dlErr := metadata.DownloadPoster(backdropURL, filepath.Join(s.posterDir, "backdrops"), filename); dlErr == nil {
						webPath := "/previews/backdrops/" + filename
						series.BackdropPath = &webPath
					}
				}
			}
		}

		if err := s.seriesRepo.Create(series); err != nil {
			log.Printf("Auto-collection: create failed for %q: %v", *match.CollectionName, err)
			return
		}
		log.Printf("Auto-collection: created %q (TMDB collection %s)", *match.CollectionName, collectionIDStr)
	}

	// Check if item is already in this (or any) series
	if s.seriesRepo.IsItemInSeries(item.ID) {
		return
	}

	// Link the item to the series
	seriesItem := &models.MovieSeriesItem{
		ID:          uuid.New(),
		SeriesID:    series.ID,
		MediaItemID: item.ID,
		SortOrder:   0, // will be sorted by year later
	}
	if item.Year != nil {
		seriesItem.SortOrder = *item.Year
	}
	if err := s.seriesRepo.AddItem(seriesItem); err != nil {
		log.Printf("Auto-collection: link item failed: %v", err)
		return
	}
	log.Printf("Auto-collection: linked %q to %q", item.Title, series.Name)
}

// applyNFOData populates a MediaItem with data from a parsed NFO file.
func (s *Scanner) applyNFOData(item *models.MediaItem, nfo *metadata.NFOData) {
	if nfo.Title != "" {
		item.Title = nfo.Title
	}
	if nfo.Plot != "" {
		item.Description = &nfo.Plot
	}
	if nfo.Tagline != "" {
		item.Tagline = &nfo.Tagline
	}
	if nfo.Year > 0 {
		item.Year = &nfo.Year
	}
	if nfo.MPAA != "" {
		item.ContentRating = &nfo.MPAA
	}
	if nfo.Country != "" {
		item.Country = &nfo.Country
	}
	if nfo.TrailerURL != "" {
		item.TrailerURL = &nfo.TrailerURL
	}
	if nfo.OriginalTitle != "" {
		item.OriginalTitle = &nfo.OriginalTitle
	}
	if nfo.SortTitle != "" {
		item.SortTitle = &nfo.SortTitle
	}
	// Apply rating from NFO
	if r := nfo.GetDefaultRating(); r != nil {
		item.Rating = r
	}
	// Apply technical metadata from NFO
	if nfo.Source != "" {
		item.SourceType = &nfo.Source
	}
	if nfo.HDRFormat != "" {
		item.HDRFormat = &nfo.HDRFormat
	}
	if nfo.DynamicRange != "" {
		item.DynamicRange = nfo.DynamicRange
	}
	if nfo.CustomNotes != "" {
		item.CustomNotes = &nfo.CustomNotes
	}
}

// autoPopulateMetadata searches external sources and applies the best match.
// When the cache server is enabled, it is tried first; direct TMDB is the fallback.
// If parsed contains inline provider IDs (TMDB, IMDB), does a direct lookup instead.
func (s *Scanner) autoPopulateMetadata(library *models.Library, item *models.MediaItem, parsed ...ParsedFilename) {
	if len(s.scrapers) == 0 || !metadata.ShouldAutoMatch(item.MediaType) {
		return
	}

	// Skip items where all fields are locked
	if item.MetadataLocked || item.IsFieldLocked("*") {
		log.Printf("Auto-match: skipping %s (metadata locked)", item.ID)
		return
	}

	// For TV shows with season grouping, match at the show level (not per-episode)
	if item.MediaType == models.MediaTypeTVShows && item.TVShowID != nil {
		s.autoMatchTVShow(*item.TVShowID)
		return
	}
	// TV shows without season grouping fall through to per-item matching below

	// Build search query from cleaned title
	searchQuery := metadata.CleanTitleForSearch(item.Title)
	if searchQuery == "" {
		return
	}

	// ── Direct TMDB lookup if we have an ID from NFO or inline filename ──
	// This bypasses fuzzy search entirely for guaranteed accuracy
	var pf ParsedFilename
	if len(parsed) > 0 {
		pf = parsed[0]
	}
	if pf.TMDBID != "" {
		log.Printf("Auto-match: direct TMDB lookup for ID %s (%q)", pf.TMDBID, item.Title)
		for _, sc := range s.scrapers {
			if t, ok := sc.(*metadata.TMDBScraper); ok {
				if item.MediaType == models.MediaTypeTVShows {
					details, err := t.GetTVDetails(pf.TMDBID)
					if err == nil && details != nil {
						s.applyDirectMatch(item, details)
						return
					}
				} else {
					details, err := t.GetDetailsWithCredits(pf.TMDBID)
					if err == nil && details != nil {
						s.applyDirectMatchWithCredits(item, details)
						return
					}
				}
				break
			}
		}
	}

	// ── Direct Audnexus ASIN lookup for audiobooks ──
	if pf.ASIN != "" && item.MediaType == models.MediaTypeAudiobooks {
		log.Printf("Auto-match: direct Audnexus lookup for ASIN %s (%q)", pf.ASIN, item.Title)
		for _, sc := range s.scrapers {
			if a, ok := sc.(*metadata.AudnexusScraper); ok {
				match, err := a.LookupByASIN(pf.ASIN)
				if err == nil && match != nil {
					s.applyAudiobookMatch(item, match)
					return
				}
				break
			}
		}
	}

	// ── Cache server: defer to batch phase for better throughput ──
	cacheClient := s.getCacheClient()
	if cacheClient != nil {
		s.mu.Lock()
		s.pendingMeta = append(s.pendingMeta, pendingMetaItem{
			Item:   item,
			Query:  searchQuery,
			Parsed: pf,
		})
		s.mu.Unlock()
		return
	}

	autoCfg := metadata.AutoMatchConfig(s.settingsRepo)
	match := metadata.FindBestMatch(s.scrapers, searchQuery, item.MediaType, item.Year)
	if match == nil || match.Confidence < autoCfg.MinConfidence {
		if match != nil {
			log.Printf("Auto-match: %q → %q rejected (confidence=%.2f < threshold=%.2f)",
				searchQuery, match.Title, match.Confidence, autoCfg.MinConfidence)
		} else {
			log.Printf("Auto-match: no match for %q", searchQuery)
		}
		return
	}

	log.Printf("Auto-match: %q → %q (source=%s, confidence=%.2f)",
		item.Title, match.Title, match.Source, match.Confidence)

	// Download poster if available (respecting poster lock)
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
		filename := item.ID.String() + ".jpg"
		saved, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err != nil {
			log.Printf("Auto-match: poster download failed for %s: %v", item.ID, err)
		} else {
			// Store as web-accessible path relative to preview root
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
			_ = saved
		}
	}

	// Update the media item with matched metadata (respecting per-field locks)
	if err := s.mediaRepo.UpdateMetadataWithLocks(item.ID, match.Title, match.Year,
		match.Description, match.Rating, posterPath, match.ContentRating, item.LockedFields); err != nil {
		log.Printf("Auto-match: DB update failed for %s: %v", item.ID, err)
	}

	// Sync in-memory poster so screenshot fallback doesn't overwrite the TMDB poster
	if posterPath != nil {
		item.PosterPath = posterPath
	}

	// Get TMDB details for genres, IMDB ID, OMDb ratings, and cast
	if match.Source == "tmdb" {
		s.enrichWithDetails(item.ID, match.ExternalID, item.MediaType, item.LockedFields)
	}

	// For MusicBrainz/OpenLibrary, enrich with full details
	if match.Source == "musicbrainz" || match.Source == "openlibrary" {
		s.enrichNonTMDBDetails(item.ID, match, item.LockedFields)
	}

	// Create artist/album from MusicBrainz match when not already linked
	if match.Source == "musicbrainz" && match.ArtistName != "" {
		s.linkMusicHierarchyFromMatch(item, match.ArtistName, match.ArtistMBID, match.AlbumTitle, match.Year)
	}

	// Propagate cover art to the parent album
	if item.MediaType == models.MediaTypeMusic && item.AlbumID != nil && match.PosterURL != nil {
		s.propagateAlbumArt(item, match.PosterURL)
	}

	// Store external IDs from direct match
	idsJSON := metadata.BuildExternalIDsFromMatch(match.Source, match.ExternalID, match.IMDBId, false)
	if idsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(item.ID, *idsJSON)
	}

	// Contribute to cache server in background (all sources)
	if cacheClient != nil {
		go cacheClient.Contribute(match)
	}
}

// applyAudiobookMatch applies an Audnexus match to an audiobook media item.
func (s *Scanner) applyAudiobookMatch(item *models.MediaItem, match *models.MetadataMatch) {
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
		filename := item.ID.String() + ".jpg"
		saved, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err != nil {
			log.Printf("Audnexus: poster download failed for %s: %v", item.ID, err)
		} else {
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
			_ = saved
		}
	}

	if err := s.mediaRepo.UpdateMetadataWithLocks(item.ID, match.Title, match.Year,
		match.Description, match.Rating, posterPath, match.ContentRating, item.LockedFields); err != nil {
		log.Printf("Audnexus: DB update failed for %s: %v", item.ID, err)
	}

	if posterPath != nil {
		item.PosterPath = posterPath
	}

	// Store genres
	if s.tagRepo != nil && len(match.Genres) > 0 && !item.IsFieldLocked("genres") {
		s.linkGenreTags(item.ID, match.Genres)
	}

	// Store external IDs
	idsJSON := metadata.BuildExternalIDsFromMatch("audnexus", match.ExternalID, "", false)
	if idsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(item.ID, *idsJSON)
	}

	log.Printf("Audnexus: matched %q → %q (ASIN=%s, narrator=%s, runtime=%dm)",
		item.Title, match.Title, match.ASIN, match.Narrator, match.RuntimeMins)
}

// applyCacheResult uses a cache server hit to populate metadata, genres, and ratings
// without making any direct TMDB/OMDb API calls.
func (s *Scanner) applyCacheResult(item *models.MediaItem, result *metadata.CacheLookupResult) {
	match := result.Match

	// Download poster if available (respecting poster lock)
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
		filename := item.ID.String() + ".jpg"
		_, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err != nil {
			log.Printf("Auto-match: poster download failed for %s: %v", item.ID, err)
		} else {
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
		}
	}

	// Update metadata (respecting per-field locks)
	if err := s.mediaRepo.UpdateMetadataWithLocks(item.ID, match.Title, match.Year,
		match.Description, match.Rating, posterPath, match.ContentRating, item.LockedFields); err != nil {
		log.Printf("Auto-match: DB update failed for %s: %v", item.ID, err)
	}

	// Sync in-memory poster so screenshot fallback doesn't overwrite the TMDB poster
	if posterPath != nil {
		item.PosterPath = posterPath
	}

	// Link genre tags from cache (respecting genres lock)
	if s.tagRepo != nil && len(result.Genres) > 0 && !item.IsFieldLocked("genres") {
		s.linkGenreTags(item.ID, result.Genres)
	}

	// Link mood tags and store keywords from cache
	if len(result.Keywords) > 0 {
		s.linkMoodTags(item.ID, result.Keywords)
		s.storeKeywords(item.ID, result.Keywords)
	}

	// Apply OMDb ratings from cache (respecting per-field locks)
	if result.Ratings != nil {
		if err := s.mediaRepo.UpdateRatingsWithLocks(item.ID, result.Ratings.IMDBRating, result.Ratings.RTScore, result.Ratings.AudienceScore, item.LockedFields); err != nil {
			log.Printf("Auto-match: ratings update failed for %s: %v", item.ID, err)
		}
	}

	// Use cast/crew from cache if available (respecting cast lock)
	if s.performerRepo != nil && !item.IsFieldLocked("cast") {
		if result.CastCrewJSON != nil && *result.CastCrewJSON != "" {
			credits := parseCacheCredits(*result.CastCrewJSON)
			if credits != nil {
				s.enrichWithCredits(item.ID, credits)
			}
		} else if match.ExternalID != "" {
			var tmdbScraper *metadata.TMDBScraper
			for _, sc := range s.scrapers {
				if t, ok := sc.(*metadata.TMDBScraper); ok {
					tmdbScraper = t
					break
				}
			}
			if tmdbScraper != nil {
				var credits *metadata.TMDBCredits
				var err error
				if item.MediaType == models.MediaTypeTVShows {
					credits, err = tmdbScraper.GetTVCredits(match.ExternalID)
				} else {
					credits, err = tmdbScraper.GetMovieCredits(match.ExternalID)
				}
				if err != nil {
					log.Printf("Auto-match: TMDB credits failed for %s: %v", match.ExternalID, err)
				} else {
					s.enrichWithCredits(item.ID, credits)
				}
			}
		}
	}

	// Store external IDs from cache
	if result.ExternalIDsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(item.ID, *result.ExternalIDsJSON)
	}

	// Apply extended metadata from cache (respecting per-field locks)
	eTagline, eLang, eCountry, eTrailer, eLogo := filterLockedExtended(item.LockedFields,
		match.Tagline, match.OriginalLanguage, match.Country, match.TrailerURL, result.LogoURL)
	extUpdate := &repository.ExtendedMetadataUpdate{
		Tagline:          eTagline,
		OriginalLanguage: eLang,
		Country:          eCountry,
		TrailerURL:       eTrailer,
		LogoPath:         eLogo,
	}

	// Apply original_title, sort_title, release_date from cache (respecting locks)
	if result.OriginalTitle != nil && !isFieldLocked(item.LockedFields, "title") {
		extUpdate.OriginalTitle = result.OriginalTitle
	}
	if result.SortTitle != nil && !isFieldLocked(item.LockedFields, "title") {
		extUpdate.SortTitle = result.SortTitle
	}
	if result.ReleaseDate != nil && !isFieldLocked(item.LockedFields, "year") {
		extUpdate.ReleaseDate = result.ReleaseDate
	}

	_ = s.mediaRepo.UpdateExtendedMetadataFull(item.ID, extUpdate)

	// Auto-create movie collection from cache data
	if match.CollectionID != nil && match.CollectionName != nil {
		s.autoCreateCollection(item, match)
	}

	// Store all unified metadata fields from cache
	s.storeUnifiedCacheFields(item.ID, result)

	// Create artist/album records from MusicBrainz data when not already linked
	if match.ArtistName != "" {
		s.linkMusicHierarchyFromMatch(item, match.ArtistName, match.ArtistMBID, match.AlbumTitle, match.Year)
	}

	// Propagate cover art to the parent album
	if item.MediaType == models.MediaTypeMusic && item.AlbumID != nil && match.PosterURL != nil {
		s.propagateAlbumArt(item, match.PosterURL)
	}
}

// storeUnifiedCacheFields persists new metadata fields from the unified cache response.
func (s *Scanner) storeUnifiedCacheFields(itemID uuid.UUID, result *metadata.CacheLookupResult) {
	if result.MetacriticScore != nil {
		_ = s.mediaRepo.UpdateMetacriticScore(itemID, *result.MetacriticScore)
	}
	if result.ContentRatingsJSON != nil {
		_ = s.mediaRepo.UpdateContentRatingsJSON(itemID, *result.ContentRatingsJSON)
	}
	if result.ContentRatingsAll != nil {
		resolved := metadata.ResolveContentRating(*result.ContentRatingsAll, "US")
		if resolved != "" {
			_ = s.mediaRepo.UpdateContentRating(itemID, resolved)
		}
	}
	if result.TaglinesJSON != nil {
		_ = s.mediaRepo.UpdateField(itemID, "taglines_json", *result.TaglinesJSON)
	}
	if result.TrailersJSON != nil {
		_ = s.mediaRepo.UpdateField(itemID, "trailers_json", *result.TrailersJSON)
	}
	if result.DescriptionsJSON != nil {
		_ = s.mediaRepo.UpdateField(itemID, "descriptions_json", *result.DescriptionsJSON)
	}

	if !result.EditionsDiscovered {
		_ = s.mediaRepo.SetEditionsPending(itemID, true)
	} else {
		_ = s.mediaRepo.SetEditionsPending(itemID, false)
	}
}

// parseCacheCredits delegates to metadata.ParseCacheCredits.
func parseCacheCredits(castCrewJSON string) *metadata.TMDBCredits {
	return metadata.ParseCacheCredits(castCrewJSON)
}

// isFieldLocked checks if a specific field name is present in the locked_fields array.
func isFieldLocked(lf pq.StringArray, field string) bool {
	for _, f := range lf {
		if f == "*" || f == field {
			return true
		}
	}
	return false
}

// filterLockedExtended nils out extended metadata values for fields that are locked on the item.
func filterLockedExtended(lf pq.StringArray, tagline, origLang, country, trailerURL, logoPath *string) (*string, *string, *string, *string, *string) {
	if len(lf) == 0 {
		return tagline, origLang, country, trailerURL, logoPath
	}
	check := func(field string) bool {
		for _, f := range lf {
			if f == "*" || f == field {
				return true
			}
		}
		return false
	}
	if check("tagline") {
		tagline = nil
	}
	if check("original_language") {
		origLang = nil
	}
	if check("country") {
		country = nil
	}
	if check("trailer_url") {
		trailerURL = nil
	}
	if check("logo_path") {
		logoPath = nil
	}
	return tagline, origLang, country, trailerURL, logoPath
}

// enrichNonTMDBDetails fetches full details from MusicBrainz or OpenLibrary
// and applies genres to the media item.
func (s *Scanner) enrichNonTMDBDetails(itemID uuid.UUID, match *models.MetadataMatch, lockedFields pq.StringArray) {
	isLocked := func(field string) bool {
		for _, f := range lockedFields {
			if f == "*" || f == field {
				return true
			}
		}
		return false
	}

	var scraper metadata.Scraper
	for _, sc := range s.scrapers {
		if sc.Name() == match.Source {
			scraper = sc
			break
		}
	}
	if scraper == nil {
		return
	}

	details, err := scraper.GetDetails(match.ExternalID)
	if err != nil {
		log.Printf("Auto-match: %s details failed for %s: %v", match.Source, match.ExternalID, err)
		return
	}

	// Apply genres from detailed metadata (respecting genres lock)
	if s.tagRepo != nil && len(details.Genres) > 0 && !isLocked("genres") {
		s.linkGenreTags(itemID, details.Genres)
	}

	// Link mood tags and store keywords from TMDB details
	if len(details.Keywords) > 0 {
		s.linkMoodTags(itemID, details.Keywords)
		s.storeKeywords(itemID, details.Keywords)
	}

	// Update description if we got a better one from details (respecting per-field locks)
	if details.Description != nil && *details.Description != "" {
		_ = s.mediaRepo.UpdateMetadataWithLocks(itemID, details.Title, details.Year,
			details.Description, details.Rating, nil, details.ContentRating, lockedFields)
	}

	// Update poster if details have one and we don't yet (respecting poster lock)
	if details.PosterURL != nil && s.posterDir != "" && !isLocked("poster_path") {
		filename := itemID.String() + ".jpg"
		_, dlErr := metadata.DownloadPoster(*details.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if dlErr == nil {
			webPath := "/previews/posters/" + filename
			_ = s.mediaRepo.UpdatePosterPath(itemID, webPath)
		}
	}
}

// EnrichMatchedItem is the exported entry-point for extended metadata enrichment.
// It fetches TMDB details (content rating, tagline, language, country, trailer),
// TMDB credits, OMDb ratings, and fanart.tv artwork — all with per-field lock awareness.
// Used by the metadata refresh handler after a base match is applied.
func (s *Scanner) EnrichMatchedItem(itemID uuid.UUID, tmdbExternalID string, mediaType models.MediaType, lockedFields pq.StringArray) {
	s.enrichWithDetails(itemID, tmdbExternalID, mediaType, lockedFields)
}

// enrichWithDetails fetches TMDB details, creates genre tags, fetches OMDb ratings, and populates cast.
// lockedFields is passed through to respect per-field metadata locks.
func (s *Scanner) enrichWithDetails(itemID uuid.UUID, tmdbExternalID string, mediaType models.MediaType, lockedFields pq.StringArray) {
	isLocked := func(field string) bool {
		for _, f := range lockedFields {
			if f == "*" || f == field {
				return true
			}
		}
		return false
	}
	_ = isLocked // ensure used
	// Find the TMDB scraper
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

	// Fetch details (movie vs TV have different endpoints)
	var details *models.MetadataMatch
	var err error
	if mediaType == models.MediaTypeTVShows {
		details, err = tmdbScraper.GetTVDetails(tmdbExternalID)
	} else {
		details, err = tmdbScraper.GetDetails(tmdbExternalID)
	}
	if err != nil {
		log.Printf("Auto-match: TMDB details failed for %s: %v", tmdbExternalID, err)
		return
	}

	// Update content rating if available (respecting content_rating lock)
	if details.ContentRating != nil && !isLocked("content_rating") {
		_ = s.mediaRepo.UpdateContentRating(itemID, *details.ContentRating)
	}

	// Apply extended metadata from TMDB details (respecting per-field locks)
	eTagline, eLang, eCountry, eTrailer, eLogo := filterLockedExtended(lockedFields,
		details.Tagline, details.OriginalLanguage, details.Country, details.TrailerURL, nil)
	detExtUpdate := &repository.ExtendedMetadataUpdate{
		Tagline:          eTagline,
		OriginalLanguage: eLang,
		Country:          eCountry,
		TrailerURL:       eTrailer,
		LogoPath:         eLogo,
	}
	if details.OriginalTitle != nil && !isLocked("title") {
		detExtUpdate.OriginalTitle = details.OriginalTitle
	}
	if details.ReleaseDate != nil && !isLocked("year") {
		detExtUpdate.ReleaseDate = details.ReleaseDate
	}
	_ = s.mediaRepo.UpdateExtendedMetadataFull(itemID, detExtUpdate)

	// Create/link genre tags (respecting genres lock)
	if s.tagRepo != nil && len(details.Genres) > 0 && !isLocked("genres") {
		s.linkGenreTags(itemID, details.Genres)
	}

	// Link mood tags and store keywords
	if len(details.Keywords) > 0 {
		s.linkMoodTags(itemID, details.Keywords)
		s.storeKeywords(itemID, details.Keywords)
	}

	// Fetch OMDb ratings if key is configured (respecting per-field locks)
	if s.settingsRepo != nil && details.IMDBId != "" {
		omdbKey, err := s.settingsRepo.Get("omdb_api_key")
		if err != nil {
			log.Printf("Auto-match: settings lookup failed: %v", err)
		} else if omdbKey != "" {
			ratings, err := metadata.FetchOMDbRatings(details.IMDBId, omdbKey)
			if err != nil {
				log.Printf("Auto-match: OMDb fetch failed for %s: %v", details.IMDBId, err)
			} else {
				if err := s.mediaRepo.UpdateRatingsWithLocks(itemID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore, lockedFields); err != nil {
					log.Printf("Auto-match: ratings update failed for %s: %v", itemID, err)
				}
			}
		}
	}

	// Fetch and populate cast/crew from TMDB credits (respecting cast lock)
	if s.performerRepo != nil && !isLocked("cast") {
		var credits *metadata.TMDBCredits
		if mediaType == models.MediaTypeTVShows {
			credits, err = tmdbScraper.GetTVCredits(tmdbExternalID)
		} else {
			credits, err = tmdbScraper.GetMovieCredits(tmdbExternalID)
		}
		if err != nil {
			log.Printf("Auto-match: TMDB credits failed for %s: %v", tmdbExternalID, err)
		} else {
			s.enrichWithCredits(itemID, credits)
		}
	}

	// Fetch extended artwork from fanart.tv (logos, banners, clearart)
	s.enrichWithFanart(itemID, tmdbExternalID, mediaType, lockedFields)
}

// enrichWithCredits creates or finds performers from TMDB credits and links them to a media item.
// Imports top 20 cast members and key crew (Director, Producer, Writer).
func (s *Scanner) enrichWithCredits(itemID uuid.UUID, credits *metadata.TMDBCredits) {
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
			log.Printf("Auto-match: create performer %q failed: %v", member.Name, err)
			continue
		}
		charName := member.Character
		if err := s.performerRepo.LinkMedia(itemID, performer.ID, "actor", charName, member.Order); err != nil {
			log.Printf("Auto-match: link performer %q to %s failed: %v", member.Name, itemID, err)
		}
	}

	// Import key crew: Director, Producer, Writer
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
			log.Printf("Auto-match: create crew %q failed: %v", member.Name, err)
			continue
		}
		role := strings.ToLower(member.Job)
		if err := s.performerRepo.LinkMedia(itemID, performer.ID, role, "", 100+importedCrew); err != nil {
			log.Printf("Auto-match: link crew %q to %s failed: %v", member.Name, itemID, err)
		}
		importedCrew++
	}
}

// findOrCreatePerformer looks up an existing performer by name or creates a new one.
// If the performer exists but has no photo, and a profilePath is available, downloads the photo.
// Thread-safe: uses mutex to prevent duplicate creation from concurrent workers.
func (s *Scanner) findOrCreatePerformer(name string, perfType models.PerformerType, profilePath string, tmdbPersonIDs ...int) (*models.Performer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.performerRepo.FindByName(name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// Download photo if the performer doesn't have one yet
		if existing.PhotoPath == nil && profilePath != "" && s.posterDir != "" {
			photoURL := "https://image.tmdb.org/t/p/w185" + profilePath
			filename := "performer_" + existing.ID.String() + ".jpg"
			if _, dlErr := metadata.DownloadPoster(photoURL, filepath.Join(s.posterDir, "posters"), filename); dlErr == nil {
				webPath := "/previews/posters/" + filename
				existing.PhotoPath = &webPath
				_ = s.performerRepo.Update(existing)
			}
		}
		return existing, nil
	}

	// Create new performer
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

	// Download profile photo from TMDB
	if profilePath != "" && s.posterDir != "" {
		photoURL := "https://image.tmdb.org/t/p/w185" + profilePath
		filename := "performer_" + p.ID.String() + ".jpg"
		if _, dlErr := metadata.DownloadPoster(photoURL, filepath.Join(s.posterDir, "posters"), filename); dlErr == nil {
			webPath := "/previews/posters/" + filename
			p.PhotoPath = &webPath
		}
	}

	if err := s.performerRepo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

// enrichWithFanart fetches extended artwork from fanart.tv and applies logos, banners, etc.
// lockedFields is checked to skip updates to locked artwork fields.
func (s *Scanner) enrichWithFanart(itemID uuid.UUID, tmdbExternalID string, mediaType models.MediaType, lockedFields pq.StringArray) {
	if s.settingsRepo == nil {
		return
	}
	fanartKey, _ := s.settingsRepo.Get("fanart_api_key")
	if fanartKey == "" {
		return
	}

	isLocked := func(field string) bool {
		for _, f := range lockedFields {
			if f == "*" || f == field {
				return true
			}
		}
		return false
	}

	client := metadata.NewFanartTVClient(fanartKey)
	var art *metadata.FanartArtwork
	var err error

	if mediaType == models.MediaTypeTVShows {
		art, err = client.GetTVArtwork(tmdbExternalID)
	} else {
		art, err = client.GetMovieArtwork(tmdbExternalID)
	}

	if err != nil {
		log.Printf("fanart.tv: fetch failed for %s: %v", tmdbExternalID, err)
		return
	}
	if art == nil {
		return
	}

	// Download and save logo (respecting logo_path lock)
	if art.LogoURL != "" && s.posterDir != "" && !isLocked("logo_path") {
		filename := "logo_" + itemID.String() + ".png"
		_, dlErr := metadata.DownloadPoster(art.LogoURL, filepath.Join(s.posterDir, "posters"), filename)
		if dlErr == nil {
			webPath := "/previews/posters/" + filename
			s.mediaRepo.DB().Exec(`UPDATE media_items SET logo_path = $1 WHERE id = $2`, webPath, itemID)
			log.Printf("fanart.tv: saved logo for %s", itemID)
		}
	}

	// Download backdrop if we don't have one yet (respecting backdrop_path lock)
	if art.BackdropURL != "" && s.posterDir != "" && !isLocked("backdrop_path") {
		filename := "backdrop_" + itemID.String() + ".jpg"
		_, dlErr := metadata.DownloadPoster(art.BackdropURL, filepath.Join(s.posterDir, "posters"), filename)
		if dlErr == nil {
			webPath := "/previews/posters/" + filename
			s.mediaRepo.DB().Exec(`UPDATE media_items SET backdrop_path = COALESCE(backdrop_path, $1) WHERE id = $2`, webPath, itemID)
		}
	}
}

// linkGenreTags creates genre tags (if they don't exist) and links them to the media item.
// Uses an in-memory genre cache to avoid per-track DB queries.
func (s *Scanner) linkGenreTags(itemID uuid.UUID, genres []string) {
	for _, genre := range genres {
		slug := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(genre, " ", "-"), "'", ""), "\"", ""))
		nameLower := strings.ToLower(genre)

		// Fast path: check cache with read lock
		s.mu.RLock()
		tagID, cached := s.genreCache[nameLower]
		if !cached {
			tagID, cached = s.genreCache[slug]
		}
		s.mu.RUnlock()

		if !cached {
			// Slow path: DB lookup, then cache
			s.mu.Lock()
			// Double-check after acquiring write lock
			tagID, cached = s.genreCache[nameLower]
			if !cached {
				tagID, cached = s.genreCache[slug]
			}
			if !cached {
				// Try DB lookup by slug
				var dbID uuid.UUID
				err := s.mediaRepo.DB().QueryRow(
					`SELECT id FROM tags WHERE slug = $1 AND category = 'genre' LIMIT 1`, slug,
				).Scan(&dbID)
				if err == nil {
					tagID = dbID
					cached = true
				} else {
					// Create new tag
					tagID = uuid.New()
					tag := &models.Tag{
						ID:       tagID,
						Name:     genre,
						Slug:     slug,
						Category: models.TagCategoryGenre,
					}
					if createErr := s.tagRepo.Create(tag); createErr != nil {
						// Slug collision race — re-query
						err2 := s.mediaRepo.DB().QueryRow(
							`SELECT id FROM tags WHERE slug = $1 AND category = 'genre' LIMIT 1`, slug,
						).Scan(&dbID)
						if err2 != nil {
							s.mu.Unlock()
							log.Printf("Auto-match: genre tag %q unresolvable: %v", genre, createErr)
							continue
						}
						tagID = dbID
					}
					cached = true
				}
				s.genreCache[nameLower] = tagID
				s.genreCache[slug] = tagID
			}
			s.mu.Unlock()
		}

		if err := s.tagRepo.AssignToMedia(itemID, tagID); err != nil {
			log.Printf("Auto-match: assign genre tag %q to %s failed: %v", genre, itemID, err)
		}
	}
}

// tmdbKeywordToMood maps TMDB keywords (lowercase) to mood tag names.
// These are the most common TMDB keywords mapped to user-friendly mood categories.
var tmdbKeywordToMood = map[string]string{
	// Feel-Good / Uplifting
	"feel-good":          "Feel-Good",
	"heartwarming":       "Feel-Good",
	"uplifting":          "Feel-Good",
	"inspirational":      "Feel-Good",
	"friendship":         "Feel-Good",
	"underdog":           "Feel-Good",
	"coming of age":      "Feel-Good",
	"road trip":          "Feel-Good",
	"buddy":              "Feel-Good",
	// Dark / Gritty
	"dark":               "Dark",
	"gritty":             "Dark",
	"dystopia":           "Dark",
	"neo-noir":           "Dark",
	"nihilism":           "Dark",
	"bleak":              "Dark",
	"post-apocalyptic":   "Dark",
	"apocalypse":         "Dark",
	// Intense / Suspenseful
	"suspense":           "Intense",
	"tension":            "Intense",
	"psychological":      "Intense",
	"thriller":           "Intense",
	"conspiracy":         "Intense",
	"paranoia":           "Intense",
	"chase":              "Intense",
	"hostage":            "Intense",
	"cat and mouse":      "Intense",
	"survival":           "Intense",
	// Romantic
	"romance":            "Romantic",
	"love":               "Romantic",
	"love triangle":      "Romantic",
	"forbidden love":     "Romantic",
	"wedding":            "Romantic",
	"first love":         "Romantic",
	"soulmates":          "Romantic",
	// Funny / Light-hearted
	"comedy":             "Funny",
	"slapstick":          "Funny",
	"satire":             "Funny",
	"parody":             "Funny",
	"absurd":             "Funny",
	"dark comedy":        "Funny",
	"farce":              "Funny",
	"quirky":             "Funny",
	"witty":              "Funny",
	// Emotional / Tearjerker
	"tearjerker":         "Emotional",
	"tragedy":            "Emotional",
	"grief":              "Emotional",
	"loss":               "Emotional",
	"death":              "Emotional",
	"dying":              "Emotional",
	"terminal illness":   "Emotional",
	"loss of loved one":  "Emotional",
	// Mind-bending
	"mind-bending":       "Mind-Bending",
	"twist ending":       "Mind-Bending",
	"time travel":        "Mind-Bending",
	"time loop":          "Mind-Bending",
	"alternate reality":  "Mind-Bending",
	"dream":              "Mind-Bending",
	"parallel universe":  "Mind-Bending",
	"nonlinear timeline": "Mind-Bending",
	"surreal":            "Mind-Bending",
	"hallucination":      "Mind-Bending",
	// Scary / Creepy
	"horror":             "Scary",
	"haunted house":      "Scary",
	"ghost":              "Scary",
	"demon":              "Scary",
	"slasher":            "Scary",
	"paranormal":         "Scary",
	"zombie":             "Scary",
	"vampire":            "Scary",
	"werewolf":           "Scary",
	"monster":            "Scary",
	"serial killer":      "Scary",
	"possession":         "Scary",
	"supernatural":       "Scary",
	"occult":             "Scary",
	// Epic / Grand
	"epic":               "Epic",
	"war":                "Epic",
	"battle":             "Epic",
	"medieval":           "Epic",
	"ancient":            "Epic",
	"mythology":          "Epic",
	"sword and sorcery":  "Epic",
	"historical":         "Epic",
	// Adrenaline / Action
	"action":             "Adrenaline",
	"explosion":          "Adrenaline",
	"car chase":          "Adrenaline",
	"martial arts":       "Adrenaline",
	"heist":              "Adrenaline",
	"revenge":            "Adrenaline",
	"gunfight":           "Adrenaline",
	"fight":              "Adrenaline",
}

// linkMoodTags maps TMDB keywords to mood categories and links them as mood tags.
func (s *Scanner) linkMoodTags(itemID uuid.UUID, keywords []string) {
	if s.tagRepo == nil || len(keywords) == 0 {
		return
	}

	// Deduplicate moods from keywords
	moodSet := make(map[string]bool)
	for _, kw := range keywords {
		if mood, ok := tmdbKeywordToMood[strings.ToLower(kw)]; ok {
			moodSet[mood] = true
		}
	}

	for mood := range moodSet {
		// Find or create the mood tag
		existing, _ := s.tagRepo.List("mood")
		var tagID uuid.UUID
		found := false
		for _, t := range existing {
			if strings.EqualFold(t.Name, mood) {
				tagID = t.ID
				found = true
				break
			}
		}
		if !found {
			tagID = uuid.New()
			tag := &models.Tag{
				ID:       tagID,
				Name:     mood,
				Category: models.TagCategoryMood,
			}
			if err := s.tagRepo.Create(tag); err != nil {
				log.Printf("Auto-match: create mood tag %q failed: %v", mood, err)
				continue
			}
		}
		if err := s.tagRepo.AssignToMedia(itemID, tagID); err != nil {
			log.Printf("Auto-match: assign mood tag %q to %s failed: %v", mood, itemID, err)
		}
	}
}

// storeKeywords saves TMDB keywords as a JSON array on the media item.
func (s *Scanner) storeKeywords(itemID uuid.UUID, keywords []string) {
	if len(keywords) == 0 {
		return
	}
	data, err := json.Marshal(keywords)
	if err != nil {
		return
	}
	kwStr := string(data)
	_, _ = s.mediaRepo.DB().Exec(`UPDATE media_items SET keywords = $1 WHERE id = $2`, kwStr, itemID)
}

