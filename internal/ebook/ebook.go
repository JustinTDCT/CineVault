// Package ebook reads the title, author, series and cover of EPUB, MOBI and
// AZW3 files without any outside tools.
package ebook

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Meta is what a book file says about itself. Empty fields weren't found.
type Meta struct {
	Format      string // "epub", "azw3", "mobi", "pdf", ...
	Title       string
	Authors     []string
	Description string
	Date        string // as written, e.g. "2018-04-05" or "1977"
	Series      string
	SeriesIndex float64
	Language    string
}

// ErrNoCover means the file has no cover image.
var ErrNoCover = errors.New("no cover image in this book")

// Formats are the book file types CineVault lists, by extension.
var Formats = map[string]string{
	".epub": "epub", ".azw3": "azw3", ".azw": "azw3", ".mobi": "mobi", ".pdf": "pdf", ".fb2": "fb2",
}

// Read returns a book file's metadata. Formats it can't look inside (PDF,
// FB2) return just the format.
func Read(p string) (Meta, error) {
	m := Meta{Format: Formats[strings.ToLower(filepath.Ext(p))]}
	switch m.Format {
	case "epub":
		return readEPUB(p, m)
	case "azw3", "mobi":
		return readMOBI(p, m)
	case "":
		return m, fmt.Errorf("%s isn't a book format CineVault reads", filepath.Ext(p))
	}
	return m, nil
}

// Cover returns a book's cover image and its MIME type.
func Cover(p string) ([]byte, string, error) {
	switch Formats[strings.ToLower(filepath.Ext(p))] {
	case "epub":
		return epubCover(p)
	case "azw3", "mobi":
		return mobiCover(p)
	}
	return nil, "", ErrNoCover
}

// ---------- EPUB ----------

