package match

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/comicvine"
	"github.com/JustinTDCT/CineVault/internal/musicbrainz"
	"github.com/JustinTDCT/CineVault/internal/openlibrary"
)

// LookupItem is an album, book, audiobook or comic series waiting for a
// lookup: its name and artist (or author) as shown, and the year the files
// give.
type LookupItem struct {
	ID        int64
	MediaType string
	Name      string
	Artist    string
	Year      int
}

func (it LookupItem) label() string {
	if it.Artist != "" {
		return it.Artist + " – " + it.Name
	}
	return it.Name
}

// Found is what a lookup found. Empty fields leave the title's own.
type Found struct {
	MBReleaseGroupID string // albums
	OLWorkID         string // books and audiobooks
	ComicVineID      int    // comic series

	Name      string
	Artist    string // an album's artist, a book's author
	Year      int
	Overview  string
	Genres    []string
	CoverURL  string
	Publisher string
}

// LookupStore is what the lookup matchers need from the database.
type LookupStore interface {
	// LookupItems returns a library's titles that are unmatched or failed
	// with an error; with all, every title that isn't locked.
	LookupItems(ctx context.Context, libraryID int64, all bool) ([]LookupItem, error)
	// SaveLookup stores a match. Locks and the library's preference for its
	// files' metadata decide which fields change.
	SaveLookup(ctx context.Context, titleID int64, f Found) error
	SaveMatchFailure(ctx context.Context, titleID int64, status, message string) error
}

// Finder looks one title up at one source.
type Finder interface {
	// Source names the service, for messages: "MusicBrainz".
	Source() string
	// NeedsArtist: without an artist (or author) a match would be a guess.
	NeedsArtist() bool
	// Find returns the match, or false when the source has none.
	Find(ctx context.Context, it LookupItem) (Found, bool, error)
}

// LookupMatcher matches a library's titles with a Finder.
type LookupMatcher struct {
	Store  LookupStore
	Finder Finder
}

// MatchLibrary matches every unmatched title in the library.
func (m LookupMatcher) MatchLibrary(ctx context.Context, libraryID int64) (Result, error) {
	return m.run(ctx, libraryID, false)
}

// RematchLibrary matches every title again, matched or not (locked ones
// excepted), for example after the source's data improved.
func (m LookupMatcher) RematchLibrary(ctx context.Context, libraryID int64) (Result, error) {
	return m.run(ctx, libraryID, true)
}

func (m LookupMatcher) run(ctx context.Context, libraryID int64, all bool) (Result, error) {
	res := Result{Messages: []string{}}
	items, err := m.Store.LookupItems(ctx, libraryID, all)
	if err != nil {
		return res, fmt.Errorf("load titles to match: %w", err)
	}
	src := m.Finder.Source()
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		fail := func(status, msg string) error {
			if status == "error" {
				res.Errors++
			} else {
				res.NotFound++
			}
			res.Messages = append(res.Messages, it.label()+": "+msg)
			return m.Store.SaveMatchFailure(ctx, it.ID, status, msg)
		}
		if m.Finder.NeedsArtist() && Normalize(it.Artist) == "" {
			if err := fail("not_found", "no artist or author in the tags or file names"); err != nil {
				return res, err
			}
			continue
		}
		f, ok, err := m.Finder.Find(ctx, it)
		if errors.Is(err, comicvine.ErrBadKey) {
			return res, err // every other title would fail the same way
		}
		if err != nil {
			if serr := fail("error", err.Error()); serr != nil {
				return res, serr
			}
			continue
		}
		if !ok {
			if err := fail("not_found", "no match on "+src); err != nil {
				return res, err
			}
			continue
		}
		if err := m.Store.SaveLookup(ctx, it.ID, f); err != nil {
			return res, fmt.Errorf("save match for %s: %w", it.label(), err)
		}
		res.Matched++
	}
	return res, nil
}

// sameArtist reports whether two artist or author names name the same act,
// ignoring case, punctuation and spacing ("J.R.R. Tolkien" and "J. R. R.
// Tolkien", or "Herbert, Frank" and "Frank Herbert"), allowing joint credits ("Queen & David Bowie") and, for people,
// a surname match ("Tolkien" and "J.R.R. Tolkien").
func sameArtist(a, b string) bool {
	na, nb := Normalize(firstLast(a)), Normalize(firstLast(b))
	if na == "" || nb == "" {
		return false
	}
	squash := func(s string) string { return strings.ReplaceAll(s, " ", "") }
	sa, sb := squash(na), squash(nb)
	if strings.Contains(sa, sb) || strings.Contains(sb, sa) {
		return true
	}
	fa, fb := strings.Fields(na), strings.Fields(nb)
	return len(fa) > 1 && len(fb) > 1 && fa[len(fa)-1] == fb[len(fb)-1] && fa[0][0] == fb[0][0]
}

