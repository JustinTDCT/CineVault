package comic

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var png1 = []byte("\x89PNG\r\n\x1a\nfake page")

const comicInfo = `<?xml version="1.0"?>
<ComicInfo><Series>Aliens</Series><Number>2</Number><Title>Book One, Part 2</Title>
<Summary>The Marines go back.</Summary><Year>1988</Year><Publisher>Dark Horse</Publisher></ComicInfo>`

func pageNames() []string {
	return []string{"Aliens 02/page10.png", "Aliens 02/page2.png", "Aliens 02/page1.png", "__MACOSX/._page1.png", "Aliens 02/.hidden.png", "ComicInfo.xml", "Aliens 02/notes.txt"}
}

func makeCBZ(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "Aliens - Book One - 2.cbz")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	for _, n := range pageNames() {
		w, _ := z.Create(n)
		if n == "ComicInfo.xml" {
			w.Write([]byte(comicInfo))
		} else {
			w.Write(append(append([]byte{}, png1...), n...))
		}
	}
	z.Close()
	f.Close()
	return p
}

func TestCBZ(t *testing.T) {
	r := &Reader{CacheDir: t.TempDir()}
	p := makeCBZ(t)
	ctx := context.Background()
	pages, info, err := r.Scan(ctx, p)
	if err != nil || pages != 3 || info == nil || info.Series != "Aliens" || info.Number != "2" || info.Year != 1988 {
		t.Fatalf("scan: %d %+v %v", pages, info, err)
	}
	names, _ := r.Pages(ctx, p)
	if len(names) != 3 || names[0] != "Aliens 02/page1.png" || names[2] != "Aliens 02/page10.png" {
		t.Errorf("pages = %v", names)
	}
	b, mime, err := r.Page(ctx, p, "v1", 1)
	if err != nil || mime != "image/png" || !bytes.HasSuffix(b, []byte("page2.png")) {
		t.Errorf("page 2: %q %s %v", b, mime, err)
	}
	if _, _, err := r.Page(ctx, p, "v1", 3); err == nil {
		t.Error("page past the end should fail")
	}
}

// CB7 goes through bsdtar and the extraction cache.
func TestCB7WithBsdtar(t *testing.T) {
	bin, err := exec.LookPath("bsdtar")
	if err != nil {
		t.Skip("bsdtar not installed")
	}
	src := t.TempDir()
	for _, n := range pageNames() {
		fp := filepath.Join(src, filepath.FromSlash(n))
		os.MkdirAll(filepath.Dir(fp), 0o755)
		content := append(append([]byte{}, png1...), n...)
		if n == "ComicInfo.xml" {
			content = []byte(comicInfo)
		}
		os.WriteFile(fp, content, 0o644)
	}
	// A symlink to a file outside the archive must not be served as a page.
	os.Symlink("/etc/hostname", filepath.Join(src, "Aliens 02", "page0.png"))
	p := filepath.Join(t.TempDir(), "Aliens - Book One - 2.cb7")
	cmd := exec.Command(bin, "--format", "7zip", "-cf", p, "-C", src, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make cb7: %v %s", err, out)
	}
	cache := t.TempDir()
	r := &Reader{CacheDir: cache, KeepExtracted: 1}
	ctx := context.Background()
	pages, info, err := r.Scan(ctx, p)
	if err != nil || info == nil || info.Title != "Book One, Part 2" {
		t.Fatalf("scan: %d %+v %v", pages, info, err)
	}
	b, mime, err := r.Page(ctx, p, "v7-1", 2)
	if err != nil || mime != "image/png" || !bytes.HasSuffix(b, []byte("page10.png")) {
		t.Errorf("page 3: %q %v", b, err)
	}
	if b, _, _ := r.Page(ctx, p, "v7-1", 0); bytes.Contains(b, []byte("hostname")) || !bytes.HasSuffix(b, []byte("page1.png")) {
		t.Errorf("first page should be page1, not the symlink: %q", b)
	}
	// A second archive pushes the first out of the cache (limit 1).
	if _, _, err := r.Page(ctx, p, "v8-1", 0); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(cache)
	if len(entries) != 1 || entries[0].Name() != "v8-1" {
		t.Errorf("cache = %v", entries)
	}
}

func TestNoBsdtar(t *testing.T) {
	r := &Reader{Bsdtar: "", CacheDir: t.TempDir()}
	t.Setenv("PATH", t.TempDir())
	p := filepath.Join(t.TempDir(), "x.cbr")
	os.WriteFile(p, []byte("Rar!\x1a\x07\x00"), 0o644)
	if _, _, err := r.Scan(context.Background(), p); err != ErrNoTool {
		t.Errorf("err = %v", err)
	}
}

func TestNaturalOrder(t *testing.T) {
	in := []string{"p10.jpg", "p9.jpg", "P1.jpg", "p01b.jpg", "cover.jpg"}
	got := imagesOf(in)
	want := []string{"cover.jpg", "P1.jpg", "p01b.jpg", "p9.jpg", "p10.jpg"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v", got)
		}
	}
}
