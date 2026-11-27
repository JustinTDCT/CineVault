package ebook

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func writeEPUB(t *testing.T, opfXML string, files map[string][]byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "book.epub")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add := func(name string, b []byte) {
		w, _ := z.Create(name)
		w.Write(b)
	}
	add("mimetype", []byte("application/epub+zip"))
	add("META-INF/container.xml", []byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))
	add("OEBPS/content.opf", []byte(opfXML))
	for n, b := range files {
		add(n, b)
	}
	z.Close()
	f.Close()
	return p
}

var jpeg = []byte{0xFF, 0xD8, 0xFF, 0xE0, 'J', 'F', 'I', 'F'}

func TestEPUB2(t *testing.T) {
	p := writeEPUB(t, `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
 <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
  <dc:title>The Silmarillion</dc:title>
  <dc:creator opf:role="aut">J.R.R. Tolkien</dc:creator>
  <dc:creator opf:role="edt">Christopher Tolkien</dc:creator>
  <dc:description>&lt;p&gt;The &lt;b&gt;first&lt;/b&gt; age.&lt;/p&gt;</dc:description>
  <dc:date>1977-09-15</dc:date>
  <dc:language>en</dc:language>
  <meta name="calibre:series" content="Middle-earth"/>
  <meta name="calibre:series_index" content="5.0"/>
  <meta name="cover" content="cover-img"/>
 </metadata>
 <manifest><item id="cover-img" href="images/cover.jpg" media-type="image/jpeg"/></manifest>
</package>`, map[string][]byte{"OEBPS/images/cover.jpg": jpeg})
	m, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Format != "epub" || m.Title != "The Silmarillion" || len(m.Authors) != 1 || m.Authors[0] != "J.R.R. Tolkien" ||
		m.Description != "The first age." || m.Date != "1977-09-15" || m.Series != "Middle-earth" || m.SeriesIndex != 5 {
		t.Errorf("meta = %+v", m)
	}
	b, mime, err := Cover(p)
	if err != nil || mime != "image/jpeg" || !bytes.Equal(b, jpeg) {
		t.Errorf("cover: %v %s %d bytes", err, mime, len(b))
	}
}

func TestEPUB3(t *testing.T) {
	p := writeEPUB(t, `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
 <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
  <dc:title>The Fall of Gondolin</dc:title>
  <dc:creator id="c1">J.R.R. Tolkien</dc:creator>
  <meta refines="#c1" property="role">aut</meta>
  <dc:creator id="c2">Alan Lee</dc:creator>
  <meta refines="#c2" property="role">ill</meta>
  <meta property="belongs-to-collection" id="s">Great Tales</meta>
  <meta refines="#s" property="group-position">3</meta>
 </metadata>
 <manifest><item id="x" href="cover.png" media-type="image/png" properties="cover-image"/></manifest>
</package>`, map[string][]byte{"OEBPS/cover.png": []byte("\x89PNG....")})
	m, _ := Read(p)
	if m.Title != "The Fall of Gondolin" || len(m.Authors) != 1 || m.Series != "Great Tales" || m.SeriesIndex != 3 {
		t.Errorf("meta = %+v", m)
	}
	if _, mime, err := Cover(p); err != nil || mime != "image/png" {
		t.Errorf("cover: %v %s", err, mime)
	}
}

// writeMOBI builds a minimal AZW3: PalmDB header, record 0 with a MOBI
// header and EXTH (author, title, description, cover), and one image record.
func writeMOBI(t *testing.T) string {
	t.Helper()
	be := binary.BigEndian
	exthRec := func(typ uint32, v []byte) []byte {
		b := make([]byte, 8+len(v))
		be.PutUint32(b, typ)
		be.PutUint32(b[4:], uint32(8+len(v)))
		copy(b[8:], v)
		return b
	}
	cover := make([]byte, 4) // cover offset 0 from the first image
	var ex bytes.Buffer
	recs := [][]byte{exthRec(100, []byte("J.R.R. Tolkien")), exthRec(503, []byte("Beowulf: A Translation and Commentary")),
		exthRec(103, []byte("<p>A translation.</p>")), exthRec(201, cover)}
	ex.WriteString("EXTH")
	size := 12
	for _, r := range recs {
		size += len(r)
	}
	h := make([]byte, 8)
	be.PutUint32(h, uint32(size))
	be.PutUint32(h[4:], uint32(len(recs)))
	ex.Write(h)
	for _, r := range recs {
		ex.Write(r)
	}
	const mobiLen = 0xE8
	rec0 := make([]byte, 16+mobiLen)
	copy(rec0[16:], "MOBI")
	be.PutUint32(rec0[20:], mobiLen)
	be.PutUint32(rec0[0x6C:], 1) // first image record
	be.PutUint32(rec0[0x80:], 0x40)
	rec0 = append(rec0, ex.Bytes()...)
	name := []byte("Beowulf")
	be.PutUint32(rec0[0x54:], uint32(len(rec0)))
	be.PutUint32(rec0[0x58:], uint32(len(name)))
	rec0 = append(rec0, name...)

	hdr := make([]byte, 78+2*8+2)
	copy(hdr[60:], "BOOKMOBI")
	be.PutUint16(hdr[76:], 2)
	be.PutUint32(hdr[78:], uint32(len(hdr)))
	be.PutUint32(hdr[86:], uint32(len(hdr)+len(rec0)))
	data := append(append(hdr, rec0...), jpeg...)
	p := filepath.Join(t.TempDir(), "Beowulf (azw3).azw3")
	os.WriteFile(p, data, 0o644)
	return p
}

func TestAZW3(t *testing.T) {
	p := writeMOBI(t)
	m, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Format != "azw3" || m.Title != "Beowulf: A Translation and Commentary" || len(m.Authors) != 1 || m.Authors[0] != "J.R.R. Tolkien" || m.Description != "A translation." {
		t.Errorf("meta = %+v", m)
	}
	b, mime, err := Cover(p)
	if err != nil || mime != "image/jpeg" || !bytes.Equal(b, jpeg) {
		t.Errorf("cover: %v %s %v", err, mime, b)
	}
}

func TestNotABook(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.azw3")
	os.WriteFile(p, []byte("hello"), 0o644)
	if _, err := Read(p); err == nil {
		t.Error("expected an error for a broken AZW3")
	}
	if m, err := Read("/x/book.pdf"); err != nil || m.Format != "pdf" {
		t.Errorf("pdf: %+v %v", m, err)
	}
}
