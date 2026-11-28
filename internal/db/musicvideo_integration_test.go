package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/match"
	"github.com/JustinTDCT/CineVault/internal/musicbrainz"
	"github.com/JustinTDCT/CineVault/internal/scan"
	"github.com/JustinTDCT/CineVault/pkg/cacheapi"
)

const takeOnMe = "4a1b6c7e-2d3f-4e5a-9b8c-7d6e5f4a3b2c"

type stubMB struct{}

func (stubMB) SearchRecordings(context.Context, string, string) ([]musicbrainz.Recording, error) {
	return []musicbrainz.Recording{{ID: takeOnMe, Title: "Take On Me", Artist: "a-ha", Score: 100,
		Releases: []musicbrainz.Release{{Title: "Hunting High and Low", Status: "Official", PrimaryType: "Album", GroupDate: "1985-06-01"}}}}, nil
}

func TestIntegrationMusicVideoCredits(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a-ha - Take On Me (Official Video).mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "Music Videos", MediaType: "musicvideo", Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (scan.Scanner{Store: d, Prober: stubProber{}}).Scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	res, err := (match.MusicVideoMatcher{Store: d, Source: stubMB{}}).MatchLibrary(ctx, lib.ID)
	if err != nil || res.Matched != 1 {
		t.Fatalf("match %+v %v", res, err)
	}
	if res, _ := (match.MusicVideoMatcher{Store: d, Source: stubMB{}}).MatchLibrary(ctx, lib.ID); res.Matched != 0 {
		t.Error("matched again")
	}

	items, err := d.CacheItems(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("items %+v %v", items, err)
	}
	it := items[0]
	if it.Key != (cacheapi.Key{Kind: cacheapi.KindMusicVideo, MBID: takeOnMe}) || len(it.Owned) != 1 || it.Owned[0].Edition != "official_video" || it.Owned[0].RuntimeMin == 0 {
		t.Fatalf("item %+v", it)
	}

	rec := cacheapi.Record{Key: it.Key, Status: cacheapi.StatusReady, RecordVersion: 2,
		Credits: &cacheapi.Credits{Song: "Take On Me", Artist: "a-ha", Album: "Hunting High and Low", Year: 1985,
			Label: "Warner Bros.", Director: "Steve Barron", Confidence: "high",
			Sources: map[string][]string{"song": {"s1"}, "artist": {"s1"}, "album": {"s1"}, "year": {"s1"}, "label": {"s1"}, "director": {"s1"}}},
		Sources: []cacheapi.Source{{ID: "s1", Name: "MusicBrainz"}}}
	if err := d.ApplyRecord(ctx, it.TitleID, rec); err != nil {
		t.Fatal(err)
	}
	titles, err := d.LibraryTitles(ctx, lib.ID)
	if err != nil || len(titles) != 1 {
		t.Fatalf("titles %+v %v", titles, err)
	}
	ti := titles[0]
	if ti.MBID != takeOnMe || ti.Album != "Hunting High and Low" || ti.AlbumYear != 1985 || ti.CacheState != "cached" {
		t.Errorf("title %+v", ti)
	}
	if c := ti.Credits; c == nil || c.Director != "Steve Barron" || c.Label != "Warner Bros." || c.Year != 1985 || c.Artist != "a-ha" {
		t.Errorf("credits %+v", ti.Credits)
	}

	// A withdrawn record takes the credits away.
	if err := d.ApplyRecord(ctx, it.TitleID, cacheapi.Record{Key: it.Key, Status: cacheapi.StatusWithdrawn, RecordVersion: 3}); err != nil {
		t.Fatal(err)
	}
	if titles, _ := d.LibraryTitles(ctx, lib.ID); titles[0].Credits != nil {
		t.Errorf("credits after withdrawal: %+v", titles[0].Credits)
	}
}
