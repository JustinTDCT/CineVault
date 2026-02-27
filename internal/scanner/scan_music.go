package scanner

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// cachedFindOrCreateArtist returns an artist from the in-memory cache or DB,
// creating one if it doesn't exist. Holds lock for the entire operation to
// prevent concurrent workers from creating duplicate records.
func (s *Scanner) cachedFindOrCreateArtist(libraryID uuid.UUID, name string) (*models.Artist, error) {
	key := libraryID.String() + "|" + strings.ToLower(name)

	// Fast path: read lock for cache hit
	s.mu.RLock()
	if cached, ok := s.artistCache[key]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// Slow path: write lock for DB lookup/create
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if cached, ok := s.artistCache[key]; ok {
		return cached, nil
	}

	artist, err := s.musicRepo.FindArtistByName(libraryID, name)
	if err != nil {
		return nil, err
	}
	if artist == nil {
		artist = &models.Artist{
			ID:        uuid.New(),
			LibraryID: libraryID,
			Name:      name,
		}
		if err := s.musicRepo.CreateArtist(artist); err != nil {
			return nil, err
		}
		log.Printf("Music hierarchy: created artist %q", name)
	}

	s.artistCache[key] = artist
	return artist, nil
}

// cachedFindOrCreateAlbum returns an album from the in-memory cache or DB,
// creating one if it doesn't exist. Holds lock for the entire operation to
// prevent concurrent workers from creating duplicate records.
func (s *Scanner) cachedFindOrCreateAlbum(artistID, libraryID uuid.UUID, title string, year *int) (*models.Album, error) {
	key := artistID.String() + "|" + strings.ToLower(title)

	// Fast path: read lock for cache hit
	s.mu.RLock()
	if cached, ok := s.albumCache[key]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// Slow path: write lock for DB lookup/create
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if cached, ok := s.albumCache[key]; ok {
		return cached, nil
	}

	album, err := s.musicRepo.FindAlbumByTitle(artistID, title)
	if err != nil {
		return nil, err
	}
	if album == nil {
		album = &models.Album{
			ID:        uuid.New(),
			ArtistID:  artistID,
			LibraryID: libraryID,
			Title:     title,
			Year:      year,
		}
		if err := s.musicRepo.CreateAlbum(album); err != nil {
			return nil, err
		}
		log.Printf("Music hierarchy: created album %q", title)
	}

	s.albumCache[key] = album
	return album, nil
}

// CleanupMusicDuplicates merges duplicate albums (same title, different
// "feat." artist variants) and removes orphaned artist records. Returns
// the number of duplicate albums removed.
func (s *Scanner) CleanupMusicDuplicates(libraryID uuid.UUID) (int, error) {
	if s.musicRepo == nil {
		return 0, nil
	}
	removed, err := s.musicRepo.CleanupDuplicateAlbums(libraryID)
	if err != nil {
		return 0, fmt.Errorf("cleanup duplicate albums: %w", err)
	}
	orphaned, err := s.musicRepo.CleanupOrphanedArtists(libraryID)
	if err != nil {
		log.Printf("Music cleanup: orphaned artist cleanup error: %v", err)
	} else if orphaned > 0 {
		log.Printf("Music cleanup: removed %d orphaned artists", orphaned)
	}
	return removed, nil
}

