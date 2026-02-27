package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
)

type MusicBrainzScraper struct {
	client  *http.Client
	limiter chan time.Time // token-bucket rate limiter: 1 req/sec
}

func NewMusicBrainzScraper() *MusicBrainzScraper {
	s := &MusicBrainzScraper{
		client:  &http.Client{Timeout: 10 * time.Second},
		limiter: make(chan time.Time, 1),
	}
	s.limiter <- time.Now().Add(-time.Second) // allow immediate first request
	return s
}

// waitRateLimit enforces MusicBrainz's 1 request/second policy.
func (s *MusicBrainzScraper) waitRateLimit() {
	last := <-s.limiter
	elapsed := time.Since(last)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}
	s.limiter <- time.Now()
}

func (s *MusicBrainzScraper) Name() string { return "musicbrainz" }

func (s *MusicBrainzScraper) Search(query string, mediaType models.MediaType, year *int) ([]*models.MetadataMatch, error) {
	// For music, search releases (albums); for music videos, search recordings
	endpoint := "release"
	if mediaType == models.MediaTypeMusicVideos {
		endpoint = "recording"
	}

	reqURL := fmt.Sprintf("https://musicbrainz.org/ws/2/%s/?query=%s&fmt=json&limit=10",
		endpoint, url.QueryEscape(query))

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("User-Agent", "CineVault/1.0 (https://github.com/JustinTDCT/CineVault)")

	s.waitRateLimit()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 503 {
		return nil, fmt.Errorf("MusicBrainz rate limited")
	}

	if endpoint == "recording" {
		var result struct {
			Recordings []struct {
				ID             string `json:"id"`
				Title          string `json:"title"`
				Score          int    `json:"score"`
				Length         int    `json:"length"`
				FirstReleaseDate string `json:"first-release-date"`
				ArtistCredit   []struct {
					Name   string `json:"name"`
					Artist struct {
						Name string `json:"name"`
					} `json:"artist"`
				} `json:"artist-credit"`
				Releases []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
					Date  string `json:"date"`
				} `json:"releases"`
			} `json:"recordings"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		var matches []*models.MetadataMatch
		for _, r := range result.Recordings {
			// Extract year
			var year *int
			dateStr := r.FirstReleaseDate
			if dateStr != "" && len(dateStr) >= 4 {
				y := 0
				fmt.Sscanf(dateStr[:4], "%d", &y)
				if y > 0 {
					year = &y
				}
			}
			// Build description with artist names
			var desc *string
			var artists []string
			for _, ac := range r.ArtistCredit {
				name := ac.Name
				if ac.Artist.Name != "" {
					name = ac.Artist.Name
				}
				artists = append(artists, name)
			}
			if len(artists) > 0 {
				d := "Recording by " + strings.Join(artists, ", ")
				desc = &d
			}

			matches = append(matches, &models.MetadataMatch{
				Source:      "musicbrainz",
				ExternalID:  r.ID,
				Title:       r.Title,
				Year:        year,
				Description: desc,
				Confidence:  float64(r.Score) / 100.0,
			})
		}
		return matches, nil
	}

	// Release search
	var result struct {
		Releases []struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			Score        int    `json:"score"`
			Date         string `json:"date"`
			Country      string `json:"country"`
			ArtistCredit []struct {
				Name   string `json:"name"`
				Artist struct {
					Name string `json:"name"`
				} `json:"artist"`
			} `json:"artist-credit"`
			ReleaseGroup struct {
				PrimaryType string `json:"primary-type"`
			} `json:"release-group"`
			CoverArtArchive struct {
				Front bool `json:"front"`
			} `json:"cover-art-archive"`
		} `json:"releases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var matches []*models.MetadataMatch
	for _, r := range result.Releases {
		var year *int
		if r.Date != "" && len(r.Date) >= 4 {
			y := 0
			fmt.Sscanf(r.Date[:4], "%d", &y)
			if y > 0 {
				year = &y
			}
		}

		var desc *string
		var artists []string
		for _, ac := range r.ArtistCredit {
			name := ac.Name
			if ac.Artist.Name != "" {
				name = ac.Artist.Name
			}
			artists = append(artists, name)
		}
		if len(artists) > 0 {
			releaseType := "Release"
			if r.ReleaseGroup.PrimaryType != "" {
				releaseType = r.ReleaseGroup.PrimaryType
			}
			d := releaseType + " by " + strings.Join(artists, ", ")
			desc = &d
		}

		// Cover art URL (if available)
		var posterURL *string
		if r.CoverArtArchive.Front {
			p := fmt.Sprintf("https://coverartarchive.org/release/%s/front-500", r.ID)
			posterURL = &p
		}

		matches = append(matches, &models.MetadataMatch{
			Source:      "musicbrainz",
			ExternalID:  r.ID,
			Title:       r.Title,
			Year:        year,
			Description: desc,
			PosterURL:   posterURL,
			Confidence:  float64(r.Score) / 100.0,
		})
	}
	return matches, nil
}

// GetDetails fetches detailed metadata for a MusicBrainz recording or release.
func (s *MusicBrainzScraper) GetDetails(externalID string) (*models.MetadataMatch, error) {
	// Try as a release first (more common for music libraries), then recording
	match, err := s.getReleaseDetails(externalID)
	if err != nil || match == nil {
		match, err = s.getRecordingDetails(externalID)
	}
	if match == nil {
		return &models.MetadataMatch{
			Source:     "musicbrainz",
			ExternalID: externalID,
			Confidence: 1.0,
		}, nil
	}
	return match, err
}

func (s *MusicBrainzScraper) getReleaseDetails(releaseID string) (*models.MetadataMatch, error) {
	reqURL := fmt.Sprintf("https://musicbrainz.org/ws/2/release/%s?inc=artists+recordings+tags+release-groups&fmt=json", releaseID)

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("User-Agent", "CineVault/1.0 (https://github.com/JustinTDCT/CineVault)")

	s.waitRateLimit()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("MusicBrainz release details returned %d", resp.StatusCode)
	}

	var r struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Date         string `json:"date"`
		ArtistCredit []struct {
			Name   string `json:"name"`
			Artist struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"artist"`
		} `json:"artist-credit"`
		ReleaseGroup struct {
			Title       string `json:"title"`
			PrimaryType string `json:"primary-type"`
		} `json:"release-group"`
		Tags []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"tags"`
		CoverArtArchive struct {
			Front bool `json:"front"`
		} `json:"cover-art-archive"`
		Media []struct {
			TrackCount int `json:"track-count"`
			Tracks     []struct {
				Title  string `json:"title"`
				Length int    `json:"length"`
			} `json:"tracks"`
		} `json:"media"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	var year *int
	if r.Date != "" && len(r.Date) >= 4 {
		y := 0
		fmt.Sscanf(r.Date[:4], "%d", &y)
		if y > 0 {
			year = &y
		}
	}

	var desc *string
	var artistName, artistMBID string
	var artists []string
	for _, ac := range r.ArtistCredit {
		name := ac.Name
		if ac.Artist.Name != "" {
			name = ac.Artist.Name
		}
		artists = append(artists, name)
		if artistMBID == "" && ac.Artist.ID != "" {
			artistMBID = ac.Artist.ID
		}
	}
	if len(artists) > 0 {
		artistName = strings.Join(artists, ", ")
		releaseType := "Release"
		if r.ReleaseGroup.PrimaryType != "" {
			releaseType = r.ReleaseGroup.PrimaryType
		}
		d := releaseType + " by " + artistName
		desc = &d
	}

	var genres []string
	for _, t := range r.Tags {
		genres = append(genres, t.Name)
	}

	var posterURL *string
	if r.CoverArtArchive.Front {
		p := fmt.Sprintf("https://coverartarchive.org/release/%s/front-500", r.ID)
		posterURL = &p
	}

	return &models.MetadataMatch{
		Source:      "musicbrainz",
		ExternalID:  r.ID,
		Title:       r.Title,
		Year:        year,
		Description: desc,
		PosterURL:   posterURL,
		Genres:      genres,
		Confidence:  1.0,
		ArtistName:  artistName,
		ArtistMBID:  artistMBID,
	}, nil
}

func (s *MusicBrainzScraper) getRecordingDetails(recordingID string) (*models.MetadataMatch, error) {
	reqURL := fmt.Sprintf("https://musicbrainz.org/ws/2/recording/%s?inc=artists+releases+tags&fmt=json", recordingID)

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("User-Agent", "CineVault/1.0 (https://github.com/JustinTDCT/CineVault)")

	s.waitRateLimit()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("MusicBrainz recording details returned %d", resp.StatusCode)
	}

	var r struct {
		ID             string `json:"id"`
		Title          string `json:"title"`
		Length         int    `json:"length"`
		FirstReleaseDate string `json:"first-release-date"`
		ArtistCredit   []struct {
			Name   string `json:"name"`
			Artist struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"artist"`
		} `json:"artist-credit"`
		Releases []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Date  string `json:"date"`
			CoverArtArchive struct {
				Front bool `json:"front"`
			} `json:"cover-art-archive"`
		} `json:"releases"`
		Tags []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	var year *int
	if r.FirstReleaseDate != "" && len(r.FirstReleaseDate) >= 4 {
		y := 0
		fmt.Sscanf(r.FirstReleaseDate[:4], "%d", &y)
		if y > 0 {
			year = &y
		}
	}

	var desc *string
	var artistName, artistMBID string
	var artists []string
	for _, ac := range r.ArtistCredit {
		name := ac.Name
		if ac.Artist.Name != "" {
			name = ac.Artist.Name
		}
		artists = append(artists, name)
		if artistMBID == "" && ac.Artist.ID != "" {
			artistMBID = ac.Artist.ID
		}
	}
	if len(artists) > 0 {
		artistName = strings.Join(artists, ", ")
		d := "Recording by " + artistName
		desc = &d
	}

	var genres []string
	for _, t := range r.Tags {
		genres = append(genres, t.Name)
	}

	// Try to get cover art from the first release
	var posterURL *string
	var albumTitle string
	for _, rel := range r.Releases {
		if albumTitle == "" {
			albumTitle = rel.Title
		}
		if rel.CoverArtArchive.Front {
			p := fmt.Sprintf("https://coverartarchive.org/release/%s/front-500", rel.ID)
			posterURL = &p
			if albumTitle == "" {
				albumTitle = rel.Title
			}
			break
		}
	}

	// Fetch record label from the first release (requires separate API call)
	var recordLabel string
	if len(r.Releases) > 0 {
		recordLabel = s.fetchReleaseLabel(r.Releases[0].ID)
	}

	return &models.MetadataMatch{
		Source:      "musicbrainz",
		ExternalID:  r.ID,
		Title:       r.Title,
		Year:        year,
		Description: desc,
		PosterURL:   posterURL,
		Genres:      genres,
		Confidence:  1.0,
		ArtistName:  artistName,
		ArtistMBID:  artistMBID,
		AlbumTitle:  albumTitle,
		RecordLabel: recordLabel,
	}, nil
}

// fetchReleaseLabel fetches label info for a MusicBrainz release.
func (s *MusicBrainzScraper) fetchReleaseLabel(releaseID string) string {
	reqURL := fmt.Sprintf("https://musicbrainz.org/ws/2/release/%s?inc=labels&fmt=json", releaseID)

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("User-Agent", "CineVault/1.0 (https://github.com/JustinTDCT/CineVault)")

	s.waitRateLimit()
	resp, err := s.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	var release struct {
		LabelInfo []struct {
			CatalogNumber string `json:"catalog-number"`
			Label         *struct {
				Name string `json:"name"`
			} `json:"label"`
		} `json:"label-info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return ""
	}

	for _, li := range release.LabelInfo {
		if li.Label != nil && li.Label.Name != "" && li.Label.Name != "[no label]" {
			return li.Label.Name
		}
	}
	return ""
}
