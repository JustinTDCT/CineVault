// Package editionlist is CineVault's built-in Edition List: the fixed
// vocabulary of editions (cuts and releases) described in the Editions &
// Versions spec, §4.
//
// The media server stores the list in its database (migrations 0001 and
// 0004), where admins can hide built-in entries, add aliases and add custom
// entries. This package holds only the built-in entries as shipped, so that
// other programs, the CineVault Cache service above all, use exactly the same
// IDs without a database.
package editionlist

import (
	"slices"
	"strings"
	"unicode"
)

// Version identifies the built-in list. It goes up by one whenever a built-in
// entry is added or changed (version 1 was migration 0001, version 2 added
// 0004's music video editions), so a cache record can say which list it was
// written against.
const Version = 2

// Media types, as used by libraries and titles.
const (
	Movie      = "movie"
	TV         = "tv"
	Music      = "music"
	MusicVideo = "musicvideo"
	Audiobook  = "audiobook"
	Book       = "book"
	Comic      = "comic"
)

// MediaTypes lists every media type in display order.
var MediaTypes = []string{Movie, TV, Music, MusicVideo, Audiobook, Book, Comic}

// Entry is one built-in edition.
type Entry struct {
	ID          string   `json:"id"`           // stable key, e.g. "directors_cut"
	DisplayName string   `json:"display_name"` // e.g. "Director's Cut"
	ShortLabel  string   `json:"short_label"`  // for cards and badges, e.g. "DC"
	MediaTypes  []string `json:"media_types"`
	Aliases     []string `json:"aliases"` // keywords matched in file names and text
	SortOrder   int      `json:"sort_order"`
	IsDefault   bool     `json:"is_default"` // the edition a title gets when nothing else applies
}

// AppliesTo reports whether the entry can be used for the media type.
func (e Entry) AppliesTo(mediaType string) bool {
	return slices.Contains(e.MediaTypes, mediaType)
}

// Mentions reports whether text names this edition: whether one of its
// aliases or its display name appears in text as whole words, ignoring case
// and punctuation. "Director's Cut", "DIRECTORS CUT" and "directors.cut" all
// mention directors_cut; "Extended" does not mention theatrical.
func (e Entry) Mentions(text string) bool {
	hay := " " + normalize(text) + " "
	for _, phrase := range append([]string{e.DisplayName}, e.Aliases...) {
		if n := normalize(phrase); n != "" && strings.Contains(hay, " "+n+" ") {
			return true
		}
	}
	return false
}

// normalize lowercases s and turns every run of characters that aren't
// letters or digits into one space.
func normalize(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		} else {
			space = true
		}
	}
	return b.String()
}

// sorted is builtin in display order: sort order, then ID.
var sorted = func() []Entry {
	s := slices.Clone(builtin)
	slices.SortStableFunc(s, func(a, b Entry) int {
		if a.SortOrder != b.SortOrder {
			return a.SortOrder - b.SortOrder
		}
		return strings.Compare(a.ID, b.ID)
	})
	return s
}()

// All returns every built-in entry in display order. The caller may modify
// the result.
func All() []Entry {
	out := make([]Entry, len(sorted))
	for i, e := range sorted {
		out[i] = clone(e)
	}
	return out
}

// ForMediaType returns the entries that apply to a media type, in display order.
func ForMediaType(mediaType string) []Entry {
	var out []Entry
	for _, e := range sorted {
		if e.AppliesTo(mediaType) {
			out = append(out, clone(e))
		}
	}
	return out
}

// Lookup returns the entry with the given ID.
func Lookup(id string) (Entry, bool) {
	for _, e := range sorted {
		if e.ID == id {
			return clone(e), true
		}
	}
	return Entry{}, false
}

// Default returns the default edition for a media type: Theatrical for
// movies, Broadcast for TV and so on.
func Default(mediaType string) (Entry, bool) {
	for _, e := range sorted {
		if e.IsDefault && e.AppliesTo(mediaType) {
			return clone(e), true
		}
	}
	return Entry{}, false
}

// Position returns the entry's index in display order, for sorting editions
// the way cards and the picker list them. It returns -1 for an unknown ID.
func Position(id string) int {
	return slices.IndexFunc(sorted, func(e Entry) bool { return e.ID == id })
}

func clone(e Entry) Entry {
	e.MediaTypes = slices.Clone(e.MediaTypes)
	e.Aliases = slices.Clone(e.Aliases)
	return e
}