// BackfillArtistImages finds artists without poster art and fetches images
// from the cache server (which sources them from fanart.tv).
func (s *Scanner) BackfillArtistImages(libraryID uuid.UUID) (int, error) {
	if s.musicRepo == nil || s.posterDir == "" {
		return 0, nil
	}
	artists, err := s.musicRepo.ListArtistsWithoutPosters(libraryID)
	if err != nil {
		return 0, fmt.Errorf("list artists: %w", err)
	}
	if len(artists) == 0 {
		return 0, nil
	}

	cacheClient := s.getCacheClient()
	if cacheClient == nil {
		return 0, fmt.Errorf("cache server not configured")
	}

	log.Printf("Artist image backfill: %d artists without photos in library %s", len(artists), libraryID)

	fetched := 0
	for _, artist := range artists {
		if artist.MBID == nil || *artist.MBID == "" {
			continue
		}
		result := cacheClient.LookupMusicArtist(*artist.MBID, artist.Name)
		if result == nil {
			continue
		}

		// Prefer cached local path, fall back to source URL
		var imageURL string
		if result.PhotoPath != nil && *result.PhotoPath != "" {
			imageURL = metadata.CacheImageURL(*result.PhotoPath)
		} else if result.PhotoURL != nil && *result.PhotoURL != "" {
			imageURL = *result.PhotoURL
		}
		if imageURL == "" {
			continue
		}

		filename := "artist_" + artist.ID.String() + ".jpg"
		_, dlErr := metadata.DownloadPoster(imageURL, filepath.Join(s.posterDir, "posters"), filename)
		if dlErr != nil {
			log.Printf("Artist image backfill: download failed for %q: %v", artist.Name, dlErr)
			continue
		}
		webPath := "/previews/posters/" + filename
		if err := s.musicRepo.UpdateArtistPosterPath(artist.ID, webPath); err == nil {
			fetched++
			log.Printf("Artist image backfill: set photo for %q", artist.Name)
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fetched, nil
}

// linkMusicHierarchyFromMatch creates/links artist and album records when a
// MusicBrainz or cache match returns artist/album info that wasn't captured
// from the filename or embedded tags.
// propagateAlbumArt copies the track's poster to its parent album if the album has no cover art.
func (s *Scanner) propagateAlbumArt(item *models.MediaItem, posterURL *string) {
	if s.musicRepo == nil || item.AlbumID == nil || posterURL == nil || *posterURL == "" || s.posterDir == "" {
		return
	}
	album, err := s.musicRepo.GetAlbumByID(*item.AlbumID)
	if err != nil || album == nil || album.PosterPath != nil {
		return
	}
	filename := "album_" + album.ID.String() + ".jpg"
	_, dlErr := metadata.DownloadPoster(*posterURL, filepath.Join(s.posterDir, "posters"), filename)
	if dlErr != nil {
		log.Printf("Album art: download failed for %q: %v", album.Title, dlErr)
		return
	}
	webPath := "/previews/posters/" + filename
	if err := s.musicRepo.UpdateAlbumPosterPath(album.ID, webPath); err != nil {
		log.Printf("Album art: DB update failed for %q: %v", album.Title, err)
		return
	}
	log.Printf("Album art: set cover for %q", album.Title)
}

// extractEmbeddedCoverArt extracts embedded cover art from an audio file using ffmpeg
// and saves it as a JPEG file. Returns the web path or empty string if extraction fails.
func (s *Scanner) extractEmbeddedCoverArt(filePath string, itemID uuid.UUID) string {
	if s.ffmpegPath == "" || s.posterDir == "" {
		return ""
	}
	filename := "album_embedded_" + itemID.String() + ".jpg"
	outPath := filepath.Join(s.posterDir, "posters", filename)
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return ""
	}
	cmd := exec.Command(s.ffmpegPath, "-i", filePath, "-an", "-vcodec", "mjpeg",
		"-frames:v", "1", "-y", outPath)
	if err := cmd.Run(); err != nil {
		return ""
	}
	info, err := os.Stat(outPath)
	if err != nil || info.Size() < 1000 {
		os.Remove(outPath)
		return ""
	}
	return "/previews/posters/" + filename
}

// BackfillAlbumArt finds albums without poster art and attempts to fetch covers
// from the Cover Art Archive via MusicBrainz release search.
func (s *Scanner) BackfillAlbumArt(libraryID uuid.UUID) (int, error) {
	if s.musicRepo == nil || s.posterDir == "" {
		return 0, nil
	}
	albums, err := s.musicRepo.ListAlbumsWithoutPosters(libraryID)
	if err != nil {
		return 0, fmt.Errorf("list albums: %w", err)
	}
	if len(albums) == 0 {
		return 0, nil
	}
	log.Printf("Album art backfill: %d albums without covers in library %s", len(albums), libraryID)

	cacheClient := s.getCacheClient()

	var mbScraper *metadata.MusicBrainzScraper
	for _, sc := range s.scrapers {
		if mb, ok := sc.(*metadata.MusicBrainzScraper); ok {
			mbScraper = mb
			break
		}
	}

	fetched := 0
	for _, album := range albums {
		found := false
		cleanTitle := cleanAlbumTitle(album.Title)
		cleanArtist := normalizeArtistForGrouping(album.ArtistName)
		query := cleanTitle
		if cleanArtist != "" {
			query = cleanArtist + " " + cleanTitle
		}

		// 1) Cache server first — try exact lookup, then fuzzy search
		if !found && cacheClient != nil {
			// Try exact alias match with artist+title, then title-only
			for _, q := range []string{query, cleanTitle} {
				result := cacheClient.Lookup(q, album.Year, models.MediaTypeMusic)
				if result != nil && result.Match != nil && result.Match.PosterURL != nil && *result.Match.PosterURL != "" {
					filename := "album_" + album.ID.String() + ".jpg"
					_, dlErr := metadata.DownloadPoster(*result.Match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
					if dlErr == nil {
						webPath := "/previews/posters/" + filename
						if err := s.musicRepo.UpdateAlbumPosterPath(album.ID, webPath); err == nil {
							fetched++
							found = true
							log.Printf("Album art backfill: set cover for %q (cache server)", album.Title)
						}
					}
					break
				}
			}
			// Fuzzy search fallback if exact alias didn't match
			if !found {
				matches := cacheClient.Search(query, models.MediaTypeMusic, album.Year, 0.5, 3)
				for _, m := range matches {
					if m.PosterURL != nil && *m.PosterURL != "" {
						filename := "album_" + album.ID.String() + ".jpg"
						_, dlErr := metadata.DownloadPoster(*m.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
						if dlErr == nil {
							webPath := "/previews/posters/" + filename
							if err := s.musicRepo.UpdateAlbumPosterPath(album.ID, webPath); err == nil {
								fetched++
								found = true
								log.Printf("Album art backfill: set cover for %q (cache search)", album.Title)
							}
						}
						break
					}
				}
			}
		}

		// 2) MusicBrainz direct (rate limited, only for cache misses)
		if !found && mbScraper != nil {
			matches, err := mbScraper.Search(query, models.MediaTypeMusic, album.Year)
			if err == nil {
				for _, m := range matches {
					if m.PosterURL != nil && *m.PosterURL != "" {
						filename := "album_" + album.ID.String() + ".jpg"
						_, dlErr := metadata.DownloadPoster(*m.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
						if dlErr != nil {
							continue
						}
						webPath := "/previews/posters/" + filename
						if err := s.musicRepo.UpdateAlbumPosterPath(album.ID, webPath); err == nil {
							fetched++
							found = true
							log.Printf("Album art backfill: set cover for %q (Cover Art Archive)", album.Title)
						}
						break
					}
				}
			}
			time.Sleep(1100 * time.Millisecond)
		}

		// 3) Fallback: extract embedded cover art from a track file
		if !found {
			tracks, _ := s.musicRepo.ListTracksByAlbum(album.ID)
			for _, track := range tracks {
				probe, probeErr := s.ffprobe.Probe(track.FilePath)
				if probeErr != nil || !probe.HasEmbeddedCoverArt() {
					continue
				}
				webPath := s.extractEmbeddedCoverArt(track.FilePath, album.ID)
				if webPath != "" {
					if err := s.musicRepo.UpdateAlbumPosterPath(album.ID, webPath); err == nil {
						fetched++
						log.Printf("Album art backfill: extracted embedded art for %q", album.Title)
					}
				}
				break
			}
		}
	}
	return fetched, nil
}

// AggregateAlbumMetadata derives album-level metadata (year, genre) from child
// tracks using majority vote, similar to Jellyfin's AlbumMetadataService.
func (s *Scanner) AggregateAlbumMetadata(libraryID uuid.UUID) (int, error) {
	if s.musicRepo == nil {
		return 0, nil
	}
	albums, err := s.musicRepo.ListAlbumsByLibrary(libraryID)
	if err != nil {
		return 0, fmt.Errorf("list albums: %w", err)
	}

	updated := 0
	for _, album := range albums {
		tracks, err := s.musicRepo.ListTracksByAlbum(album.ID)
		if err != nil || len(tracks) == 0 {
			continue
		}

		// Aggregate year by majority vote
		yearCounts := map[int]int{}
		for _, t := range tracks {
			if t.Year != nil && *t.Year > 0 {
				yearCounts[*t.Year]++
			}
		}
		if album.Year == nil && len(yearCounts) > 0 {
			bestYear, bestCount := 0, 0
			for y, c := range yearCounts {
				if c > bestCount {
					bestYear, bestCount = y, c
				}
			}
			if bestYear > 0 {
				s.mediaRepo.DB().Exec(`UPDATE albums SET year = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, bestYear, album.ID)
				updated++
			}
		}

		// Aggregate genre from track tags
		if album.Genre == nil && s.tagRepo != nil {
			genreCounts := map[string]int{}
			for _, t := range tracks {
				tags, _ := s.tagRepo.GetMediaTags(t.ID)
				for _, tag := range tags {
					if tag.Category == "genre" {
						genreCounts[tag.Name]++
					}
				}
			}
			if len(genreCounts) > 0 {
				bestGenre, bestCount := "", 0
				for g, c := range genreCounts {
					if c > bestCount {
						bestGenre, bestCount = g, c
					}
				}
				if bestGenre != "" {
					s.mediaRepo.DB().Exec(`UPDATE albums SET genre = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, bestGenre, album.ID)
				}
			}
		}
	}
	return updated, nil
}

func (s *Scanner) linkMusicHierarchyFromMatch(item *models.MediaItem, artistName, artistMBID, albumTitle string, year *int) {
	if artistName == "" || s.musicRepo == nil {
		return
	}
	if item.MediaType != models.MediaTypeMusic && item.MediaType != models.MediaTypeMusicVideos {
		return
	}
	if item.ArtistID != nil {
		return
	}

	artist, err := s.cachedFindOrCreateArtist(item.LibraryID, artistName)
	if err != nil {
		log.Printf("Music hierarchy (match): artist %q error: %v", artistName, err)
		return
	}
	item.ArtistID = &artist.ID
	s.mediaRepo.DB().Exec(`UPDATE media_items SET artist_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, artist.ID, item.ID)

	// Store MusicBrainz artist ID if available and not yet set
	if artistMBID != "" && artist.MBID == nil {
		_ = s.musicRepo.UpdateArtistMBID(artist.ID, artistMBID)
		artist.MBID = &artistMBID
	}

	if albumTitle != "" {
		album, err := s.cachedFindOrCreateAlbum(artist.ID, item.LibraryID, albumTitle, year)
		if err != nil {
			log.Printf("Music hierarchy (match): album %q error: %v", albumTitle, err)
			return
		}
		item.AlbumID = &album.ID
		s.mediaRepo.DB().Exec(`UPDATE media_items SET album_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, album.ID, item.ID)
	}
}

// RefreshMusicItem updates a music track's metadata and hierarchy linkage.
// Fast path: if tags already exist in DB, derives artist/album from file path
// (zero file I/O). Full path: ffprobes the file when tags are missing.
func (s *Scanner) RefreshMusicItem(item *models.MediaItem) error {
	// Already fully linked — nothing to do
	if item.ArtistID != nil && item.AlbumID != nil && item.Title != "" && item.DurationSeconds != nil {
		return nil
	}

	// Fast path: tags exist in DB, just need hierarchy linkage
	if item.Title != "" && item.DurationSeconds != nil && item.ArtistID == nil {
		return s.refreshMusicFastPath(item)
	}

	// Full path: ffprobe to read tags + technical metadata
	return s.refreshMusicFullPath(item)
}

// refreshMusicFastPath links a track to its artist/album hierarchy.
// Prefers stored album_artist tag over file path parsing for accuracy.
func (s *Scanner) refreshMusicFastPath(item *models.MediaItem) error {
	var artistName, albumName string

	// Prefer embedded album_artist tag (already in DB) over path parsing.
	if item.AlbumArtist != nil && *item.AlbumArtist != "" {
		artistName = normalizeArtistForGrouping(*item.AlbumArtist)
		albumName, _ = parseArtistAlbumFromPath(item.FilePath)
		if albumName == "" {
			albumName = ""
		}
	} else {
		artistName, albumName = parseArtistAlbumFromPath(item.FilePath)
	}
	if artistName == "" {
		return nil
	}

	setClauses := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []interface{}{}
	idx := 1

	parsed := ParsedFilename{Artist: artistName, Album: albumName}
	if s.musicRepo != nil {
		if err := s.handleMusicHierarchy(&models.Library{ID: item.LibraryID}, item, parsed); err != nil {
			log.Printf("Music refresh fast: hierarchy failed for %s: %v", item.FileName, err)
		}
	}

	if item.ArtistID != nil {
		setClauses = append(setClauses, fmt.Sprintf("artist_id = $%d", idx))
		args = append(args, *item.ArtistID)
		idx++
	}
	if item.AlbumID != nil {
		setClauses = append(setClauses, fmt.Sprintf("album_id = $%d", idx))
		args = append(args, *item.AlbumID)
		idx++
	}

	if len(args) == 0 {
		return nil
	}

	args = append(args, item.ID)
	query := fmt.Sprintf("UPDATE media_items SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), idx)
	_, err := s.mediaRepo.DB().Exec(query, args...)
	return err
}

// refreshMusicFullPath does a complete ffprobe to read tags + technical metadata.
func (s *Scanner) refreshMusicFullPath(item *models.MediaItem) error {
	probe, err := s.ffprobe.Probe(item.FilePath)
	if err != nil {
		return fmt.Errorf("ffprobe: %w", err)
	}

	dur := probe.GetDurationSeconds()
	ac := probe.GetAudioCodec()
	af := probe.GetAudioFormat()
	br := probe.GetBitrate()
	ext := strings.TrimPrefix(filepath.Ext(item.FilePath), ".")

	setClauses := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []interface{}{}
	idx := 1

	if dur > 0 {
		setClauses = append(setClauses, fmt.Sprintf("duration_seconds = $%d", idx))
		args = append(args, dur)
		idx++
	}
	if ac != "" {
		setClauses = append(setClauses, fmt.Sprintf("audio_codec = $%d", idx))
		args = append(args, ac)
		idx++
	}
	if af != "" {
		setClauses = append(setClauses, fmt.Sprintf("audio_format = $%d", idx))
		args = append(args, af)
		idx++
	}
	if br > 0 {
		setClauses = append(setClauses, fmt.Sprintf("bitrate = $%d", idx))
		args = append(args, br)
		idx++
	}
	if ext != "" {
		setClauses = append(setClauses, fmt.Sprintf("container = $%d", idx))
		args = append(args, ext)
		idx++
	}

	var tagArtist, tagAlbumArtist, album, genre string
	var trackNum, discNum *int
	if len(probe.Format.Tags) > 0 {
		for k, v := range probe.Format.Tags {
			switch strings.ToLower(k) {
			case "title":
				v = strings.TrimSpace(v)
				if v != "" {
					setClauses = append(setClauses, fmt.Sprintf("title = $%d", idx))
					args = append(args, v)
					idx++
				}
			case "album_artist":
				tagAlbumArtist = strings.TrimSpace(v)
			case "artist":
				tagArtist = strings.TrimSpace(v)
			case "album":
				if album == "" {
					album = strings.TrimSpace(v)
				}
			case "genre":
				genre = strings.TrimSpace(v)
			case "track":
				if trackNum == nil {
					if n, err := strconv.Atoi(strings.Split(strings.TrimSpace(v), "/")[0]); err == nil {
						trackNum = &n
					}
				}
			case "disc", "discnumber", "disc_number":
				if discNum == nil {
					if n, err := strconv.Atoi(strings.Split(strings.TrimSpace(v), "/")[0]); err == nil {
						discNum = &n
					}
				}
			case "date":
				v = strings.TrimSpace(v)
				if len(v) >= 4 {
					if y, err := strconv.Atoi(v[:4]); err == nil && y > 1000 && y < 3000 {
						setClauses = append(setClauses, fmt.Sprintf("year = $%d", idx))
						args = append(args, y)
						idx++
					}
				}
			}
		}
	}

	if trackNum != nil {
		setClauses = append(setClauses, fmt.Sprintf("track_number = $%d", idx))
		args = append(args, *trackNum)
		idx++
	}
	if discNum != nil {
		setClauses = append(setClauses, fmt.Sprintf("disc_number = $%d", idx))
		args = append(args, *discNum)
		idx++
	}

	// Resolve hierarchy — prefer embedded tags, fall back to path parsing
	artist := tagAlbumArtist
	if artist == "" {
		artist = tagArtist
	}
	if artist == "" {
		artist, album = parseArtistAlbumFromPath(item.FilePath)
	}
	if artist != "" && s.musicRepo != nil {
		parsed := ParsedFilename{Artist: artist, Album: album, TrackNumber: trackNum, DiscNumber: discNum}
		if err := s.handleMusicHierarchy(&models.Library{ID: item.LibraryID}, item, parsed); err != nil {
			log.Printf("Music refresh: hierarchy update failed for %s: %v", item.FileName, err)
		}
	}
	if item.ArtistID != nil {
		setClauses = append(setClauses, fmt.Sprintf("artist_id = $%d", idx))
		args = append(args, *item.ArtistID)
		idx++
	}
	if item.AlbumID != nil {
		setClauses = append(setClauses, fmt.Sprintf("album_id = $%d", idx))
		args = append(args, *item.AlbumID)
		idx++
	}
	if tagAlbumArtist != "" {
		setClauses = append(setClauses, fmt.Sprintf("album_artist = $%d", idx))
		args = append(args, tagAlbumArtist)
		idx++
	}

	args = append(args, item.ID)
	query := fmt.Sprintf("UPDATE media_items SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), idx)
	if _, err := s.mediaRepo.DB().Exec(query, args...); err != nil {
		return fmt.Errorf("update media item: %w", err)
	}

	// Link genre tag from embedded metadata
	if genre != "" && s.tagRepo != nil {
		s.linkGenreTags(item.ID, []string{genre})
	}

	return nil
}
