package theme

import (
	"bytes"
	"flag"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite tokens.css from tokens.json")

func TestTokensCSSUpToDate(t *testing.T) {
	src, err := os.ReadFile("tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	want, err := TokensCSS(src)
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile("tokens.css", want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile("tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("tokens.css is out of date: run go test ./pkg/theme -update")
	}
}

var (
	varUse  = regexp.MustCompile(`var\(--([a-z0-9-]+)`)
	varDecl = regexp.MustCompile(`(?:^|[{;\s])--([a-z0-9-]+)\s*:`)
	urlRef  = regexp.MustCompile(`url\("\./([^"]+)"\)`)
)

// Every custom property a stylesheet uses is declared (a typo would
// silently fall back), and every font file a stylesheet names exists.
func TestStylesheetsResolve(t *testing.T) {
	declared := map[string]bool{} // by tokens.css, or locally in a stylesheet (such as --cols)
	for _, name := range Stylesheets {
		css, _ := fs.ReadFile(FS(), name)
		for _, m := range varDecl.FindAllSubmatch(css, -1) {
			declared[string(m[1])] = true
		}
	}
	for _, name := range Stylesheets {
		css, err := fs.ReadFile(FS(), name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, m := range varUse.FindAllSubmatch(css, -1) {
			if !declared[string(m[1])] {
				t.Errorf("%s uses --%s, which tokens.json doesn't define", name, m[1])
			}
		}
		for _, m := range urlRef.FindAllSubmatch(css, -1) {
			if _, err := fs.Stat(FS(), string(m[1])); err != nil {
				t.Errorf("%s links %s: %v", name, m[1], err)
			}
		}
	}
}

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(http.StripPrefix("/theme/", Handler()))
	defer srv.Close()
	for path, want := range map[string]int{
		"/theme/cinevault.css": 200, "/theme/fonts/manrope-latin-wght-normal.woff2": 200,
		"/theme/tokens.json": 404, "/theme/fonts/": 404, "/theme/nope.css": 404,
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("%s: %d, want %d", path, resp.StatusCode, want)
		}
		if want == 200 && !strings.Contains(resp.Header.Get("Cache-Control"), "max-age") {
			t.Errorf("%s: no Cache-Control", path)
		}
	}
}
