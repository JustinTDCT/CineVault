// Package match links library titles to TMDb and stores their metadata.
package match

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/JustinTDCT/CineVault/internal/tmdb"
)

// Title is a title waiting to be matched.
type Title struct {
	ID        int64
	MediaType string // "movie" or "tv"
	Name      string // as read from file names
	Year      int    // 0 when unknown
	TMDbID    int    // from a {tmdb-…} tag, 0 when absent
}

// Store is what the matcher needs from the database.
type Store interface {
	// TitlesToMatch returns titles that are unmatched or failed with an error.
	TitlesToMatch(ctx context.Context, libraryID int64) ([]Title, error)
	SaveMatch(ctx context.Context, titleID int64, m tmdb.Movie) error
	// SaveSeason stores a season's details and all its episodes, owned or not.
	SaveSeason(ctx context.Context, titleID int64, s tmdb.Season) error
	// ShowsNeedingEpisodes lists matched shows with episodes that have no TMDb
	// details yet (new files since the show was matched), by season.
	ShowsNeedingEpisodes(ctx context.Context, libraryID int64) ([]ShowSeasons, error)
	// TitlesNeedingCredits lists matched movies and shows with no cast yet
	// (matched before cast was stored).
	TitlesNeedingCredits(ctx context.Context, libraryID int64) ([]Title, error)
	// SaveCredits stores a title's cast and directors (a show's creators).
	SaveCredits(ctx context.Context, titleID int64, c tmdb.Credits) error
	// SaveMatchFailure records status "not_found" or "error".
	SaveMatchFailure(ctx context.Context, titleID int64, status, message string) error
}

// Source is the TMDb API (tmdb.Client in production).
type Source interface {
	SearchMovie(ctx context.Context, query string, year int) ([]tmdb.SearchResult, error)
	Movie(ctx context.Context, id int) (tmdb.Movie, error)
	SearchTV(ctx context.Context, query string, year int) ([]tmdb.SearchResult, error)
	TV(ctx context.Context, id int) (tmdb.Show, error)
	Season(ctx context.Context, showID, season int) (tmdb.Season, error)
}

// ShowSeasons is a matched show and the seasons to fetch for it.
type ShowSeasons struct {
	TitleID int64
	Name    string
	TMDbID  int
	Seasons []int
}

// Result summarizes a matching run.
type Result struct {
	Matched  int `json:"matched"`
	NotFound int `json:"not_found"`
	Errors   int `json:"errors"`
	// Episodes is how many TV episodes had details fetched.
	Episodes int `json:"episodes"`
	// Credits is how many titles matched earlier had their cast fetched.
	Credits  int      `json:"credits"`
	Messages []string `json:"messages"`
}

// Matcher matches titles in movie and TV libraries.
type Matcher struct {
	Store  Store
	Source Source
}

// MatchLibrary matches every unmatched title in the library. A rejected TMDb
// token stops the run, since every other title would fail the same way.
func (m Matcher) MatchLibrary(ctx context.Context, libraryID int64) (Result, error) {
	res := Result{Messages: []string{}}
	titles, err := m.Store.TitlesToMatch(ctx, libraryID)
	if err != nil {
		return res, fmt.Errorf("load titles to match: %w", err)
	}
	for _, t := range titles {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		var (
			movie tmdb.Movie
			show  tmdb.Show
			found bool
			err   error
		)
		if t.MediaType == "tv" {
			show, found, err = m.findShow(ctx, t)
			movie = show.AsMovie()
		} else {
			movie, found, err = m.find(ctx, t)
		}
		switch {
		case errors.Is(err, tmdb.ErrUnauthorized):
			return res, err
		case err != nil:
			res.Errors++
			res.Messages = append(res.Messages, fmt.Sprintf("%s: %v", label(t), err))
			if serr := m.Store.SaveMatchFailure(ctx, t.ID, "error", err.Error()); serr != nil {
				return res, serr
			}
		case !found:
			res.NotFound++
			res.Messages = append(res.Messages, label(t)+": no match on TMDb")
			if serr := m.Store.SaveMatchFailure(ctx, t.ID, "not_found", "no match on TMDb"); serr != nil {
				return res, serr
			}
		default:
			if err := m.Store.SaveMatch(ctx, t.ID, movie); err != nil {
				return res, fmt.Errorf("save match for %s: %w", label(t), err)
			}
			res.Matched++
			if t.MediaType == "tv" {
				var seasons []int
				for _, s := range show.Seasons {
					seasons = append(seasons, s.SeasonNumber)
				}
				if err := m.fetchSeasons(ctx, ShowSeasons{TitleID: t.ID, Name: label(t), TMDbID: show.ID, Seasons: seasons}, &res); err != nil {
					return res, err
				}
			}
		}
	}

	// Cast for titles matched before cast was stored.
	if err := m.fetchCredits(ctx, libraryID, &res); err != nil {
		return res, err
	}

	// Episodes added to shows that were matched on an earlier scan.
	shows, err := m.Store.ShowsNeedingEpisodes(ctx, libraryID)
	if err != nil {
		return res, fmt.Errorf("load shows with new episodes: %w", err)
	}
	for _, s := range shows {
		if err := m.fetchSeasons(ctx, s, &res); err != nil {
			return res, err
		}
	}
	return res, nil
}

