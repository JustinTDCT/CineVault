package db

import (
	"context"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/match"
	"github.com/JustinTDCT/CineVault/internal/scan"
	"github.com/JustinTDCT/CineVault/pkg/cacheapi"
)

func TestIntegrationAlbumLookup(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "Music", MediaType: "music", Paths: []string{t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	tags := scan.ParsedName{Title: "the bridge", SortName: "bridge", Artist: "ACE OF BASE"}
	id, err := d.FindOrCreateTitle(ctx, lib.ID, "music", tags)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.FindOrCreateEdition(ctx, id, "original", 0); err != nil {
		t.Fatal(err)
	}
	items, err := d.LookupItems(ctx, lib.ID, false)
	if err != nil || len(items) != 1 || items[0].Artist != "ACE OF BASE" || items[0].MediaType != "music" {
		t.Fatalf("items %+v %v", items, err)
	}
	const rg = "1d0e3a7f-6c2b-4e8a-9f1d-2b3c4d5e6f70"
	found := match.Found{MBReleaseGroupID: rg, Name: "The Bridge", Artist: "Ace of Base", Year: 1995,
		Genres: []string{"Eurodance", "Pop"}, CoverURL: "https://coverartarchive.org/release-group/" + rg + "/front-500"}
	if err := d.SaveLookup(ctx, id, found); err != nil {
		t.Fatal(err)
	}
	titles, _ := d.LibraryTitles(ctx, lib.ID)
	ti := titles[0]
	if ti.Name != "The Bridge" || ti.Artist != "Ace of Base" || ti.Year != 1995 || ti.MBReleaseGroupID != rg ||
		ti.PosterURL != found.CoverURL || len(ti.Genres) != 2 || ti.MatchStatus != "matched" {
		t.Fatalf("after lookup: %+v", ti)
	}

	// The next scan still finds the album by what its tags say.
	if again, err := d.FindOrCreateTitle(ctx, lib.ID, "music", tags); err != nil || again != id {
		t.Fatalf("rescan split the album: %d vs %d (%v)", again, id, err)
	}

	// The album is now something the cache can know about.
	cacheItems, err := d.CacheItems(ctx)
	if err != nil || len(cacheItems) != 1 || cacheItems[0].Key != (cacheapi.Key{Kind: cacheapi.KindAlbum, MBID: rg}) {
		t.Fatalf("cache items %+v %v", cacheItems, err)
	}
	if k, ok, err := d.TitleCacheKey(ctx, id); err != nil || !ok || k.Kind != cacheapi.KindAlbum {
		t.Errorf("cache key %+v %v %v", k, ok, err)
	}

	// An edited artist survives the next lookup; an empty finding keeps the cover.
	artist := "Ace of Base (Sweden)"
	if err := d.EditTitle(ctx, id, catalog.TitleEdit{Artist: &artist}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveLookup(ctx, id, match.Found{MBReleaseGroupID: rg, Name: "The Bridge", Artist: "Ace of Base"}); err != nil {
		t.Fatal(err)
	}
	titles, _ = d.LibraryTitles(ctx, lib.ID)
	if ti := titles[0]; ti.Artist != artist || ti.PosterURL != found.CoverURL {
		t.Errorf("after edit and lookup: artist %q poster %q", ti.Artist, ti.PosterURL)
	}

	// Locked titles aren't offered for lookup, even when matching again.
	if _, err := d.SetTitleLocks(ctx, id, catalog.TitleLocks{Locked: true}); err != nil {
		t.Fatal(err)
	}
	if items, _ := d.LookupItems(ctx, lib.ID, true); len(items) != 0 {
		t.Errorf("locked title offered: %+v", items)
	}
}
