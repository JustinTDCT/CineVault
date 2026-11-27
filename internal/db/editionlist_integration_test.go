package db

import (
	"context"
	"slices"
	"testing"

	"github.com/JustinTDCT/CineVault/pkg/editionlist"
)

// TestIntegrationEditionListMatchesPackage checks that the built-in entries the
// migrations create are exactly pkg/editionlist, which the CineVault Cache
// service uses. A migration that changes a built-in entry must change the
// package (and its Version) in the same commit.
func TestIntegrationEditionListMatchesPackage(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	types, err := d.EditionTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}

	want := editionlist.All()
	var got []editionlist.Entry
	for _, e := range types {
		if !e.Builtin {
			continue
		}
		got = append(got, editionlist.Entry{
			ID: e.ID, DisplayName: e.DisplayName, ShortLabel: e.ShortLabel,
			MediaTypes: e.MediaTypes, Aliases: e.Aliases,
			SortOrder: e.SortOrder, IsDefault: e.IsDefault,
		})
	}
	if len(got) != len(want) {
		t.Fatalf("migrations create %d built-in entries, pkg/editionlist has %d", len(got), len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.ID != w.ID || g.DisplayName != w.DisplayName || g.ShortLabel != w.ShortLabel ||
			g.SortOrder != w.SortOrder || g.IsDefault != w.IsDefault ||
			!sameSet(g.MediaTypes, w.MediaTypes) || !slices.Equal(g.Aliases, w.Aliases) {
			t.Errorf("entry %d differs:\nmigrations: %+v\npackage:    %+v", i, g, w)
		}
	}
}

// sameSet compares media types ignoring order: migration 0004 appends to
// arrays, so the stored order is an accident of history.
func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
