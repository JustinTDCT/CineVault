// Package comic reads comic book archives: CBZ natively, and CBR, CB7 and
// CBT with bsdtar (libarchive-tools). Pages are the archive's images in
// natural order ("page2" before "page10").
package comic

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Extensions are the comic file types, by extension.
var Extensions = map[string]bool{".cbz": true, ".cbr": true, ".cb7": true, ".cbt": true}

var imageTypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp",
	".gif": "image/gif", ".avif": "image/avif", ".jxl": "image/jxl",
}

// ErrNoTool means a CBR/CB7 file needs bsdtar and it isn't installed.
var ErrNoTool = errors.New("reading CBR and CB7 comics needs bsdtar: sudo apt install libarchive-tools")

// Info is a ComicInfo.xml file (the format ComicRack and most taggers write).
type Info struct {
	Series    string  `xml:"Series"`
	Number    string  `xml:"Number"`
	Title     string  `xml:"Title"`
	Summary   string  `xml:"Summary"`
	Year      int     `xml:"Year"`
	Volume    int     `xml:"Volume"`
	Writer    string  `xml:"Writer"`
	Publisher string  `xml:"Publisher"`
	PageCount int     `xml:"PageCount"`
	Count     int     `xml:"Count"`
	Rating    float64 `xml:"CommunityRating"`
}

// Reader opens comic archives. Bsdtar is the bsdtar binary ("" to look it
// up); CacheDir holds extracted CBR/CB7 files so pages load quickly.
type Reader struct {
	Bsdtar   string
	CacheDir string
	// KeepExtracted is how many extracted archives to keep (default 12).
	KeepExtracted int

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func (r *Reader) bsdtar() (string, error) {
	if r.Bsdtar != "" {
		return r.Bsdtar, nil
	}
	p, err := exec.LookPath("bsdtar")
	if err != nil {
		return "", ErrNoTool
	}
	return p, nil
}

// Available reports whether CBR/CB7 support (bsdtar) is installed.
func (r *Reader) Available() bool {
	_, err := r.bsdtar()
	return err == nil
}

func isZip(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	b := make([]byte, 4)
	_, _ = io.ReadFull(f, b)
	return bytes.Equal(b, []byte("PK\x03\x04"))
}

// Scan reads what a scan needs: the page count and ComicInfo.xml, if any.
func (r *Reader) Scan(ctx context.Context, p string) (pages int, info *Info, err error) {
	names, err := r.list(ctx, p)
	if err != nil {
		return 0, nil, err
	}
	pages = len(imagesOf(names))
	for _, n := range names {
		if strings.EqualFold(path.Base(n), "ComicInfo.xml") {
			b, err := r.readEntry(ctx, p, n)
			if err == nil {
				var ci Info
				d := xml.NewDecoder(bytes.NewReader(b))
				d.Strict = false
				d.CharsetReader = func(_ string, rd io.Reader) (io.Reader, error) { return rd, nil }
				if d.Decode(&ci) == nil {
					info = &ci
				}
			}
			break
		}
	}
	return pages, info, nil
}

// Pages lists the page images in reading order.
func (r *Reader) Pages(ctx context.Context, p string) ([]string, error) {
	names, err := r.list(ctx, p)
	if err != nil {
		return nil, err
	}
	return imagesOf(names), nil
}

// Page returns page i (from 0) and its MIME type. key names the file in the
// extraction cache (for example "v12-345678") so a changed file gets a new one.
func (r *Reader) Page(ctx context.Context, p, key string, i int) ([]byte, string, error) {
	if isZip(p) {
		names, err := r.list(ctx, p)
		if err != nil {
			return nil, "", err
		}
		pages := imagesOf(names)
		if i < 0 || i >= len(pages) {
			return nil, "", os.ErrNotExist
		}
		b, err := r.readEntry(ctx, p, pages[i])
		return b, imageTypes[strings.ToLower(path.Ext(pages[i]))], err
	}
	dir, err := r.extract(ctx, p, key)
	if err != nil {
		return nil, "", err
	}
	var names []string
	_ = filepath.WalkDir(dir, func(fp string, d os.DirEntry, err error) error {
		// Regular files only: a symlink in an archive mustn't point the
		// reader at files outside it.
		if err == nil && d.Type().IsRegular() {
			rel, _ := filepath.Rel(dir, fp)
			names = append(names, filepath.ToSlash(rel))
		}
		return nil
	})
	pages := imagesOf(names)
	if i < 0 || i >= len(pages) {
		return nil, "", os.ErrNotExist
	}
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(pages[i])))
	return b, imageTypes[strings.ToLower(path.Ext(pages[i]))], err
}

