// Package catalog holds CineVault's library data types, shared by the database
// layer and the API without tying the API to a database driver.
package catalog

// MediaTypes are the kinds of library CineVault supports.
var MediaTypes = []string{"movie", "tv", "music", "audiobook", "book"}

// EditionType is one entry in the Edition List: a fixed edition name such as
// "Director's Cut" (see docs/specs/editions-and-versions.md §4).
type EditionType struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	ShortLabel  string   `json:"short_label"`
	MediaTypes  []string `json:"media_types"`
	Aliases     []string `json:"aliases"`
	SortOrder   int      `json:"sort_order"`
	IsDefault   bool     `json:"is_default"`
	Builtin     bool     `json:"builtin"`
	Hidden      bool     `json:"hidden"`
}

// AppliesTo reports whether the edition type can be used for the media type.
func (e EditionType) AppliesTo(mediaType string) bool {
	for _, m := range e.MediaTypes {
		if m == mediaType {
			return true
		}
	}
	return false
}

// ValidMediaType reports whether s is a supported media type.
func ValidMediaType(s string) bool {
	for _, m := range MediaTypes {
		if m == s {
			return true
		}
	}
	return false
}
