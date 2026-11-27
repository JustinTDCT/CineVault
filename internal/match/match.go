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
	ID     int64
	Name   string // as read from file names
	Year   int    // 0 when unknown
	TMDbID int    // from a {tmdb-…} tag, 0 when absent
}

// Store is what the matcher needs from the database.
type Store interface {
	// TitlesToMatch returns titles that are unmatched or failed with an error.
	TitlesToMatch(ctx context.Context, libraryID int64) ([]Title, error)
	SaveMatch(ctx context.Context, titleID int64, m tmdb.Movie) error
	// SaveMatchFailure records status "not_found" or "error".
	SaveMatchFailure(ctx context.Context, titleID int64, status, message string) error
}

// Source is the TMDb API (tmdb.Client in production).
type Source interface {
	SearchMovie(ctx context.Context, query string, year int) ([]tmdb.SearchResult, error)
	Movie(ctx context.Context, id int) (tmdb.Movie, error)
}

// Result summarizes a matching run.
type Result struct {
	Matched  int      `json:"matched"`
	NotFound int      `json:"not_found"`
	Errors   int      `json:"errors"`
	Messages []string `json:"messages"`
}

// Matcher matches titles in movie libraries.
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
		movie, found, err := m.find(ctx, t)
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
		}
	}
	return res, nil
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
