package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// FanartTVClient fetches extended artwork from fanart.tv.
type FanartTVClient struct {
	apiKey string
	client *http.Client
}

// FanartArtwork holds the different artwork types available from fanart.tv.
type FanartArtwork struct {
	LogoURL     string
	ClearArtURL string
	BannerURL   string
	DiscURL     string
	ThumbURL    string
	BackdropURL string
}

func NewFanartTVClient(apiKey string) *FanartTVClient {
	return &FanartTVClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetMovieArtwork fetches extended artwork for a movie by TMDB ID.
func (c *FanartTVClient) GetMovieArtwork(tmdbID string) (*FanartArtwork, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("fanart.tv API key not configured")
	}

	reqURL := fmt.Sprintf("https://webservice.fanart.tv/v3/movies/%s?api_key=%s", tmdbID, c.apiKey)
	resp, err := c.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fanart.tv returned %d", resp.StatusCode)
	}

	var result struct {
		HDMovieLogos    []fanartImage `json:"hdmovielogo"`
		MovieLogos      []fanartImage `json:"movielogo"`
		HDClearArt      []fanartImage `json:"hdmovieclearart"`
		MovieClearArt   []fanartImage `json:"movieclearart"`
		MovieBanners    []fanartImage `json:"moviebanner"`
		MovieDiscs      []fanartImage `json:"moviedisc"`
		MovieThumbs     []fanartImage `json:"moviethumb"`
		MovieBackgrounds []fanartImage `json:"moviebackground"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	art := &FanartArtwork{}
	art.LogoURL = firstFanartURL(result.HDMovieLogos, result.MovieLogos)
	art.ClearArtURL = firstFanartURL(result.HDClearArt, result.MovieClearArt)
	art.BannerURL = firstFanartURL(result.MovieBanners)
	art.DiscURL = firstFanartURL(result.MovieDiscs)
	art.ThumbURL = firstFanartURL(result.MovieThumbs)
	art.BackdropURL = firstFanartURL(result.MovieBackgrounds)

	return art, nil
}

// GetTVArtwork fetches extended artwork for a TV show by TVDB ID.
func (c *FanartTVClient) GetTVArtwork(tvdbID string) (*FanartArtwork, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("fanart.tv API key not configured")
	}

	reqURL := fmt.Sprintf("https://webservice.fanart.tv/v3/tv/%s?api_key=%s", tvdbID, c.apiKey)
	resp, err := c.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fanart.tv returned %d", resp.StatusCode)
	}

	var result struct {
		HDTVLogos       []fanartImage `json:"hdtvlogo"`
		ClearLogos      []fanartImage `json:"clearlogo"`
		HDClearArt      []fanartImage `json:"hdclearart"`
		ClearArt        []fanartImage `json:"clearart"`
		TVBanners       []fanartImage `json:"tvbanner"`
		TVThumbs        []fanartImage `json:"tvthumb"`
		ShowBackgrounds []fanartImage `json:"showbackground"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	art := &FanartArtwork{}
	art.LogoURL = firstFanartURL(result.HDTVLogos, result.ClearLogos)
	art.ClearArtURL = firstFanartURL(result.HDClearArt, result.ClearArt)
	art.BannerURL = firstFanartURL(result.TVBanners)
	art.ThumbURL = firstFanartURL(result.TVThumbs)
	art.BackdropURL = firstFanartURL(result.ShowBackgrounds)

	return art, nil
}

type fanartImage struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Likes string `json:"likes"`
	Lang  string `json:"lang"`
}

// firstFanartURL returns the URL of the first image from multiple preference-ordered slices.
// Prefers English-language images.
func firstFanartURL(imageSets ...[]fanartImage) string {
	// First pass: find an English image
	for _, images := range imageSets {
		for _, img := range images {
			if (img.Lang == "en" || img.Lang == "") && img.URL != "" {
				return img.URL
			}
		}
	}
	// Second pass: any language
	for _, images := range imageSets {
		if len(images) > 0 && images[0].URL != "" {
			return images[0].URL
		}
	}
	return ""
}
