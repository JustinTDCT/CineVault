package metadata

import (
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/libraries"
	"github.com/JustinTDCT/CineVault/internal/media"
)

type Matcher struct {
	db          *sql.DB
	cfg         *config.Config
	cacheClient *CacheClient
	mediaRepo   *media.Repository
}

func NewMatcher(db *sql.DB, cfg *config.Config, cacheClient *CacheClient, mediaRepo *media.Repository) *Matcher {
	return &Matcher{db: db, cfg: cfg, cacheClient: cacheClient, mediaRepo: mediaRepo}
}

func (m *Matcher) AutoMatch(item *media.MediaItem, libType libraries.LibraryType) error {
	if !m.cfg.CacheServerEnabled() || !libType.HasMetadata() {
		return nil
	}

	cacheType := libType.CacheServerType()
	if cacheType == "" {
		return nil
	}

	params := map[string]string{}
	if item.Title != nil {
		params["title"] = *item.Title
	}
	if item.ReleaseYear != nil {
		params["year"] = strconv.Itoa(*item.ReleaseYear)
	}
	if item.SeasonNumber != nil && item.EpisodeNumber != nil {
		params["season"] = strconv.Itoa(*item.SeasonNumber)
		params["episode"] = strconv.Itoa(*item.EpisodeNumber)
	}

	result, err := m.cacheClient.Lookup(cacheType, params)
	if err != nil || result.Status != "ok" || result.Data == nil {
		return err
	}

	autoThreshold := m.getAutomatchThreshold()
	if result.Confidence < autoThreshold {
		return nil
	}

	metaJSON, _ := json.Marshal(result.Data)
	cacheID := result.CacheID
	return m.mediaRepo.UpdateMetadata(item.ID, &cacheID, result.Confidence, metaJSON)
}

func (m *Matcher) ManualSearch(title string, libType libraries.LibraryType, year int) ([]LookupResult, error) {
	cacheType := libType.CacheServerType()
	if cacheType == "" {
		return nil, nil
	}

	params := map[string]string{"title": title}
	if year > 0 {
		params["year"] = strconv.Itoa(year)
	}

	result, err := m.cacheClient.Lookup(cacheType, params)
	if err != nil {
		return nil, err
	}

	if result.Status == "ok" && result.Data != nil {
		return []LookupResult{*result}, nil
	}
	return nil, nil
}

func (m *Matcher) ApplyMatch(itemID, cacheID string) error {
	result, err := m.cacheClient.GetRecord(cacheID)
	if err != nil || result.Data == nil {
		return err
	}

	metaJSON, _ := json.Marshal(result.Data)
	return m.mediaRepo.UpdateMetadata(itemID, &cacheID, result.Confidence, metaJSON)
}

func (m *Matcher) getAutomatchThreshold() float64 {
	var val string
	err := m.db.QueryRow("SELECT value FROM settings WHERE key='automatch_min_pct'").Scan(&val)
	if err == nil {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f / 100.0
		}
	}
	return 0.85
}
