package cacheapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const testToken = "cvc_TESTTOKEN"

// fakeCache serves the golden files the way the real service would.
func fakeCache(t *testing.T) (*Client, *[]*http.Request) {
	t.Helper()
	var mu sync.Mutex
	var seen []*http.Request
	golden := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write(golden("status.json"))
	})
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			seen = append(seen, r)
			mu.Unlock()
			if r.Header.Get("Authorization") != "Bearer "+testToken {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write(golden("error_token_revoked.json"))
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /v1/titles/movie/tmdb/{id}", authed(func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("id") {
		case "679":
			w.Write(golden("record_aliens.json"))
		case "348":
			w.WriteHeader(http.StatusAccepted)
			w.Write(golden("lookup_pending.json"))
		case "429":
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"code":"rate_limited","message":"slow down"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"code":"not_found","message":"no such title"}}`))
		}
	}))
	mux.HandleFunc("POST /v1/sync", authed(func(w http.ResponseWriter, r *http.Request) {
		var req SyncRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Validate() != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		json.NewEncoder(w).Encode(SyncResponse{Changed: []Changed{}, Pending: []Key{}, NotFound: []Key{}, Unchanged: len(req.Items)})
	}))
	mux.HandleFunc("POST /v1/records", authed(func(w http.ResponseWriter, r *http.Request) {
		var req RecordsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Validate() != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		var aliens Record
		json.Unmarshal(golden("record_aliens.json"), &aliens)
		out := RecordsResponse{}
		for _, k := range req.Keys {
			rec := aliens
			rec.Key = k
			out.Records = append(out.Records, rec)
		}
		json.NewEncoder(w).Encode(out)
	}))
	mux.HandleFunc("POST /v1/reports", authed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write(golden("report_response.json"))
	}))
	mux.HandleFunc("POST /v1/contribute", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r)
		mu.Unlock()
		var req ContributeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Validate() != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(ContributeResponse{Accepted: len(req.Items)})
	})
	mux.HandleFunc("GET /v1/subscription", authed(func(w http.ResponseWriter, r *http.Request) {
		w.Write(golden("subscription.json"))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL + "/", Token: testToken, UserAgent: "CineVault/test"}, &seen
}

func TestLookupStates(t *testing.T) {
	c, seen := fakeCache(t)
	ctx := context.Background()

	res, err := c.Lookup(ctx, Key{Kind: KindMovie, TMDbID: 679}, "directors_cut", "extended")
	if err != nil || res.State != Ready || res.Record == nil || res.Record.RecordVersion != 4 {
		t.Fatalf("ready lookup = %+v, %v", res, err)
	}
	r := (*seen)[0]
	if r.URL.Query().Get("editions") != "directors_cut,extended" || r.Header.Get("User-Agent") != "CineVault/test" {
		t.Errorf("request: query %q, user agent %q", r.URL.RawQuery, r.Header.Get("User-Agent"))
	}

	res, err = c.Lookup(ctx, Key{Kind: KindMovie, TMDbID: 348})
	if err != nil || res.State != Researching || res.RetryAfter != time.Hour {
		t.Errorf("pending lookup = %+v, %v", res, err)
	}

	res, err = c.Lookup(ctx, Key{Kind: KindMovie, TMDbID: 999999})
	if err != nil || res.State != NotFound {
		t.Errorf("missing lookup = %+v, %v", res, err)
	}

	_, err = c.Lookup(ctx, Key{Kind: KindMovie, TMDbID: 429})
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeRateLimited || apiErr.RetryAfter != 30*time.Second || !IsRetryable(err) {
		t.Errorf("rate-limited lookup error = %#v", err)
	}

	if _, err := c.Lookup(ctx, Key{Kind: "film", TMDbID: 1}); err == nil {
		t.Error("bad key accepted")
	}
}

func TestAuthError(t *testing.T) {
	c, _ := fakeCache(t)
	c.Token = "cvc_WRONG"
	_, err := c.Lookup(context.Background(), Key{Kind: KindMovie, TMDbID: 679})
	if !IsAuth(err) || IsRetryable(err) {
		t.Fatalf("err = %v, want an auth error that isn't retryable", err)
	}
	var apiErr *Error
	if errors.As(err, &apiErr); apiErr.Code != CodeTokenRevoked {
		t.Errorf("code %q", apiErr.Code)
	}
}

func TestSyncAllBatches(t *testing.T) {
	c, seen := fakeCache(t)
	items := make([]SyncItem, 2*MaxSyncItems+5)
	for i := range items {
		items[i] = SyncItem{Kind: KindMovie, TMDbID: i + 1}
	}
	res, err := c.SyncAll(context.Background(), items)
	if err != nil {
		t.Fatal(err)
	}
	if res.Unchanged != len(items) || len(*seen) != 3 {
		t.Errorf("unchanged %d over %d requests, want %d over 3", res.Unchanged, len(*seen), len(items))
	}
}

func TestRecordsBatches(t *testing.T) {
	c, seen := fakeCache(t)
	keys := make([]Key, MaxRecordKeys+1)
	for i := range keys {
		keys[i] = Key{Kind: KindMovie, TMDbID: i + 1}
	}
	recs, err := c.Records(context.Background(), keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != len(keys) || recs[MaxRecordKeys].Key != keys[MaxRecordKeys] || len(*seen) != 2 {
		t.Errorf("got %d records over %d requests", len(recs), len(*seen))
	}
}

func TestReportSubscriptionStatus(t *testing.T) {
	c, _ := fakeCache(t)
	ctx := context.Background()
	rep, err := c.Report(ctx, Report{Key: Key{Kind: KindMovie, TMDbID: 679}, Field: FieldRuntime, Message: "148 min"})
	if err != nil || rep.ReportID == "" {
		t.Errorf("report = %+v, %v", rep, err)
	}
	if _, err := c.Report(ctx, Report{Key: Key{Kind: KindMovie, TMDbID: 679}, Field: "plot", Message: "x"}); err == nil {
		t.Error("invalid report sent")
	}
	sub, err := c.Subscription(ctx)
	if err != nil || sub.Status != "active" || sub.ExpiresAt.Year() != 2027 {
		t.Errorf("subscription = %+v, %v", sub, err)
	}
	c.Token = "" // status needs no token
	st, err := c.Status(ctx)
	if err != nil || st.Status != "ok" {
		t.Errorf("status = %+v, %v", st, err)
	}
}

func TestNetworkErrorIsRetryable(t *testing.T) {
	c := &Client{BaseURL: "http://127.0.0.1:1", Token: testToken}
	_, err := c.Status(context.Background())
	if err == nil || !IsRetryable(err) || IsAuth(err) {
		t.Errorf("err = %v", err)
	}
}

func TestContributeIsAnonymousAndBatched(t *testing.T) {
	c, seen := fakeCache(t)
	items := make([]ContributeItem, MaxContributeItems+5)
	for i := range items {
		items[i] = ContributeItem{Kind: "movie", TMDbID: i + 1, Editions: []ContributeEdition{{Edition: "theatrical", RuntimeMin: 120}}}
	}
	n, err := c.Contribute(context.Background(), "K3VQ7TZ2MXN4P6RBYC5HWJ8DLE9FGA2S", items)
	if err != nil || n != len(items) {
		t.Fatalf("accepted %d, err %v", n, err)
	}
	if len(*seen) != 2 {
		t.Fatalf("requests = %d, want 2 batches", len(*seen))
	}
	for _, r := range *seen {
		if r.Header.Get("Authorization") != "" {
			t.Error("a contribution must not carry the subscriber's token")
		}
	}
	if _, err := c.Contribute(context.Background(), "short", items[:1]); err == nil {
		t.Error("a short install ID should be refused before sending")
	}
}
