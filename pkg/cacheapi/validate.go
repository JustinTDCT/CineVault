package cacheapi

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/pkg/editionlist"
)

// Provider returns where the key's ID comes from: ProviderTMDb,
// ProviderMusicBrainz, ProviderOpenLibrary or ProviderComicVine.
func (k Key) Provider() string { return ProviderFor(k.Kind) }

// ProviderFor returns the provider a kind's IDs come from; "" for an unknown kind.
func ProviderFor(kind string) string {
	switch kind {
	case KindMovie, KindTV:
		return ProviderTMDb
	case KindMusicVideo, KindAlbum:
		return ProviderMusicBrainz
	case KindBook, KindAudiobook:
		return ProviderOpenLibrary
	case KindComic:
		return ProviderComicVine
	}
	return ""
}

// providerSegment is the provider's short name in a lookup path.
func providerSegment(kind string) string {
	switch ProviderFor(kind) {
	case ProviderMusicBrainz:
		return "mb"
	case ProviderOpenLibrary:
		return "ol"
	case ProviderComicVine:
		return "cv"
	}
	return "tmdb"
}

// ProviderID returns the key's ID as text: "679", a MusicBrainz UUID, an
// Open Library work ID ("OL45883W") or a Comic Vine volume ID ("18166").
func (k Key) ProviderID() string {
	switch k.Provider() {
	case ProviderMusicBrainz:
		return k.MBID
	case ProviderOpenLibrary:
		return k.OLID
	case ProviderComicVine:
		return strconv.Itoa(k.CVID)
	}
	return strconv.Itoa(k.TMDbID)
}

// KeyFor builds a key from a kind and a provider ID as text, the reverse of
// ProviderID. The result still needs Validate.
func KeyFor(kind, providerID string) Key {
	switch ProviderFor(kind) {
	case ProviderMusicBrainz:
		return Key{Kind: kind, MBID: providerID}
	case ProviderOpenLibrary:
		return Key{Kind: kind, OLID: providerID}
	case ProviderComicVine:
		id, _ := strconv.Atoi(providerID)
		return Key{Kind: kind, CVID: id}
	}
	id, _ := strconv.Atoi(providerID)
	return Key{Kind: kind, TMDbID: id}
}

// Path returns the key as used in a lookup path: "movie/tmdb/679",
// "musicvideo/mb/<recording>", "album/mb/<release group>",
// "book/ol/OL45883W" or "comic/cv/18166".
func (k Key) Path() string {
	return k.Kind + "/" + providerSegment(k.Kind) + "/" + k.ProviderID()
}

// String returns the key as "movie:679" or "album:<release group ID>".
func (k Key) String() string { return k.Kind + ":" + k.ProviderID() }

// MediaType returns the Edition List media type for the key's kind.
func (k Key) MediaType() string { return MediaTypeFor(k.Kind) }

// MediaTypeFor returns the Edition List media type for a kind: an album's is
// "music"; the others share their kind's name. "" for an unknown kind.
func MediaTypeFor(kind string) string {
	switch kind {
	case KindMovie:
		return editionlist.Movie
	case KindTV:
		return editionlist.TV
	case KindMusicVideo:
		return editionlist.MusicVideo
	case KindAlbum:
		return editionlist.Music
	case KindBook:
		return editionlist.Book
	case KindAudiobook:
		return editionlist.Audiobook
	case KindComic:
		return editionlist.Comic
	}
	return ""
}

// KindForMediaType returns the kind for a library media type ("music" is
// "album"); "" when the media type has no kind.
func KindForMediaType(mediaType string) string {
	for _, k := range Kinds {
		if MediaTypeFor(k) == mediaType {
			return k
		}
	}
	return ""
}

// Validate checks the key's kind and that exactly the ID its kind uses is set.
func (k Key) Validate() error {
	if k.MediaType() == "" {
		return fmt.Errorf("unknown kind %q", k.Kind)
	}
	others := func(tmdb int, mb, ol string, cv int) bool { return tmdb != 0 || mb != "" || ol != "" || cv != 0 }
	switch k.Provider() {
	case ProviderMusicBrainz:
		what := "recording"
		if k.Kind == KindAlbum {
			what = "release group"
		}
		if !IsMBID(k.MBID) || others(k.TMDbID, "", k.OLID, k.CVID) {
			return fmt.Errorf("%s: needs a MusicBrainz %s ID (a lowercase UUID) and no other ID", k.Kind, what)
		}
	case ProviderOpenLibrary:
		if !IsOLID(k.OLID) || others(k.TMDbID, k.MBID, "", k.CVID) {
			return fmt.Errorf("%s: needs an Open Library work ID such as OL45883W and no other ID", k.Kind)
		}
	case ProviderComicVine:
		if k.CVID <= 0 || others(k.TMDbID, k.MBID, k.OLID, 0) {
			return fmt.Errorf("%s: needs a positive Comic Vine volume ID and no other ID", k.Kind)
		}
	default:
		if k.TMDbID <= 0 || others(0, k.MBID, k.OLID, k.CVID) {
			return fmt.Errorf("%s: TMDb ID must be positive", k.Kind)
		}
	}
	return nil
}

