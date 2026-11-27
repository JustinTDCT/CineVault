package cacheapi

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/pkg/editionlist"
)

// Path returns the key as used in a lookup path: "movie/tmdb/679".
func (k Key) Path() string { return k.Kind + "/tmdb/" + strconv.Itoa(k.TMDbID) }

// String returns the key as "movie:679".
func (k Key) String() string { return k.Kind + ":" + strconv.Itoa(k.TMDbID) }

// MediaType returns the Edition List media type for the key's kind.
func (k Key) MediaType() string {
	switch k.Kind {
	case KindMovie:
		return editionlist.Movie
	case KindTV:
		return editionlist.TV
	}
	return ""
}

// Validate checks the key's kind and ID.
func (k Key) Validate() error {
	if k.MediaType() == "" {
		return fmt.Errorf("unknown kind %q", k.Kind)
	}
	if k.TMDbID <= 0 {
		return fmt.Errorf("%s: TMDb ID must be positive", k.Kind)
	}
	return nil
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
		if len(r.Editions) > 0 || r.Base != nil {
			return errors.New("a withdrawn record has no editions or base metadata")
		}
		return nil
	default:
		return fmt.Errorf("unknown record status %q", r.Status)
	}
	if r.Base != nil && (r.Base.License == "" || r.Base.Title == "") {
		return errors.New("base metadata needs a license and a title")
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
