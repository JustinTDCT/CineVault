package scan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/mediainfo"
)

type fakeVersion struct {
	id, editionID, size int64
	episodeID, last     int64
	missing             bool
	info                mediainfo.Info
}

type fakeStore struct {
	nextID   int64
	titles   map[string]int64 // "alien|1979" -> id
	editions map[string]int64 // "titleID|edition" -> id
	runtimes map[int64]int
	versions map[string]*fakeVersion
	episodes map[string]int64 // "titleID|S1E3" -> id
	epNames  map[int64]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{titles: map[string]int64{}, editions: map[string]int64{}, runtimes: map[int64]int{}, versions: map[string]*fakeVersion{}, episodes: map[string]int64{}, epNames: map[int64]string{}}
}

func (f *fakeStore) id() int64 { f.nextID++; return f.nextID }

func (f *fakeStore) EditionTypes(context.Context) ([]catalog.EditionType, error) {
	types := append([]catalog.EditionType{}, movieTypes...)
	types[0].IsDefault = true // theatrical
	types[0].MediaTypes = []string{"movie"}
	for i := range types[1:] {
		types[i+1].MediaTypes = []string{"movie"}
	}
	types = append(types, catalog.EditionType{ID: "official_video", DisplayName: "Official Video", MediaTypes: []string{"musicvideo"}, IsDefault: true},
		catalog.EditionType{ID: "acoustic", DisplayName: "Acoustic/Unplugged", MediaTypes: []string{"musicvideo"}, Aliases: []string{"mtv unplugged", "unplugged"}})
	types = append(types, catalog.EditionType{ID: "broadcast", DisplayName: "Broadcast", MediaTypes: []string{"tv"}, IsDefault: true})
	return types, nil
}

func (f *fakeStore) FindOrCreateEpisode(_ context.Context, titleID int64, season, episode int, name string) (int64, error) {
	key := fmt.Sprintf("%d|S%dE%d", titleID, season, episode)
	if id, ok := f.episodes[key]; ok {
		return id, nil
	}
	f.episodes[key] = f.id()
	f.epNames[f.episodes[key]] = name
	return f.episodes[key], nil
}

func (f *fakeStore) KnownVersions(context.Context, int64) (map[string]KnownVersion, error) {
	out := map[string]KnownVersion{}
	for p, v := range f.versions {
		out[p] = KnownVersion{ID: v.id, Size: v.size}
	}
	return out, nil
}

func (f *fakeStore) FindOrCreateTitle(_ context.Context, _ int64, _ string, p ParsedName) (int64, error) {
	key := fmt.Sprintf("%s|%d", strings.ToLower(p.Title), p.Year)
	if p.Artist != "" {
		key += "|" + strings.ToLower(p.Artist)
	}
	if id, ok := f.titles[key]; ok {
		return id, nil
	}
	f.titles[key] = f.id()
	return f.titles[key], nil
}

func (f *fakeStore) FindOrCreateEdition(_ context.Context, titleID int64, ed string, runtime int) (int64, error) {
	key := fmt.Sprintf("%d|%s", titleID, ed)
	if id, ok := f.editions[key]; ok {
		return id, nil
	}
	id := f.id()
	f.editions[key], f.runtimes[id] = id, runtime
	return id, nil
}

func (f *fakeStore) UpsertVersion(_ context.Context, editionID int64, v NewVersion) error {
	if old, ok := f.versions[v.Path]; ok {
		old.editionID, old.size, old.missing, old.info = editionID, v.SizeBytes, false, v.Info
		old.episodeID, old.last = v.EpisodeID, int64(v.LastEpisode)
		return nil
	}
	f.versions[v.Path] = &fakeVersion{id: f.id(), editionID: editionID, size: v.SizeBytes, info: v.Info, episodeID: v.EpisodeID, last: int64(v.LastEpisode)}
	return nil
}

func (f *fakeStore) SetMissing(_ context.Context, ids []int64, missing bool) error {
	for _, id := range ids {
		for _, v := range f.versions {
			if v.id == id {
				v.missing = missing
			}
		}
	}
	return nil
}

// editionsOf returns "title (year): edition, edition" for checking grouping.
func (f *fakeStore) summary() []string {
	byID := map[int64]string{}
	for k, id := range f.titles {
		byID[id] = k
	}
	groups := map[string][]string{}
	for k := range f.editions {
		var tid int64
		var ed string
		fmt.Sscanf(strings.Replace(k, "|", " ", 1), "%d %s", &tid, &ed)
		groups[byID[tid]] = append(groups[byID[tid]], ed)
	}
	var out []string
	for t, eds := range groups {
		sort.Strings(eds)
		out = append(out, t+": "+strings.Join(eds, ","))
	}
	sort.Strings(out)
	return out
}