// IsOLID reports whether s is an Open Library work ID: "OL", digits, "W".
func IsOLID(s string) bool {
	if len(s) < 4 || !strings.HasPrefix(s, "OL") || s[len(s)-1] != 'W' {
		return false
	}
	for _, r := range s[2 : len(s)-1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// IsMBID reports whether s is a MusicBrainz ID: a lowercase UUID.
func IsMBID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
				return false
			}
		}
	}
	return true
}

// Validate checks a record against the rules the service promises:
//   - every edition is a built-in Edition List ID for the title's media type,
//     appears once, and editions are in Edition List order
//   - a runtime is never given without a source, and every source a fact
//     points to is listed
//   - withdrawn editions and records carry no facts
//
// The service runs it before serving a record; clients may run it too.
func (r Record) Validate() error {
	if err := r.Key.Validate(); err != nil {
		return err
	}
	if r.RecordVersion < 1 {
		return errors.New("record_version must be at least 1")
	}
	switch r.Status {
	case StatusReady:
	case StatusWithdrawn:
		if len(r.Editions) > 0 || r.Base != nil || r.Credits != nil {
			return errors.New("a withdrawn record has no editions, base metadata or credits")
		}
		return nil
	default:
		return fmt.Errorf("unknown record status %q", r.Status)
	}
	if r.Base != nil && (r.Base.License == "" || r.Base.Title == "") {
		return errors.New("base metadata needs a license and a title")
	}
	if (r.Credits != nil) != (r.Key.Kind == KindMusicVideo) {
		return errors.New("a music video's record has credits, and only a music video's")
	}

	sources := map[string]bool{}
	for _, s := range r.Sources {
		if s.ID == "" || s.Name == "" {
			return errors.New("every source needs an ID and a name")
		}
		if sources[s.ID] {
			return fmt.Errorf("source %q listed twice", s.ID)
		}
		sources[s.ID] = true
	}
	checkSources := func(where string, ids []string) error {
		for _, id := range ids {
			if !sources[id] {
				return fmt.Errorf("%s: unknown source %q", where, id)
			}
		}
		return nil
	}

	if c := r.Credits; c != nil {
		if strings.TrimSpace(c.Song) == "" || strings.TrimSpace(c.Artist) == "" {
			return errors.New("credits need a song and an artist")
		}
		if c.Year < 0 || c.Year > 9999 {
			return fmt.Errorf("credits: year %d is out of range", c.Year)
		}
		set := map[string]bool{
			CreditSong: true, CreditArtist: true, CreditAlbum: c.Album != "", CreditYear: c.Year != 0,
			CreditLabel: c.Label != "", CreditDirector: c.Director != "",
		}
		for f, has := range set {
			if has && len(c.Sources[f]) == 0 {
				return fmt.Errorf("credits: %s without a source", f)
			}
		}
		for f, ids := range c.Sources {
			if !set[f] {
				return fmt.Errorf("credits: sources for %q, which has no value", f)
			}
			if err := checkSources("credits "+f, ids); err != nil {
				return err
			}
		}
		switch c.Confidence {
		case "", ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		default:
			return fmt.Errorf("credits: unknown confidence %q", c.Confidence)
		}
	}

	mediaType := r.Key.MediaType()
	seen := map[string]bool{}
	last := -1
	for _, e := range r.Editions {
		entry, ok := editionlist.Lookup(e.Edition)
		if !ok {
			return fmt.Errorf("edition %q is not in the Edition List", e.Edition)
		}
		if !entry.AppliesTo(mediaType) {
			return fmt.Errorf("edition %q doesn't apply to %s", e.Edition, mediaType)
		}
		if seen[e.Edition] {
			return fmt.Errorf("edition %q listed twice", e.Edition)
		}
		seen[e.Edition] = true
		if p := editionlist.Position(e.Edition); p < last {
			return fmt.Errorf("edition %q is out of Edition List order", e.Edition)
		} else {
			last = p
		}

		switch e.Status {
		case EditionWithdrawn:
			if e.RuntimeMin != 0 || e.DifferenceShort != "" || len(e.DifferenceFull) > 0 || e.ContentRating != nil {
				return fmt.Errorf("withdrawn edition %q carries facts", e.Edition)
			}
			continue
		case EditionPublished:
		default:
			return fmt.Errorf("edition %q: unknown status %q", e.Edition, e.Status)
		}

		if e.RuntimeMin < 0 {
			return fmt.Errorf("edition %q: negative runtime", e.Edition)
		}
		if e.RuntimeMin > 0 && len(e.RuntimeSources) == 0 {
			return fmt.Errorf("edition %q: runtime without a source", e.Edition)
		}
		if e.RuntimeMin == 0 && len(e.RuntimeSources) > 0 {
			return fmt.Errorf("edition %q: runtime sources without a runtime", e.Edition)
		}
		if err := checkSources("edition "+e.Edition, e.RuntimeSources); err != nil {
			return err
		}
		switch e.Confidence {
		case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		default:
			return fmt.Errorf("edition %q: unknown confidence %q", e.Edition, e.Confidence)
		}
		for i, d := range e.DifferenceFull {
			switch d.Change {
			case ChangeAdded, ChangeRemoved, ChangeChanged:
			default:
				return fmt.Errorf("edition %q difference %d: unknown change %q", e.Edition, i, d.Change)
			}
			if strings.TrimSpace(d.Text) == "" {
				return fmt.Errorf("edition %q difference %d: empty text", e.Edition, i)
			}
			if len(d.Sources) == 0 {
				return fmt.Errorf("edition %q difference %d: no source", e.Edition, i)
			}
			if err := checkSources(fmt.Sprintf("edition %s difference %d", e.Edition, i), d.Sources); err != nil {
				return err
			}
		}
		if e.ContentRating != nil && (e.ContentRating.Country == "" || e.ContentRating.Rating == "") {
			return fmt.Errorf("edition %q: content rating needs a country and a rating", e.Edition)
		}
	}
	return nil
}