// firstLast turns "Herbert, Frank" (how book files often list authors)
// into "Frank Herbert"; other names are returned as they are.
func firstLast(s string) string {
	last, first, ok := strings.Cut(s, ",")
	if !ok || strings.Contains(first, ",") || strings.TrimSpace(first) == "" || strings.Contains(last, "&") {
		return s
	}
	return strings.TrimSpace(first) + " " + strings.TrimSpace(last)
}

// searchTitles are the titles to search a book by: its main title, before
// a subtitle or a bracket, and then the whole name when they differ.
func searchTitles(name string) []string {
	main := name
	if i := strings.IndexAny(name, ":("); i > 0 {
		main = strings.TrimSpace(name[:i])
	}
	if main == name {
		return []string{name}
	}
	return []string{main, name}
}

// mainTitle is a book title without its subtitle: "Dune: Deluxe Edition"
// and "Dune" compare equal.
func mainTitle(s string) string {
	if i := strings.IndexAny(s, ":("); i > 0 {
		s = s[:i]
	}
	return Normalize(s)
}

// AlbumFinder matches albums to MusicBrainz release groups, with genres and
// the Cover Art Archive's front cover.
type AlbumFinder struct {
	MB interface {
		SearchReleaseGroups(ctx context.Context, artist, album string) ([]musicbrainz.ReleaseGroup, error)
		ReleaseGroupGenres(ctx context.Context, id string, max int) ([]string, error)
		CoverArtURL(ctx context.Context, releaseGroupID string) (string, error)
	}
}

func (AlbumFinder) Source() string    { return "MusicBrainz" }
func (AlbumFinder) NeedsArtist() bool { return true }

func (f AlbumFinder) Find(ctx context.Context, it LookupItem) (Found, bool, error) {
	groups, err := f.MB.SearchReleaseGroups(ctx, it.Artist, it.Name)
	if err != nil {
		return Found{}, false, err
	}
	g, ok := PickReleaseGroup(it, groups)
	if !ok {
		return Found{}, false, nil
	}
	out := Found{MBReleaseGroupID: g.ID, Name: g.Title, Artist: g.Artist, Year: yearOf(g.FirstReleaseDate)}
	if out.Genres, err = f.MB.ReleaseGroupGenres(ctx, g.ID, 4); err != nil {
		return Found{}, false, err
	}
	if out.CoverURL, err = f.MB.CoverArtURL(ctx, g.ID); err != nil {
		return Found{}, false, err
	}
	return out, true, nil
}

