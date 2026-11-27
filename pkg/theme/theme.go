// Package theme is CineVault's look: the design system "CineVault"
// (direction D · Widescreen) as files both apps load. The media server's web
// app imports them at build time; the CineVault Cache serves them to its
// review UI from this package, so a change here reaches both.
//
//	tokens.json    the design tokens: colours, type, spacing, radii (the source)
//	tokens.css     the tokens as CSS custom properties, generated from tokens.json
//	fonts.css      @font-face rules for the two bundled fonts
//	cinevault.css  every cv-… class pages use
//	fonts/         Bebas Neue and Manrope (SIL Open Font License 1.1)
//
// After editing tokens.json, regenerate tokens.css with
// `go test ./pkg/theme -update`; the test fails while the two disagree.
package theme

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
)

//go:embed tokens.json tokens.css fonts.css cinevault.css fonts
var files embed.FS

// Stylesheets lists the stylesheets in the order a page links them.
var Stylesheets = []string{"fonts.css", "tokens.css", "cinevault.css"}

// FS returns the theme's files.
func FS() fs.FS { return files }

// Handler serves the theme's files, for example under /theme/. Fonts and
// stylesheets change only with a new build, so they may be cached for a day.
func Handler() http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".json") || strings.HasSuffix(r.URL.Path, "/") || r.URL.Path == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fileServer.ServeHTTP(w, r)
	})
}

// tokenFile is the part of tokens.json that becomes CSS.
type tokenFile struct {
	Name  string `json:"name"`
	Color struct {
		Tokens []token `json:"tokens"`
	} `json:"color"`
	Type struct {
		Families map[string]string `json:"families"`
	} `json:"type"`
	Spacing tokenList `json:"spacing"`
	Radius  tokenList `json:"radius"`
	Shadow  tokenList `json:"shadow"`
	Layout  tokenList `json:"layout"`
}

type tokenList struct {
	Tokens []token `json:"tokens"`
}

type token struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Usage string `json:"usage"`
}

var tokenName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// TokensCSS turns tokens.json into tokens.css: one custom property per token,
// --font-<key> per type family, in the file's order.
func TokensCSS(tokensJSON []byte) ([]byte, error) {
	var t tokenFile
	if err := json.Unmarshal(tokensJSON, &t); err != nil {
		return nil, fmt.Errorf("tokens.json: %w", err)
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "/* %s — generated from tokens.json by `go test ./pkg/theme -update`. Do not edit. */\n:root {\n", t.Name)
	seen := map[string]bool{}
	write := func(group string, ts []token) error {
		if len(ts) == 0 {
			return nil
		}
		fmt.Fprintf(&b, "  /* %s */\n", group)
		for _, tk := range ts {
			if !tokenName.MatchString(tk.Name) || seen[tk.Name] {
				return fmt.Errorf("token %q: names are lower-case words joined by hyphens, each used once", tk.Name)
			}
			if strings.ContainsAny(tk.Value, ";{}<>") || tk.Value == "" {
				return fmt.Errorf("token %s: bad value %q", tk.Name, tk.Value)
			}
			seen[tk.Name] = true
			fmt.Fprintf(&b, "  --%s: %s;\n", tk.Name, tk.Value)
		}
		return nil
	}
	for _, g := range []struct {
		name string
		ts   []token
	}{
		{"colour", t.Color.Tokens}, {"spacing", t.Spacing.Tokens}, {"radius", t.Radius.Tokens},
		{"shadow", t.Shadow.Tokens}, {"layout", t.Layout.Tokens},
	} {
		if err := write(g.name, g.ts); err != nil {
			return nil, err
		}
	}
	b.WriteString("  /* type */\n")
	for _, key := range []string{"display", "text"} {
		stack, ok := t.Type.Families[key]
		if !ok {
			return nil, fmt.Errorf("tokens.json: type.families.%s is missing", key)
		}
		fmt.Fprintf(&b, "  --font-%s: %s;\n", key, stack)
	}
	b.WriteString("  color-scheme: dark;\n}\n")
	return b.Bytes(), nil
}