type fakeProber struct {
	calls  int
	failOn string
}

func (p *fakeProber) Probe(_ context.Context, path string) (mediainfo.Info, error) {
	p.calls++
	if p.failOn != "" && strings.Contains(path, p.failOn) {
		return mediainfo.Info{}, errors.New("ffprobe: Invalid data found when processing input")
	}
	return mediainfo.Info{Container: "mkv", VideoCodec: "hevc", Width: 1920, Height: 800, DurationSeconds: 7020.4}, nil
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanAlienLibrary(t *testing.T) {
	dir := t.TempDir()
	remastered := filepath.Join(dir, "Alien - Remastered (1979) [Bluray-1080p x265].mkv")
	dc := filepath.Join(dir, "Alien {edition-Director's Cut} (1979) [1080p-MP4].mkv")
	write(t, remastered, "remastered")
	write(t, dc, "directors cut")
	// Things the scanner must skip.
	write(t, filepath.Join(dir, "Alien (1979).nfo"), "x")
	write(t, filepath.Join(dir, "sample.mkv"), "x")
	write(t, filepath.Join(dir, ".hidden", "Other (2000).mkv"), "x")
	write(t, filepath.Join(dir, ".DS_Store.mkv"), "x")

	store, prober := newFakeStore(), &fakeProber{}
	s := Scanner{Store: store, Prober: prober}
	lib := catalog.Library{ID: 1, MediaType: "movie", Paths: []string{dir}}

	res, err := s.Scan(context.Background(), lib)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesSeen != 2 || res.Added != 2 || len(res.Errors) != 0 {
		t.Fatalf("first scan: %+v", res)
	}
	if got := store.summary(); len(got) != 1 || got[0] != "alien|1979: directors_cut,remastered" {
		t.Errorf("grouping = %v, want one title with two editions", got)
	}
	for _, rt := range store.runtimes {
		if rt != 7020 {
			t.Errorf("edition runtime = %d, want 7020", rt)
		}
	}

	// A second scan changes nothing and doesn't probe again.
	prober.calls = 0
	res, err = s.Scan(context.Background(), lib)
	if err != nil || res.Unchanged != 2 || res.Added+res.Updated != 0 || prober.calls != 0 {
		t.Errorf("rescan: %+v probes=%d err=%v", res, prober.calls, err)
	}

	// A changed file is re-read; a deleted file is marked missing.
	write(t, dc, "directors cut, re-encoded")
	if err := os.Remove(remastered); err != nil {
		t.Fatal(err)
	}
	res, err = s.Scan(context.Background(), lib)
	if err != nil || res.Updated != 1 || res.Missing != 1 {
		t.Errorf("after changes: %+v err=%v", res, err)
	}
	if !store.versions[remastered].missing || store.versions[dc].missing {
		t.Error("missing flags wrong")
	}

	// The file comes back: no longer missing.
	write(t, remastered, "remastered")
	if _, err := s.Scan(context.Background(), lib); err != nil {
		t.Fatal(err)
	}
	if store.versions[remastered].missing {
		t.Error("returned file still marked missing")
	}
}

func TestScanReportsBadFilesAndFolders(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Good (2001).mkv"), "ok")
	write(t, filepath.Join(dir, "Broken (2002).mkv"), "bad")

	store := newFakeStore()
	s := Scanner{Store: store, Prober: &fakeProber{failOn: "Broken"}}
	lib := catalog.Library{ID: 1, MediaType: "movie", Paths: []string{dir, filepath.Join(dir, "does-not-exist")}}

	res, err := s.Scan(context.Background(), lib)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 || len(res.Errors) != 2 {
		t.Fatalf("got %+v", res)
	}
	msgs := res.Errors[0].Error + " | " + res.Errors[1].Error
	if !strings.Contains(msgs, "Invalid data") || !strings.Contains(msgs, "folder or file not found") {
		t.Errorf("errors = %v", res.Errors)
	}
}

func TestScanRejectsUnsupportedLibraries(t *testing.T) {
	s := Scanner{Store: newFakeStore(), Prober: &fakeProber{}}
	if _, err := s.Scan(context.Background(), catalog.Library{MediaType: "music"}); err == nil {
		t.Error("music libraries should be rejected for now")
	}
}

func TestScanStopsWhenCancelled(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "A (2001).mkv"), "a")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := Scanner{Store: newFakeStore(), Prober: &fakeProber{}}
	if _, err := s.Scan(ctx, catalog.Library{MediaType: "movie", Paths: []string{dir}}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestFingerprint(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("a", 200<<10)
	write(t, filepath.Join(dir, "a"), big)
	write(t, filepath.Join(dir, "b"), big)                               // same content, other name
	write(t, filepath.Join(dir, "c"), big[:len(big)-1]+"z")              // differs in the last byte
	write(t, filepath.Join(dir, "d"), big[:100<<10]+"z"+big[100<<10+1:]) // differs in the middle only
	write(t, filepath.Join(dir, "small"), "tiny")

	fp := func(n string) string {
		s, err := Fingerprint(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	if fp("a") != fp("b") {
		t.Error("identical files should match")
	}
	if fp("a") == fp("c") {
		t.Error("a change at the end should change the fingerprint")
	}
	if fp("a") != fp("d") {
		t.Error("only the ends are hashed; the middle is covered by size")
	}
	if !strings.HasPrefix(fp("small"), "v1:4:") {
		t.Errorf("small = %s", fp("small"))
	}
}

func TestScanTVLibrary(t *testing.T) {
	dir := t.TempDir()
	show := filepath.Join(dir, "3 Body Problem", "Season 1")
	for _, n := range []string{"01 - Countdown", "02 - Red Coast", "03 - Destroyer of Worlds"} {
		base := "3 Body Problem - S01E" + n + " [WEBRip-1080p, x265 EAC3 Atmos]"
		write(t, filepath.Join(show, base+".mkv"), "video "+n)
		write(t, filepath.Join(show, base+".en.srt"), "subs") // sidecar subtitles are skipped
	}
	write(t, filepath.Join(dir, "Lost", "Season 1", "Lost - S01E01-E02 - Pilot.mkv"), "two episodes")
	write(t, filepath.Join(dir, "Lost", "Season 1", "Lost - Behind the Scenes.mkv"), "extra")

	store := newFakeStore()
	sc := Scanner{Store: store, Prober: &fakeProber{}}
	res, err := sc.Scan(context.Background(), catalog.Library{ID: 1, MediaType: "tv", Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesSeen != 5 || res.Added != 4 || len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Error, "season and episode") {
		t.Fatalf("result = %+v", res)
	}
	if len(store.titles) != 2 || store.titles["3 body problem|0"] == 0 || store.titles["lost|0"] == 0 {
		t.Errorf("titles = %v", store.titles)
	}
	if len(store.episodes) != 4 {
		t.Errorf("episodes = %v", store.episodes)
	}
	tbp := store.titles["3 body problem|0"]
	if id := store.episodes[fmt.Sprintf("%d|S1E3", tbp)]; store.epNames[id] != "Destroyer of Worlds" {
		t.Errorf("episode name = %q", store.epNames[id])
	}
	pilot := store.versions[filepath.Join(dir, "Lost", "Season 1", "Lost - S01E01-E02 - Pilot.mkv")]
	if pilot == nil || pilot.episodeID == 0 || pilot.last != 2 {
		t.Errorf("multi-episode file = %+v", pilot)
	}
	// Every file of a show shares the show's default edition.
	eds := map[int64]bool{}
	for _, v := range store.versions {
		eds[v.editionID] = true
	}
	if len(eds) != 2 {
		t.Errorf("editions used = %v, want one per show", eds)
	}
}

func TestScanMusicVideos(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"Alice In Chains - No Excuses (MTV Unplugged).mp4", "Alice In Chains - No Excuses.mp4",
		"Alice Cooper - Poison.mp4", "Bell Biv DeVoe - Poison.mp4"} {
		write(t, filepath.Join(dir, n), n)
	}
	store := newFakeStore()
	res, err := Scanner{Store: store, Prober: &fakeProber{}}.Scan(context.Background(), catalog.Library{ID: 3, MediaType: "musicvideo", Paths: []string{dir}})
	if err != nil || res.Added != 4 || len(res.Errors) != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	// Two editions of one song; two different songs that share a name.
	if len(store.titles) != 3 {
		t.Errorf("titles = %v", store.titles)
	}
	eds := map[string]bool{}
	for k := range store.editions {
		eds[k[strings.Index(k, "|")+1:]] = true
	}
	if !eds["acoustic"] || !eds["official_video"] {
		t.Errorf("editions = %v", store.editions)
	}
}