// PickReleaseGroup chooses an album's release group: a search result
// scoring at least minScore with the album's title and artist. Among those:
// the year the files give, then a studio album (before an EP, a
// compilation or live album, then a single), then the score, then the
// earliest.
func PickReleaseGroup(it LookupItem, groups []musicbrainz.ReleaseGroup) (musicbrainz.ReleaseGroup, bool) {
	title := Normalize(it.Name)
	var ok []musicbrainz.ReleaseGroup
	for _, g := range groups {
		if g.Score < minScore || g.ID == "" || Normalize(g.Title) != title || !sameArtist(g.Artist, it.Artist) {
			continue
		}
		ok = append(ok, g)
	}
	if len(ok) == 0 {
		return musicbrainz.ReleaseGroup{}, false
	}
	kind := func(g musicbrainz.ReleaseGroup) int {
		switch {
		case g.PrimaryType == "Album" && len(g.SecondaryTypes) == 0:
			return 0
		case g.PrimaryType == "EP" && len(g.SecondaryTypes) == 0:
			return 1
		case g.PrimaryType == "Album" || g.PrimaryType == "EP":
			return 2
		case g.PrimaryType == "Single":
			return 3
		}
		return 4
	}
	sort.SliceStable(ok, func(i, j int) bool {
		a, b := ok[i], ok[j]
		if it.Year > 0 {
			if ya, yb := yearOf(a.FirstReleaseDate) == it.Year, yearOf(b.FirstReleaseDate) == it.Year; ya != yb {
				return ya
			}
		}
		if ka, kb := kind(a), kind(b); ka != kb {
			return ka < kb
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return dateKey(a.FirstReleaseDate) < dateKey(b.FirstReleaseDate)
	})
	return ok[0], true
}

func dateKey(d string) string {
	if d == "" {
		return "9999"
	}
	return d
}

// BookFinder matches books and audiobooks to Open Library works, with the
// work's description, subjects and cover.
type BookFinder struct {
	OL interface {
		Search(ctx context.Context, title, author string) ([]openlibrary.Work, error)
		WorkDetails(ctx context.Context, id string) (openlibrary.Details, error)
	}
}

func (BookFinder) Source() string    { return "Open Library" }
func (BookFinder) NeedsArtist() bool { return true }

func (f BookFinder) Find(ctx context.Context, it LookupItem) (Found, bool, error) {
	// Search by the main title first ("Beowulf", not "Beowulf: A Translation
	// and Commentary, together with Sellic Spell"), then by the whole name.
	var w openlibrary.Work
	ok := false
	for _, q := range searchTitles(it.Name) {
		works, err := f.OL.Search(ctx, q, it.Artist)
		if err != nil {
			return Found{}, false, err
		}
		if w, ok = PickWork(it, works); ok {
			break
		}
	}
	if !ok {
		return Found{}, false, nil
	}
	d, err := f.OL.WorkDetails(ctx, w.ID)
	if err != nil {
		return Found{}, false, err
	}
	authors := w.Authors
	if len(authors) > 2 {
		authors = authors[:2]
	}
	return Found{OLWorkID: w.ID, Name: w.Title, Artist: strings.Join(authors, " & "), Year: w.FirstPublishYear,
		Overview: d.Description, Genres: openlibrary.Genres(d.Subjects, 4), CoverURL: w.CoverURL()}, true, nil
}

// PickWork chooses a book's work: the same title (subtitles aside) by an
// author the files name. Among those, the work with the most editions is
// the book itself rather than a study guide or an excerpt; then the
// earliest.
func PickWork(it LookupItem, works []openlibrary.Work) (openlibrary.Work, bool) {
	title := mainTitle(it.Name)
	var ok []openlibrary.Work
	for _, w := range works {
		if w.ID == "" || mainTitle(w.Title) != title {
			continue
		}
		for _, a := range w.Authors {
			if sameArtist(a, it.Artist) {
				ok = append(ok, w)
				break
			}
		}
	}
	if len(ok) == 0 {
		return openlibrary.Work{}, false
	}
	sort.SliceStable(ok, func(i, j int) bool {
		if ok[i].EditionCount != ok[j].EditionCount {
			return ok[i].EditionCount > ok[j].EditionCount
		}
		yi, yj := ok[i].FirstPublishYear, ok[j].FirstPublishYear
		return yi > 0 && (yj == 0 || yi < yj)
	})
	return ok[0], true
}

// ComicFinder matches comic series to Comic Vine volumes.
type ComicFinder struct {
	CV interface {
		SearchVolumes(ctx context.Context, name string) ([]comicvine.Volume, error)
	}
}

func (ComicFinder) Source() string    { return "Comic Vine" }
func (ComicFinder) NeedsArtist() bool { return false }

func (f ComicFinder) Find(ctx context.Context, it LookupItem) (Found, bool, error) {
	vols, err := f.CV.SearchVolumes(ctx, it.Name)
	if err != nil {
		return Found{}, false, err
	}
	v, ok := PickVolume(it, vols)
	if !ok {
		return Found{}, false, nil
	}
	overview := v.Description
	if overview == "" {
		overview = v.Deck
	}
	return Found{ComicVineID: v.ID, Name: v.Name, Year: v.StartYear, Publisher: v.Publisher, Overview: overview,
		CoverURL: v.ImageURL}, true, nil
}

// PickVolume chooses a series' volume: the same name. When the files give a
// year, the volume must start that year (many series restart under the same
// name); otherwise the one with the most issues, then the earliest.
func PickVolume(it LookupItem, vols []comicvine.Volume) (comicvine.Volume, bool) {
	name := Normalize(it.Name)
	var ok []comicvine.Volume
	for _, v := range vols {
		if v.ID <= 0 || Normalize(v.Name) != name || (it.Year > 0 && v.StartYear != it.Year) {
			continue
		}
		ok = append(ok, v)
	}
	if len(ok) == 0 {
		return comicvine.Volume{}, false
	}
	sort.SliceStable(ok, func(i, j int) bool {
		if ok[i].IssueCount != ok[j].IssueCount {
			return ok[i].IssueCount > ok[j].IssueCount
		}
		return ok[i].StartYear < ok[j].StartYear
	})
	return ok[0], true
}
