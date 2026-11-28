package match

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/musicbrainz"
)

// MusicVideo is a music video title waiting for a MusicBrainz match: the
// artist and song read from its file name.
type MusicVideo struct {
	ID     int64
	Artist string
	Song   string
}

// MusicVideoMatch is what a match stores: the recording, and the song,
// artist, album and year MusicBrainz gives for it.
type MusicVideoMatch struct {
	RecordingID string
	Song        string
	Artist      string
	Album       string
	Year        int
}

// MusicVideoStore is what the music video matcher needs from the database.
type MusicVideoStore interface {
	// MusicVideosToMatch returns a library's music videos that are unmatched
	// or failed with an error.
	MusicVideosToMatch(ctx context.Context, libraryID int64) ([]MusicVideo, error)
	SaveMusicVideoMatch(ctx context.Context, titleID int64, m MusicVideoMatch) error
	SaveMatchFailure(ctx context.Context, titleID int64, status, message string) error
}

// MusicVideoSource is MusicBrainz (musicbrainz.Client in production).
type MusicVideoSource interface {
	SearchRecordings(ctx context.Context, artist, title string) ([]musicbrainz.Recording, error)
}

// MusicVideoMatcher matches music videos to MusicBrainz recordings. That
// match is the key the CineVault Cache uses for them (cache decision 11).
type MusicVideoMatcher struct {
	Store  MusicVideoStore
	Source MusicVideoSource
}

// minScore is the lowest MusicBrainz search score a match may have.
const minScore = 80

// MatchLibrary matches every unmatched music video in the library.
func (m MusicVideoMatcher) MatchLibrary(ctx context.Context, libraryID int64) (Result, error) {
	res := Result{Messages: []string{}}
	videos, err := m.Store.MusicVideosToMatch(ctx, libraryID)
	if err != nil {
		return res, fmt.Errorf("load music videos to match: %w", err)
	}
	for _, v := range videos {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		label := v.Song
		if v.Artist != "" {
			label = v.Artist + " – " + v.Song
		}
		if Normalize(v.Artist) == "" {
			// A song title alone is a guess: many artists share titles.
			res.NotFound++
			res.Messages = append(res.Messages, label+`: no artist in the file name; name it "Artist - Song"`)
			if serr := m.Store.SaveMatchFailure(ctx, v.ID, "not_found", "no artist in the file name"); serr != nil {
				return res, serr
			}
			continue
		}
		recs, err := m.Source.SearchRecordings(ctx, v.Artist, v.Song)
		if err != nil {
			res.Errors++
			res.Messages = append(res.Messages, fmt.Sprintf("%s: %v", label, err))
			if serr := m.Store.SaveMatchFailure(ctx, v.ID, "error", err.Error()); serr != nil {
				return res, serr
			}
			continue
		}
		rec, ok := PickRecording(v, recs)
		if !ok {
			res.NotFound++
			res.Messages = append(res.Messages, label+": no match on MusicBrainz")
			if serr := m.Store.SaveMatchFailure(ctx, v.ID, "not_found", "no match on MusicBrainz"); serr != nil {
				return res, serr
			}
			continue
		}
		mm := MusicVideoMatch{RecordingID: rec.ID, Song: rec.Title, Artist: rec.Artist, Year: yearOf(rec.FirstReleaseDate)}
		if album, ok := rec.Album(); ok {
			mm.Album = album.Title
			for _, d := range []string{album.GroupDate, album.Date} {
				if y := yearOf(d); y > 0 {
					mm.Year = y
					break
				}
			}
		}
		if err := m.Store.SaveMusicVideoMatch(ctx, v.ID, mm); err != nil {
			return res, fmt.Errorf("save match for %s: %w", label, err)
		}
		res.Matched++
	}
	return res, nil
}

// PickRecording chooses the recording for a music video: a search result
// scoring at least minScore whose title is the song and whose artist credit
// names the artist. Without an artist on both sides there is no match.
// Among those, one on an official studio album wins (it
// gives the album to show), then the higher score, then the earlier release.
func PickRecording(v MusicVideo, recs []musicbrainz.Recording) (musicbrainz.Recording, bool) {
	song, artist := Normalize(v.Song), Normalize(v.Artist)
	if song == "" || artist == "" {
		return musicbrainz.Recording{}, false
	}
	var ok []musicbrainz.Recording
	for _, r := range recs {
		if r.Score < minScore || r.ID == "" || Normalize(r.Title) != song {
			continue
		}
		credit := Normalize(r.Artist)
		if credit == "" || (!strings.Contains(credit, artist) && !strings.Contains(artist, credit)) {
			continue
		}
		ok = append(ok, r)
	}
	if len(ok) == 0 {
		return musicbrainz.Recording{}, false
	}
	albumRank := func(r musicbrainz.Recording) int {
		a, has := r.Album()
		switch {
		case !has:
			return 3
		case a.Status == "Official" && a.PrimaryType == "Album" && len(a.SecondaryTypes) == 0:
			return 0
		case a.Status == "Official":
			return 1
		}
		return 2
	}
	first := func(r musicbrainz.Recording) string {
		if r.FirstReleaseDate == "" {
			return "9999"
		}
		return r.FirstReleaseDate
	}
	sort.SliceStable(ok, func(i, j int) bool {
		a, b := ok[i], ok[j]
		if ra, rb := albumRank(a), albumRank(b); ra != rb {
			return ra < rb
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return first(a) < first(b)
	})
	return ok[0], true
}

func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	y := 0
	for _, r := range date[:4] {
		if r < '0' || r > '9' {
			return 0
		}
		y = y*10 + int(r-'0')
	}
	return y
}