type opf struct {
	Metadata struct {
		Titles   []string `xml:"title"`
		Creators []struct {
			Name string `xml:",chardata"`
			Role string `xml:"role,attr"`
			ID   string `xml:"id,attr"`
		} `xml:"creator"`
		Description string   `xml:"description"`
		Date        []string `xml:"date"`
		Language    string   `xml:"language"`
		Metas       []struct {
			Name     string `xml:"name,attr"`
			Content  string `xml:"content,attr"`
			Property string `xml:"property,attr"`
			Refines  string `xml:"refines,attr"`
			Value    string `xml:",chardata"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest []struct {
		ID         string `xml:"id,attr"`
		Href       string `xml:"href,attr"`
		MediaType  string `xml:"media-type,attr"`
		Properties string `xml:"properties,attr"`
	} `xml:"manifest>item"`
}

func openOPF(p string) (*zip.ReadCloser, *opf, string, error) {
	z, err := zip.OpenReader(p)
	if err != nil {
		return nil, nil, "", fmt.Errorf("open EPUB: %w", err)
	}
	var container struct {
		Rootfiles []struct {
			Path string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := readXML(&z.Reader, "META-INF/container.xml", &container); err != nil || len(container.Rootfiles) == 0 {
		z.Close()
		return nil, nil, "", errors.New("EPUB has no META-INF/container.xml")
	}
	root := container.Rootfiles[0].Path
	var o opf
	if err := readXML(&z.Reader, root, &o); err != nil {
		z.Close()
		return nil, nil, "", fmt.Errorf("EPUB package file: %w", err)
	}
	return z, &o, root, nil
}

func readEPUB(p string, m Meta) (Meta, error) {
	z, o, _, err := openOPF(p)
	if err != nil {
		return m, err
	}
	defer z.Close()
	md := o.Metadata
	if len(md.Titles) > 0 {
		m.Title = clean(md.Titles[0])
	}
	for _, c := range md.Creators {
		role := strings.ToLower(c.Role)
		for _, mt := range md.Metas { // EPUB 3 puts the role in a refining meta
			if c.ID != "" && mt.Refines == "#"+c.ID && mt.Property == "role" {
				role = strings.ToLower(clean(mt.Value))
			}
		}
		if (role == "" || role == "aut") && clean(c.Name) != "" {
			m.Authors = append(m.Authors, clean(c.Name))
		}
	}
	m.Description = stripTags(md.Description)
	if len(md.Date) > 0 {
		m.Date = clean(md.Date[0])
	}
	m.Language = clean(md.Language)
	collections := map[string]string{} // id -> name, for EPUB 3 collections
	for _, mt := range md.Metas {
		switch {
		case mt.Name == "calibre:series":
			m.Series = clean(mt.Content)
		case mt.Name == "calibre:series_index":
			m.SeriesIndex, _ = strconv.ParseFloat(clean(mt.Content), 64)
		case mt.Property == "belongs-to-collection":
			collections[""] = clean(mt.Value)
		case mt.Property == "group-position" && m.SeriesIndex == 0:
			m.SeriesIndex, _ = strconv.ParseFloat(clean(mt.Value), 64)
		}
	}
	if m.Series == "" {
		m.Series = collections[""]
	}
	return m, nil
}

func epubCover(p string) ([]byte, string, error) {
	z, o, root, err := openOPF(p)
	if err != nil {
		return nil, "", err
	}
	defer z.Close()
	href, mime := "", ""
	for _, it := range o.Manifest { // EPUB 3
		if strings.Contains(" "+it.Properties+" ", " cover-image ") {
			href, mime = it.Href, it.MediaType
		}
	}
	if href == "" { // EPUB 2: <meta name="cover" content="item-id"/>
		for _, mt := range o.Metadata.Metas {
			if mt.Name == "cover" {
				for _, it := range o.Manifest {
					if it.ID == mt.Content {
						href, mime = it.Href, it.MediaType
					}
				}
			}
		}
	}
	if href == "" { // last resort: an image called cover
		for _, it := range o.Manifest {
			if strings.HasPrefix(it.MediaType, "image/") && strings.Contains(strings.ToLower(it.ID+it.Href), "cover") {
				href, mime = it.Href, it.MediaType
				break
			}
		}
	}
	if href == "" {
		return nil, "", ErrNoCover
	}
	b, err := readFile(&z.Reader, path.Join(path.Dir(root), href))
	if err != nil {
		return nil, "", ErrNoCover
	}
	return b, mime, nil
}

func readFile(z *zip.Reader, name string) ([]byte, error) {
	name = strings.TrimPrefix(path.Clean(name), "/")
	for _, f := range z.File {
		if f.Name == name || strings.EqualFold(f.Name, name) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, 32<<20))
		}
	}
	return nil, os.ErrNotExist
}

func readXML(z *zip.Reader, name string, v any) error {
	b, err := readFile(z, name)
	if err != nil {
		return err
	}
	d := xml.NewDecoder(bytes.NewReader(b))
	d.Strict = false
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	return d.Decode(v)
}

// ---------- MOBI / AZW3 ----------

// mobiFile is a PalmDB file's record table plus its first record.
type mobiFile struct {
	data    []byte
	offsets []uint32
}

func (f mobiFile) record(i int) []byte {
	if i < 0 || i >= len(f.offsets) {
		return nil
	}
	start, end := int(f.offsets[i]), len(f.data)
	if i+1 < len(f.offsets) {
		end = int(f.offsets[i+1])
	}
	if start > end || end > len(f.data) {
		return nil
	}
	return f.data[start:end]
}

func openMOBI(p string) (mobiFile, []byte, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return mobiFile{}, nil, err
	}
	if len(data) < 78 || string(data[60:68]) != "BOOKMOBI" {
		return mobiFile{}, nil, errors.New("not a MOBI/AZW3 file")
	}
	n := int(binary.BigEndian.Uint16(data[76:78]))
	f := mobiFile{data: data}
	for i := 0; i < n && 78+i*8+4 <= len(data); i++ {
		f.offsets = append(f.offsets, binary.BigEndian.Uint32(data[78+i*8:]))
	}
	rec0 := f.record(0)
	if len(rec0) < 0x84 || string(rec0[16:20]) != "MOBI" {
		return f, nil, errors.New("MOBI header missing")
	}
	return f, rec0, nil
}

// exth returns the EXTH records of record 0, by type.
func exth(rec0 []byte) map[uint32][][]byte {
	out := map[uint32][][]byte{}
	hlen := int(binary.BigEndian.Uint32(rec0[20:24]))
	if binary.BigEndian.Uint32(rec0[0x80:0x84])&0x40 == 0 {
		return out
	}
	p := 16 + hlen
	if p+12 > len(rec0) || string(rec0[p:p+4]) != "EXTH" {
		return out
	}
	count := int(binary.BigEndian.Uint32(rec0[p+8:]))
	p += 12
	for i := 0; i < count && p+8 <= len(rec0); i++ {
		typ := binary.BigEndian.Uint32(rec0[p:])
		size := int(binary.BigEndian.Uint32(rec0[p+4:]))
		if size < 8 || p+size > len(rec0) {
			break
		}
		out[typ] = append(out[typ], rec0[p+8:p+size])
		p += size
	}
	return out
}

func readMOBI(p string, m Meta) (Meta, error) {
	_, rec0, err := openMOBI(p)
	if err != nil {
		return m, err
	}
	x := exth(rec0)
	str := func(t uint32) string {
		if v := x[t]; len(v) > 0 {
			return clean(string(v[0]))
		}
		return ""
	}
	m.Title = str(503) // updated title
	if m.Title == "" {
		off, n := int(binary.BigEndian.Uint32(rec0[0x54:])), int(binary.BigEndian.Uint32(rec0[0x58:]))
		if off+n <= len(rec0) {
			m.Title = clean(string(rec0[off : off+n]))
		}
	}
	for _, a := range x[100] {
		if s := clean(string(a)); s != "" {
			m.Authors = append(m.Authors, s)
		}
	}
	m.Description = stripTags(str(103))
	m.Date = str(106)
	m.Language = str(524)
	return m, nil
}

func mobiCover(p string) ([]byte, string, error) {
	f, rec0, err := openMOBI(p)
	if err != nil {
		return nil, "", err
	}
	x := exth(rec0)
	firstImage := int(binary.BigEndian.Uint32(rec0[0x6C:0x70]))
	for _, t := range []uint32{201, 202} { // cover, then thumbnail
		if v := x[t]; len(v) > 0 && len(v[0]) >= 4 {
			img := f.record(firstImage + int(binary.BigEndian.Uint32(v[0])))
			if mime := imageType(img); mime != "" {
				return img, mime, nil
			}
		}
	}
	return nil, "", ErrNoCover
}

func imageType(b []byte) string {
	switch {
	case len(b) > 3 && b[0] == 0xFF && b[1] == 0xD8:
		return "image/jpeg"
	case len(b) > 8 && string(b[1:4]) == "PNG":
		return "image/png"
	case len(b) > 6 && string(b[:3]) == "GIF":
		return "image/gif"
	}
	return ""
}

// ---------- helpers ----------

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

// stripTags turns an HTML description into plain text.
func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
			b.WriteRune(' ')
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	out := clean(b.String())
	for _, e := range [][2]string{{"&amp;", "&"}, {"&quot;", `"`}, {"&#39;", "'"}, {"&apos;", "'"}, {"&lt;", "<"}, {"&gt;", ">"}, {"&nbsp;", " "}} {
		out = strings.ReplaceAll(out, e[0], e[1])
	}
	return out
}
