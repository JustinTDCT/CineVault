package cacheapi

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

func compact(b []byte) string {
	var buf bytes.Buffer
	if json.Compact(&buf, b) != nil {
		return string(b)
	}
	return buf.String()
}

func aliens(t *testing.T) Record {
	t.Helper()
	var r Record
	readGolden(t, "record_aliens.json", &r)
	return r
}

func TestRecordValidateRejects(t *testing.T) {
	cases := []struct {
		name   string
		change func(r *Record)
		want   string
	}{
		{"unknown kind", func(r *Record) { r.Key.Kind = "film" }, "unknown kind"},
		{"zero version", func(r *Record) { r.RecordVersion = 0 }, "record_version"},
		{"unknown edition", func(r *Record) { r.Editions[0].Edition = "rogue_cut" }, "not in the Edition List"},
		{"edition for another media type", func(r *Record) { r.Editions[0].Edition = "deluxe" }, "doesn't apply to movie"},
		{"out of order", func(r *Record) { r.Editions[0], r.Editions[2] = r.Editions[2], r.Editions[0] }, "out of Edition List order"},
		{"listed twice", func(r *Record) { r.Editions[1] = r.Editions[0] }, "listed twice"},
		{"runtime without a source", func(r *Record) { r.Editions[0].RuntimeSources = nil }, "runtime without a source"},
		{"sources without a runtime", func(r *Record) { r.Editions[0].RuntimeMin = 0 }, "without a runtime"},
		{"unknown source", func(r *Record) { r.Editions[2].RuntimeSources = []string{"s9"} }, "unknown source"},
		{"difference without a source", func(r *Record) { r.Editions[2].DifferenceFull[0].Sources = nil }, "no source"},
		{"bad change", func(r *Record) { r.Editions[2].DifferenceFull[0].Change = "moved" }, "unknown change"},
		{"bad confidence", func(r *Record) { r.Editions[0].Confidence = "sure" }, "unknown confidence"},
		{"withdrawn with facts", func(r *Record) { r.Editions[1].DifferenceShort = "x" }, "carries facts"},
		{"withdrawn record with editions", func(r *Record) { r.Status = StatusWithdrawn }, "withdrawn record"},
		{"base without license", func(r *Record) { r.Base.License = "" }, "license"},
		{"duplicate source", func(r *Record) { r.Sources[1].ID = "s1" }, "listed twice"},
	}
	if err := aliens(t).Validate(); err != nil {
		t.Fatalf("golden record invalid: %v", err)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := aliens(t)
			c.change(&r)
			err := r.Validate()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Validate() = %v, want an error containing %q", err, c.want)
			}
		})
	}
}

