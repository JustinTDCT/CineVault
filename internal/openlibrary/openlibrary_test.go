package openlibrary

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSearchAndDetails(t *testing.T) {
	var agent, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.Header.Get("User-Agent")
		switch r.URL.Path {
		case "/search.json":
			query = r.URL.RawQuery
			w.Write([]byte(`{"docs":[{"key":"/works/OL893415W","title":"Dune","author_name":["Frank Herbert"],"first_publish_year":1965,"cover_i":11481354,"edition_count":185},{"key":"bad"}]}`))
		case "/works/OL893415W.json":
			w.Write([]byte(`{"description":{"type":"/type/text","value":"Desert planet.\r\n----------\r\nAlso contained in: …"},"subjects":["Science fiction","nyt:mass-market-paperback=2021","Accessible book","Arrakis (Imaginary place) -- Fiction","Fiction","Science fiction"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, UserAgent: "CineVault/test", Every: time.Millisecond}
	works, err := c.Search(context.Background(), "Dune", "Frank Herbert")
	if err != nil || len(works) != 1 || works[0].ID != "OL893415W" || works[0].CoverURL() != "https://covers.openlibrary.org/b/id/11481354-L.jpg" {
		t.Fatalf("%+v %v", works, err)
	}
	if agent != "CineVault/test" || !strings.Contains(query, "author=Frank+Herbert") || !strings.Contains(query, "title=Dune") {
		t.Errorf("agent %q query %q", agent, query)
	}
	d, err := c.WorkDetails(context.Background(), "OL893415W")
	if err != nil || d.Description != "Desert planet." {
		t.Fatalf("%+v %v", d, err)
	}
	if g := Genres(d.Subjects, 4); strings.Join(g, "|") != "Science fiction|Fiction" {
		t.Errorf("genres %q", g)
	}
	if _, err := c.WorkDetails(context.Background(), "OL1W"); err == nil {
		t.Error("404 passed")
	}
}

func TestGenresSkipCatalogueTerms(t *testing.T) {
	got := Genres([]string{"Literary collections", "Middle Earth (Imaginary place)", "English Fantasy literature", "Fantasy fiction", "Fiction"}, 4)
	if strings.Join(got, "|") != "English Fantasy literature|Fantasy fiction|Fiction" {
		t.Errorf("genres %q", got)
	}
}

func TestPlainDescription(t *testing.T) {
	var d struct{}
	_ = d
	if got := CleanDescription("A book. ([source][1])\n\n[1]: http://x"); got != "A book." {
		t.Errorf("%q", got)
	}
}
