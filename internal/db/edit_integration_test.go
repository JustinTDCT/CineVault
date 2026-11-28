package db

import (
	"context"
	"slices"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/scan"
	"github.com/JustinTDCT/CineVault/internal/tmdb"
)

// Locks and the library's preference decide what an automatic write may
// change: a field edited by hand stays, a locked title stays whole, and a
// library that prefers its files only lets lookups fill gaps.
func TestIntegrationLocksAndPreferLocal(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "Movies", MediaType: "movie", Paths: []string{t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	id, err := d.FindOrCreateTitle(ctx, lib.ID, "movie", scan.ParsedName{Title: "Alien", SortName: "alien", Year: 1979})
	if err != nil {
		t.Fatal(err)
	}
	get := func() catalog.Title {
		t.Helper()
		ts, err := d.LibraryTitles(ctx, lib.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range ts {
			if x.ID == id {
				return x
			}
		}
		t.Fatal("title gone")
		return catalog.Title{}
	}
	if _, err := d.FindOrCreateEdition(ctx, id, "theatrical", 0); err != nil {
		t.Fatal(err)
	}
	movie := func(title, overview, tagline string) tmdb.Movie {
		return tmdb.Movie{ID: 348, Title: title, Overview: overview, Tagline: tagline, ReleaseDate: "1979-05-25"}
	}

	// Lookups win by default.
	if err := d.SaveMatch(ctx, id, movie("Alien", "In space…", "")); err != nil {
		t.Fatal(err)
	}
	if ti := get(); ti.Overview != "In space…" || ti.TMDbID != 348 || ti.Locked || len(ti.LockedFields) != 0 {
		t.Fatalf("after match: %+v", ti)
	}

	// A person's edit is kept and locked; the rest still updates.
	name, year := "Alien: The Director's Cut", 1979
	if err := d.EditTitle(ctx, id, catalog.TitleEdit{Name: &name, Year: &year}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveMatch(ctx, id, movie("Alien", "No one can hear you scream.", "")); err != nil {
		t.Fatal(err)
	}
	ti := get()
	if ti.Name != name || ti.Overview != "No one can hear you scream." || !slices.Equal(ti.LockedFields, []string{"name", "year"}) {
		t.Fatalf("after edit and match: name %q overview %q locks %v", ti.Name, ti.Overview, ti.LockedFields)
	}

	// A locked title doesn't change at all.
	if _, err := d.SetTitleLocks(ctx, id, catalog.TitleLocks{Locked: true, Fields: ti.LockedFields}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveMatch(ctx, id, movie("Alien", "Changed again", "Tagline")); err != nil {
		t.Fatal(err)
	}
	if ti := get(); !ti.Locked || ti.Overview != "No one can hear you scream." || ti.Tagline != "" {
		t.Fatalf("locked title changed: %+v", ti)
	}

	// Unlocked, in a library that prefers its files: only gaps are filled.
	if _, err := d.SetTitleLocks(ctx, id, catalog.TitleLocks{}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetPreferLocalMetadata(ctx, lib.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveMatch(ctx, id, movie("Alien", "Changed again", "In space no one can hear you scream.")); err != nil {
		t.Fatal(err)
	}
	ti = get()
	if ti.Name != name || ti.Overview != "No one can hear you scream." || ti.Tagline != "In space no one can hear you scream." || ti.Locked {
		t.Fatalf("prefer local: name %q overview %q tagline %q", ti.Name, ti.Overview, ti.Tagline)
	}
	if libs, _ := d.Libraries(ctx); !libs[0].PreferLocalMetadata {
		t.Error("library setting not listed")
	}
	if err := d.EditTitle(ctx, 999999, catalog.TitleEdit{Name: &name}); err != catalog.ErrNotFound {
		t.Errorf("unknown title: %v", err)
	}
}