func TestTVDefaultEdition(t *testing.T) {
	r := Record{
		Key: Key{Kind: KindTV, TMDbID: 1399}, Status: StatusReady, RecordVersion: 1,
		Editions: []Edition{{Edition: "broadcast", Status: EditionPublished, Confidence: ConfidenceHigh}},
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.Editions[0].Edition = "theatrical" // movie only
	if err := r.Validate(); err == nil {
		t.Error("theatrical accepted for a TV show")
	}
}

func TestRequestLimits(t *testing.T) {
	items := make([]SyncItem, MaxSyncItems+1)
	for i := range items {
		items[i] = SyncItem{Kind: KindMovie, TMDbID: i + 1}
	}
	if err := (SyncRequest{Items: items}).Validate(); err == nil {
		t.Error("oversized sync accepted")
	}
	if err := (SyncRequest{Items: items[:MaxSyncItems]}).Validate(); err != nil {
		t.Error(err)
	}
	if err := (SyncRequest{}).Validate(); err == nil {
		t.Error("empty sync accepted")
	}
	keys := make([]Key, MaxRecordKeys+1)
	for i := range keys {
		keys[i] = Key{Kind: KindMovie, TMDbID: i + 1}
	}
	if err := (RecordsRequest{Keys: keys}).Validate(); err == nil {
		t.Error("oversized records request accepted")
	}
}

func TestReportValidate(t *testing.T) {
	ok := Report{Key: Key{Kind: KindMovie, TMDbID: 679}, Edition: "special_edition", Field: FieldRuntime, Message: "148 min"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []Report{
		{Key: ok.Key, Field: "plot", Message: "x"},
		{Key: ok.Key, Field: FieldOther, Message: "  "},
		{Key: ok.Key, Edition: "rogue_cut", Field: FieldOther, Message: "x"},
		{Key: ok.Key, Field: FieldOther, Message: strings.Repeat("é", MaxReportMessage+1)},
	}
	for i, r := range bad {
		if r.Validate() == nil {
			t.Errorf("bad report %d accepted", i)
		}
	}
}

func TestKeyPath(t *testing.T) {
	k := Key{Kind: KindMovie, TMDbID: 679}
	if k.Path() != "movie/tmdb/679" || k.String() != "movie:679" {
		t.Errorf("Path %q, String %q", k.Path(), k.String())
	}
}

func TestMusicVideoKeys(t *testing.T) {
	const id = "4a1b6c7e-2d3f-4e5a-9b8c-7d6e5f4a3b2c"
	k := Key{Kind: KindMusicVideo, MBID: id}
	if err := k.Validate(); err != nil {
		t.Fatal(err)
	}
	if k.Path() != "musicvideo/mb/"+id || k.String() != "musicvideo:"+id || k.Provider() != ProviderMusicBrainz || k.ProviderID() != id {
		t.Errorf("path %q string %q provider %q", k.Path(), k.String(), k.Provider())
	}
	if KeyFor(KindMusicVideo, id) != k || KeyFor(KindMovie, "679") != (Key{Kind: KindMovie, TMDbID: 679}) {
		t.Error("KeyFor doesn't invert ProviderID")
	}
	for _, bad := range []Key{
		{Kind: KindMusicVideo},
		{Kind: KindMusicVideo, MBID: "4A1B6C7E-2D3F-4E5A-9B8C-7D6E5F4A3B2C"}, // uppercase
		{Kind: KindMusicVideo, MBID: id, TMDbID: 5},
		{Kind: KindMusicVideo, MBID: "not-a-uuid"},
		{Kind: KindMovie, TMDbID: 679, MBID: id},
	} {
		if bad.Validate() == nil {
			t.Errorf("%+v passed", bad)
		}
	}
}

func TestCreditsValidation(t *testing.T) {
	var golden Record
	readGolden(t, "record_musicvideo.json", &golden)
	clone := func() Record {
		r := golden
		c := *golden.Credits
		c.Sources = map[string][]string{}
		for k, v := range golden.Credits.Sources {
			c.Sources[k] = v
		}
		r.Credits = &c
		return r
	}
	for name, mod := range map[string]func(*Record){
		"director without a source": func(r *Record) { delete(r.Credits.Sources, CreditDirector) },
		"unknown source":            func(r *Record) { r.Credits.Sources[CreditLabel] = []string{"s9"} },
		"sources for an empty value": func(r *Record) {
			r.Credits.Label = ""
		},
		"no artist":         func(r *Record) { r.Credits.Artist = " " },
		"credits on a film": func(r *Record) { r.Key = Key{Kind: KindMovie, TMDbID: 679} },
		"no credits":        func(r *Record) { r.Credits = nil },
		"bad confidence":    func(r *Record) { r.Credits.Confidence = "certain" },
	} {
		r := clone()
		mod(&r)
		if r.Validate() == nil {
			t.Errorf("%s: passed", name)
		}
	}
	r := clone()
	r.Credits.Director, r.Credits.Label = "", ""
	delete(r.Credits.Sources, CreditDirector)
	delete(r.Credits.Sources, CreditLabel)
	if err := r.Validate(); err != nil {
		t.Errorf("MusicBrainz facts only: %v", err)
	}
}
