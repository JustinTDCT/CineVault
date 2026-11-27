package tmdb

import (
	"context"
	"net/url"
	"strconv"
)

// Show is a TV show's details.
type Show struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	OriginalName   string  `json:"original_name"`
	Overview       string  `json:"overview"`
	Tagline        string  `json:"tagline"`
	FirstAirDate   string  `json:"first_air_date"`
	EpisodeRunTime []int   `json:"episode_run_time"`
	PosterPath     string  `json:"poster_path"`
	BackdropPath   string  `json:"backdrop_path"`
	VoteAverage    float64 `json:"vote_average"`
	Status         string  `json:"status"` // e.g. "Ended", "Returning Series"
	Genres         []struct {
		Name string `json:"name"`
	} `json:"genres"`
	ExternalIDs struct {
		IMDbID string `json:"imdb_id"`
	} `json:"external_ids"`
	Seasons []SeasonSummary `json:"seasons"`
}

// SeasonSummary is a season as listed in a show's details.
type SeasonSummary struct {
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	EpisodeCount int    `json:"episode_count"`
}

// Season is one season with its episodes.
type Season struct {
	SeasonNumber int       `json:"season_number"`
	Name         string    `json:"name"`
	Overview     string    `json:"overview"`
	PosterPath   string    `json:"poster_path"`
	AirDate      string    `json:"air_date"`
	Episodes     []Episode `json:"episodes"`
}

// Episode is one episode of a season.
type Episode struct {
	ID            int     `json:"id"`
	SeasonNumber  int     `json:"season_number"`
	EpisodeNumber int     `json:"episode_number"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"air_date"`
	Runtime       int     `json:"runtime"` // minutes
	StillPath     string  `json:"still_path"`
	VoteAverage   float64 `json:"vote_average"`
}

// AsMovie maps the show's fields onto a Movie, so shows are stored in the
// same title columns: name, first air date as release date, typical episode
// runtime, posters and genres.
func (s Show) AsMovie() Movie {
	m := Movie{
		ID: s.ID, IMDbID: s.ExternalIDs.IMDbID, Title: s.Name, OriginalTitle: s.OriginalName,
		Overview: s.Overview, Tagline: s.Tagline, ReleaseDate: s.FirstAirDate,
		PosterPath: s.PosterPath, BackdropPath: s.BackdropPath, VoteAverage: s.VoteAverage,
	}
	if len(s.EpisodeRunTime) > 0 {
		m.Runtime = s.EpisodeRunTime[0]
	}
	for _, g := range s.Genres {
		m.Genres = append(m.Genres, struct {
			Name string `json:"name"`
		}{g.Name})
	}
	return m
}

// SearchTV searches shows by name, optionally narrowed to the year the show
// first aired (0 = any). Results use SearchResult's fields: Title is the
// show's name and ReleaseDate its first air date.
func (c *Client) SearchTV(ctx context.Context, query string, year int) ([]SearchResult, error) {
	q := url.Values{"query": {query}, "include_adult": {"false"}}
	if year > 0 {
		q.Set("first_air_date_year", strconv.Itoa(year))
	}
	var out struct {
		Results []struct {
			ID           int     `json:"id"`
			Name         string  `json:"name"`
			OriginalName string  `json:"original_name"`
			FirstAirDate string  `json:"first_air_date"`
			Popularity   float64 `json:"popularity"`
		} `json:"results"`
	}
	if err := c.get(ctx, "/search/tv", q, &out); err != nil {
		return nil, err
	}
	res := make([]SearchResult, 0, len(out.Results))
	for _, r := range out.Results {
		res = append(res, SearchResult{ID: r.ID, Title: r.Name, OriginalTitle: r.OriginalName, ReleaseDate: r.FirstAirDate, Popularity: r.Popularity})
	}
	return res, nil
}

// TV fetches a show's details, including its IMDb ID and season list.
func (c *Client) TV(ctx context.Context, id int) (Show, error) {
	var s Show
	err := c.get(ctx, "/tv/"+strconv.Itoa(id), url.Values{"append_to_response": {"external_ids"}}, &s)
	return s, err
}

// Season fetches one season of a show with all its episodes.
func (c *Client) Season(ctx context.Context, showID, season int) (Season, error) {
	var s Season
	err := c.get(ctx, "/tv/"+strconv.Itoa(showID)+"/season/"+strconv.Itoa(season), url.Values{}, &s)
	return s, err
}
