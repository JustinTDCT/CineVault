package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
)

func TestEditTitle(t *testing.T) {
	s, store := libServer(nil)
	code, body := send(t, s, "PATCH", "/api/titles/1", `{"name":"Alien (Director's Cut)","year":1979,"genres":["Horror"]}`)
	if code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", code, body)
	}
	if e := store.lastEdit; e.Name == nil || *e.Name != "Alien (Director's Cut)" || e.Year == nil || *e.Year != 1979 || e.Genres == nil || e.Overview != nil {
		t.Errorf("edit %+v", e)
	}
	for req, want := range map[string]string{
		`{"name":"  "}`:       "needs a name",
		`{"year":12}`:         "between 1800",
		`{"series_index":-1}`: "negative",
		`{"title":"x"}`:       "as JSON",
		`not json`:            "as JSON",
	} {
		if code, body := send(t, s, "PATCH", "/api/titles/1", req); code != http.StatusBadRequest || !strings.Contains(body, want) {
			t.Errorf("%s: code=%d body=%s", req, code, body)
		}
	}
	if code, _ := send(t, s, "PATCH", "/api/titles/9", `{"name":"x"}`); code != http.StatusNotFound {
		t.Errorf("unknown title: %d", code)
	}
}

func TestTitleLocksAndLibrarySetting(t *testing.T) {
	s, store := libServer(nil)
	code, body := send(t, s, "PUT", "/api/titles/1/locks", `{"locked":true,"fields":["name"]}`)
	if code != http.StatusOK || !strings.Contains(body, `"locked": true`) || !strings.Contains(body, `"name"`) {
		t.Errorf("locks: code=%d body=%s", code, body)
	}
	store.libs = []catalog.Library{{ID: 1, Name: "Music", MediaType: "music"}}
	code, body = send(t, s, "PATCH", "/api/libraries/1", `{"prefer_local_metadata":true}`)
	if code != http.StatusOK || !store.libs[0].PreferLocalMetadata || !strings.Contains(body, `"prefer_local_metadata": true`) {
		t.Errorf("library setting: code=%d body=%s", code, body)
	}
	if code, _ := send(t, s, "PATCH", "/api/libraries/1", `{}`); code != http.StatusBadRequest {
		t.Errorf("empty body: %d", code)
	}
}