// Validate checks a sync request's size and keys.
func (s SyncRequest) Validate() error {
	if len(s.Items) == 0 || len(s.Items) > MaxSyncItems {
		return fmt.Errorf("a sync request needs 1 to %d items, not %d", MaxSyncItems, len(s.Items))
	}
	for _, it := range s.Items {
		if err := it.Key().Validate(); err != nil {
			return err
		}
		if it.HaveVersion < 0 {
			return fmt.Errorf("%s: negative have_version", it.Key())
		}
	}
	return nil
}

// Validate checks a records request's size and keys.
func (r RecordsRequest) Validate() error {
	if len(r.Keys) == 0 || len(r.Keys) > MaxRecordKeys {
		return fmt.Errorf("a records request needs 1 to %d keys, not %d", MaxRecordKeys, len(r.Keys))
	}
	for _, k := range r.Keys {
		if err := k.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks a mistake report.
func (r Report) Validate() error {
	if err := r.Key.Validate(); err != nil {
		return err
	}
	if r.Edition != "" {
		if _, ok := editionlist.Lookup(r.Edition); !ok {
			return fmt.Errorf("edition %q is not in the Edition List", r.Edition)
		}
	}
	switch r.Field {
	case FieldExists, FieldRuntime, FieldDifferenceShort, FieldDifferenceFull, FieldContentRating, FieldOther:
	default:
		return fmt.Errorf("unknown field %q", r.Field)
	}
	if strings.TrimSpace(r.Message) == "" {
		return errors.New("a report needs a message")
	}
	if n := len([]rune(r.Message)); n > MaxReportMessage {
		return fmt.Errorf("message is %d characters; the limit is %d", n, MaxReportMessage)
	}
	return nil
}

// Validate checks a contribution before it's sent: an install ID, at most
// MaxContributeItems valid keys, and runtimes within range. Edition IDs aren't
// checked here; the service drops ones it doesn't know, so a server with an
// older or newer Edition List never fails a whole batch.
func (r ContributeRequest) Validate() error {
	if len(r.Install) < 16 || len(r.Install) > 128 {
		return errors.New("contribution needs an install ID of 16 to 128 characters")
	}
	if len(r.Items) == 0 || len(r.Items) > MaxContributeItems {
		return fmt.Errorf("contribution needs 1 to %d items, got %d", MaxContributeItems, len(r.Items))
	}
	for _, it := range r.Items {
		if err := it.Key().Validate(); err != nil {
			return err
		}
		if len(it.Editions) > MaxLookupEditions {
			return fmt.Errorf("%s: at most %d editions", it.Key(), MaxLookupEditions)
		}
		for _, e := range it.Editions {
			if e.Edition == "" {
				return fmt.Errorf("%s: an edition needs an ID", it.Key())
			}
			if e.RuntimeMin < 0 || e.RuntimeMin > MaxRuntimeMin {
				return fmt.Errorf("%s: runtime %d is out of range", it.Key(), e.RuntimeMin)
			}
		}
	}
	return nil
}