// fetchCredits stores the cast of matched titles that have none yet. A
// failure is reported and the title is tried again on the next match.
func (m Matcher) fetchCredits(ctx context.Context, libraryID int64, res *Result) error {
	titles, err := m.Store.TitlesNeedingCredits(ctx, libraryID)
	if err != nil {
		return fmt.Errorf("load titles needing cast: %w", err)
	}
	for _, t := range titles {
		if err := ctx.Err(); err != nil {
			return err
		}
		var credits tmdb.Credits
		if t.MediaType == "tv" {
			var s tmdb.Show
			s, err = m.Source.TV(ctx, t.TMDbID)
			credits = s.AsMovie().Credits
		} else {
			var mv tmdb.Movie
			mv, err = m.Source.Movie(ctx, t.TMDbID)
			credits = mv.Credits
		}
		if errors.Is(err, tmdb.ErrUnauthorized) {
			return err
		}
		if err != nil {
			res.Messages = append(res.Messages, fmt.Sprintf("%s: cast: %v", label(t), err))
			continue
		}
		if err := m.Store.SaveCredits(ctx, t.ID, credits); err != nil {
			return fmt.Errorf("save cast for %s: %w", label(t), err)
		}
		res.Credits++
	}
	return nil
}

// fetchSeasons stores each season's episodes. A season TMDb doesn't have is
// skipped; other failures are reported and the rest carry on.
func (m Matcher) fetchSeasons(ctx context.Context, s ShowSeasons, res *Result) error {
	for _, n := range s.Seasons {
		season, err := m.Source.Season(ctx, s.TMDbID, n)
		switch {
		case errors.Is(err, tmdb.ErrNotFound):
			continue
		case errors.Is(err, tmdb.ErrUnauthorized):
			return err
		case err != nil:
			res.Errors++
			res.Messages = append(res.Messages, fmt.Sprintf("%s season %d: %v", s.Name, n, err))
			continue
		}
		if err := m.Store.SaveSeason(ctx, s.TitleID, season); err != nil {
			return fmt.Errorf("save %s season %d: %w", s.Name, n, err)
		}
		res.Episodes += len(season.Episodes)
	}
	return nil
}

func (m Matcher) findShow(ctx context.Context, t Title) (tmdb.Show, bool, error) {
	if t.TMDbID > 0 {
		show, err := m.Source.TV(ctx, t.TMDbID)
		if errors.Is(err, tmdb.ErrNotFound) {
			return show, false, nil
		}
		return show, err == nil, err
	}
	results, err := m.Source.SearchTV(ctx, t.Name, t.Year)
	if err != nil {
		return tmdb.Show{}, false, err
	}
	best, ok := Pick(t, results)
	if !ok && t.Year > 0 {
		if results, err = m.Source.SearchTV(ctx, t.Name, 0); err != nil {
			return tmdb.Show{}, false, err
		}
		best, ok = Pick(t, results)
	}
	if !ok {
		return tmdb.Show{}, false, nil
	}
	show, err := m.Source.TV(ctx, best.ID)
	return show, err == nil, err
}

func (m Matcher) find(ctx context.Context, t Title) (tmdb.Movie, bool, error) {
	if t.TMDbID > 0 {
		movie, err := m.Source.Movie(ctx, t.TMDbID)
		if errors.Is(err, tmdb.ErrNotFound) {
			return movie, false, nil
		}
		return movie, err == nil, err
	}
	results, err := m.Source.SearchMovie(ctx, t.Name, t.Year)
	if err != nil {
		return tmdb.Movie{}, false, err
	}
	best, ok := Pick(t, results)
	if !ok && t.Year > 0 {
		// Release years differ between countries; retry without the year.
		if results, err = m.Source.SearchMovie(ctx, t.Name, 0); err != nil {
			return tmdb.Movie{}, false, err
		}
		best, ok = Pick(t, results)
	}
	if !ok {
		return tmdb.Movie{}, false, nil
	}
	movie, err := m.Source.Movie(ctx, best.ID)
	return movie, err == nil, err
}

// Pick chooses the search result that matches the title, or reports none.
// The name must match TMDb's title or original title once punctuation,
// case and a leading "The" are ignored. When the file has a year, the
// release year must be within one year of it (release dates vary by country).
// Among acceptable results, an exact year wins, then TMDb's own ranking.
func Pick(t Title, results []tmdb.SearchResult) (tmdb.SearchResult, bool) {
	want := Normalize(t.Name)
	bestScore := -1
	var best tmdb.SearchResult
	for _, r := range results {
		if Normalize(r.Title) != want && Normalize(r.OriginalTitle) != want {
			continue
		}
		score := 0
		if t.Year > 0 {
			switch d := r.Year() - t.Year; {
			case d == 0:
				score = 2
			case d == 1 || d == -1:
				score = 1
			default:
				continue
			}
		}
		if score > bestScore {
			best, bestScore = r, score
		}
	}
	return best, bestScore >= 0
}

// Normalize makes titles comparable: lowercase, "&" as "and", punctuation
// removed, and a leading "the" dropped. "Mission: Impossible - Fallout" and
// "Mission Impossible Fallout" normalize the same.
func Normalize(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "&", " and ")
	var b strings.Builder
	space := true
	for _, r := range s {
		switch {
		case r == '\'' || r == '’':
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		default:
			if !space {
				b.WriteByte(' ')
				space = true
			}
		}
	}
	out := strings.TrimSpace(b.String())
	return strings.TrimPrefix(out, "the ")
}

func label(t Title) string {
	if t.Year > 0 {
		return fmt.Sprintf("%s (%d)", t.Name, t.Year)
	}
	return t.Name
}