func (r *Reader) list(ctx context.Context, p string) ([]string, error) {
	if isZip(p) {
		z, err := zip.OpenReader(p)
		if err != nil {
			return nil, fmt.Errorf("open comic: %w", err)
		}
		defer z.Close()
		var names []string
		for _, f := range z.File {
			if !f.FileInfo().IsDir() {
				names = append(names, f.Name)
			}
		}
		return names, nil
	}
	bin, err := r.bsdtar()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-tf", p).Output()
	if err != nil {
		return nil, fmt.Errorf("read comic archive: %w", exitMessage(err))
	}
	var names []string
	for _, n := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if n = strings.TrimSpace(n); n != "" && !strings.HasSuffix(n, "/") {
			names = append(names, n)
		}
	}
	return names, nil
}

func (r *Reader) readEntry(ctx context.Context, p, name string) ([]byte, error) {
	if isZip(p) {
		z, err := zip.OpenReader(p)
		if err != nil {
			return nil, err
		}
		defer z.Close()
		for _, f := range z.File {
			if f.Name == name {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(io.LimitReader(rc, 64<<20))
			}
		}
		return nil, os.ErrNotExist
	}
	bin, err := r.bsdtar()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-xOf", p, name).Output()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, exitMessage(err))
	}
	return out, nil
}

// extract unpacks an archive into the cache once, keeping only the most
// recently used ones.
func (r *Reader) extract(ctx context.Context, p, key string) (string, error) {
	bin, err := r.bsdtar()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(r.CacheDir, key)
	lock := r.lockFor(key)
	lock.Lock()
	defer lock.Unlock()
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		now := time.Now()
		_ = os.Chtimes(dir, now, now)
		return dir, nil
	}
	tmp := dir + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-xf", p, "-C", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf("unpack comic: %v %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.Rename(tmp, dir); err != nil {
		return "", err
	}
	now := time.Now()
	_ = os.Chtimes(dir, now, now)
	r.prune(key)
	return dir, nil
}

func (r *Reader) lockFor(key string) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locks == nil {
		r.locks = map[string]*sync.Mutex{}
	}
	if l, ok := r.locks[key]; ok {
		return l
	}
	l := &sync.Mutex{}
	r.locks[key] = l
	return l
}

// prune removes the least recently used extracted archives beyond the limit.
func (r *Reader) prune(keep string) {
	limit := r.KeepExtracted
	if limit <= 0 {
		limit = 12
	}
	entries, err := os.ReadDir(r.CacheDir)
	if err != nil {
		return
	}
	type dirInfo struct {
		name string
		mod  time.Time
	}
	var dirs []dirInfo
	for _, e := range entries {
		if !e.IsDir() || strings.HasSuffix(e.Name(), ".tmp") || e.Name() == keep {
			continue
		}
		if info, err := e.Info(); err == nil {
			dirs = append(dirs, dirInfo{e.Name(), info.ModTime()})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].mod.After(dirs[j].mod) })
	for i, d := range dirs { // keep itself counts as one of the limit
		if i >= limit-1 {
			_ = os.RemoveAll(filepath.Join(r.CacheDir, d.name))
		}
	}
}

// imagesOf picks the page images from an archive listing, in natural order,
// skipping macOS metadata and hidden files.
func imagesOf(names []string) []string {
	var out []string
	for _, n := range names {
		base := path.Base(n)
		if strings.HasPrefix(n, "__MACOSX/") || strings.HasPrefix(base, ".") {
			continue
		}
		if _, ok := imageTypes[strings.ToLower(path.Ext(n))]; ok {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return naturalLess(out[i], out[j]) })
	return out
}

// naturalLess compares names with digit runs as numbers, ignoring case.
func naturalLess(a, b string) bool {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	i, j := 0, 0
	for i < len(ra) && j < len(rb) {
		if unicode.IsDigit(ra[i]) && unicode.IsDigit(rb[j]) {
			si := i
			for i < len(ra) && unicode.IsDigit(ra[i]) {
				i++
			}
			sj := j
			for j < len(rb) && unicode.IsDigit(rb[j]) {
				j++
			}
			na := strings.TrimLeft(string(ra[si:i]), "0")
			nb := strings.TrimLeft(string(rb[sj:j]), "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		if ra[i] != rb[j] {
			return ra[i] < rb[j]
		}
		i++
		j++
	}
	return len(ra)-i < len(rb)-j
}

func exitMessage(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return errors.New(strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}
