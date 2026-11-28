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
	// or failed with an error; with all, every music video in it.
	MusicVideosToMatch(ctx context.Context, libraryID int64, all bool) ([]MusicVideo, error)
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
	return m.match(ctx, libraryID, false)
}

// RematchLibrary matches every music video in the library again, matched
// or not, for example after the album rule changed. A video whose recording
// changes is looked up in the cache afresh.
func (m MusicVideoMatcher) RematchLibrary(ctx context.Context, libraryID int64) (Result, error) {
	return m.match(ctx, libraryID, true)
}

func (m MusicVideoMatcher) match(ctx context.Context, libraryID int64, all bool) (Result, error) {
	res := Result{Messages: []string{}}
	videos, err := m.Store.MusicVideosToMatch(ctx, libraryID, all)
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
// Among those, the one whose album is best (see musicbrainz.Album) wins.
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
	// Every compilation's copy of a song can be its own recording, so the
	// recordings compete by their best album: the artist's own studio album
	// beats a single beats a compilation, then the earliest, then the score.
	type cand struct {
		rec  musicbrainz.Recording
		rank int
		date string
	}
	cs := make([]cand, 0, len(ok))
	for _, r := range ok {
		c := cand{rec: r, rank: 99, date: "9999"}
		if a, has := r.Album(); has {
			c.rank, c.date = musicbrainz.AlbumRank(a, r.Artist), musicbrainz.AlbumDate(a)
		}
		cs = append(cs, c)
	}
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.date != b.date {
			return a.date < b.date
		}
		return a.rec.Score > b.rec.Score
	})
	return cs[0].rec, true
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
