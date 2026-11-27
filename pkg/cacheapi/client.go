package cacheapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// maxResponseBytes caps how much of a reply the client reads.
const maxResponseBytes = 32 << 20

// Client calls the CineVault Cache API. The zero value isn't usable; set
// BaseURL and Token.
type Client struct {
	// BaseURL is the service root, without /v1: "https://cache.cinevault.net"
	// or "http://10.150.125.59:8457".
	BaseURL string
	// Token is this server's subscription token. It is sent as a bearer token
	// and never logged.
	Token string
	// UserAgent should be "CineVault/<version>".
	UserAgent string
	// HTTP is the client used for requests; nil means a client with a 30 s
	// timeout.
	HTTP *http.Client
}

var defaultHTTP = &http.Client{Timeout: 30 * time.Second}

// Error is an error reply from the service.
type Error struct {
	StatusCode int
	Code       string // one of the Code constants, or "" if the body wasn't an ErrorBody
	Message    string
	// RetryAfter is the Retry-After delay the service asked for, if any.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("cinevault cache: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("cinevault cache: %s (HTTP %d): %s", e.Code, e.StatusCode, e.Message)
}

// IsAuth reports whether err means the token is missing, wrong or revoked:
// stop calling until the token changes.
func IsAuth(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.StatusCode == http.StatusUnauthorized
}

// IsInactive reports whether err means the subscription isn't active: treat
// the server as unsubscribed.
func IsInactive(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == CodeSubscriptionInactive
}

// IsRetryable reports whether a later retry of the same request may succeed:
// rate limits, server errors and network failures.
func IsRetryable(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
	}
	return err != nil && !errors.Is(err, context.Canceled)
}

// LookupState is the outcome of a lookup.
type LookupState int

const (
	// Ready: Record holds the title's record.
	Ready LookupState = iota + 1
	// Researching: the cache queued the title for research. Fill it from
	// direct sources and mark it "Cache pending".
	Researching
	// NotFound: TMDb has no title with this key.
	NotFound
)

// LookupResult is what Lookup returns.
type LookupResult struct {
	State      LookupState
	Record     *Record       // set when State is Ready
	RetryAfter time.Duration // a hint when State is Researching
}

// Lookup fetches one title's record. editions are Edition List IDs found in
// the user's file names (optional); they only raise the research priority.
func (c *Client) Lookup(ctx context.Context, key Key, editions ...string) (LookupResult, error) {
	if err := key.Validate(); err != nil {
		return LookupResult{}, err
	}
	if len(editions) > MaxLookupEditions {
		editions = editions[:MaxLookupEditions]
	}
	path := "/titles/" + key.Path()
	if len(editions) > 0 {
		path += "?editions=" + url.QueryEscape(strings.Join(editions, ","))
	}
	status, body, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		var e *Error
		if errors.As(err, &e) && e.StatusCode == http.StatusNotFound && e.Code == CodeNotFound {
			return LookupResult{State: NotFound}, nil
		}
		return LookupResult{}, err
	}
	switch status {
	case http.StatusOK:
		var r Record
		if err := json.Unmarshal(body, &r); err != nil {
			return LookupResult{}, fmt.Errorf("cinevault cache: bad record for %s: %w", key, err)
		}
		return LookupResult{State: Ready, Record: &r}, nil
	case http.StatusAccepted:
		var p Pending
		if err := json.Unmarshal(body, &p); err != nil {
			return LookupResult{}, fmt.Errorf("cinevault cache: bad pending reply for %s: %w", key, err)
		}
		return LookupResult{State: Researching, RetryAfter: time.Duration(p.RetryAfterS) * time.Second}, nil
	}
	return LookupResult{}, &Error{StatusCode: status}
}

// Sync sends one batch of at most MaxSyncItems items. SyncAll splits a
// larger list.
func (c *Client) Sync(ctx context.Context, items []SyncItem) (SyncResponse, error) {
	req := SyncRequest{Items: items}
	if err := req.Validate(); err != nil {
		return SyncResponse{}, err
	}
	var out SyncResponse
	return out, c.call(ctx, http.MethodPost, "/sync", req, &out)
}

// SyncAll syncs any number of items in batches of MaxSyncItems and merges
// the replies. It stops at the first error.
func (c *Client) SyncAll(ctx context.Context, items []SyncItem) (SyncResponse, error) {
	all := SyncResponse{Changed: []Changed{}, Pending: []Key{}, NotFound: []Key{}}
	for start := 0; start < len(items); start += MaxSyncItems {
		end := min(start+MaxSyncItems, len(items))
		r, err := c.Sync(ctx, items[start:end])
		if err != nil {
			return all, err
		}
		all.Changed = append(all.Changed, r.Changed...)
		all.Pending = append(all.Pending, r.Pending...)
		all.NotFound = append(all.NotFound, r.NotFound...)
		all.Unchanged += r.Unchanged
	}
	return all, nil
}

// Records fetches records for any number of keys, in batches of
// MaxRecordKeys, in key order. It stops at the first error.
func (c *Client) Records(ctx context.Context, keys []Key) ([]Record, error) {
	out := make([]Record, 0, len(keys))
	for start := 0; start < len(keys); start += MaxRecordKeys {
		end := min(start+MaxRecordKeys, len(keys))
		req := RecordsRequest{Keys: keys[start:end]}
		if err := req.Validate(); err != nil {
			return out, err
		}
		var r RecordsResponse
		if err := c.call(ctx, http.MethodPost, "/records", req, &r); err != nil {
			return out, err
		}
		if len(r.Records) != len(req.Keys) {
			return out, fmt.Errorf("cinevault cache: asked for %d records, got %d", len(req.Keys), len(r.Records))
		}
		out = append(out, r.Records...)
	}
	return out, nil
}

// Report sends a "Report a mistake".
func (c *Client) Report(ctx context.Context, r Report) (ReportResponse, error) {
	if err := r.Validate(); err != nil {
		return ReportResponse{}, err
	}
	var out ReportResponse
	return out, c.call(ctx, http.MethodPost, "/reports", r, &out)
}

// Contribute shares which titles this server has (spec §5a). It needs no
// token, so free servers can call it; a client with a token doesn't send it,
// so a contribution can't be tied to a subscriber. Larger lists are sent in
// batches of MaxContributeItems; the result is the total accepted.
func (c *Client) Contribute(ctx context.Context, install string, items []ContributeItem) (int, error) {
	anon := *c
	anon.Token = ""
	total := 0
	for start := 0; start < len(items); start += MaxContributeItems {
		end := min(start+MaxContributeItems, len(items))
		req := ContributeRequest{Install: install, Items: items[start:end]}
		if err := req.Validate(); err != nil {
			return total, err
		}
		var out ContributeResponse
		if err := anon.call(ctx, http.MethodPost, "/contribute", req, &out); err != nil {
			return total, err
		}
		total += out.Accepted
	}
	return total, nil
}

// Subscription returns the token's subscription status. It is also the
// "Test token" check in Settings.
func (c *Client) Subscription(ctx context.Context) (Subscription, error) {
	var out Subscription
	return out, c.call(ctx, http.MethodGet, "/subscription", nil, &out)
}

// Status checks that the service is up. It needs no token.
func (c *Client) Status(ctx context.Context) (ServiceStatus, error) {
	var out ServiceStatus
	return out, c.call(ctx, http.MethodGet, "/status", nil, &out)
}

// call sends a request and decodes a 2xx JSON reply into out.
func (c *Client) call(ctx context.Context, method, path string, in, out any) error {
	_, body, err := c.do(ctx, method, path, in)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("cinevault cache: bad reply to %s %s: %w", method, path, err)
	}
	return nil
}

// do sends a request under /v1 and returns the status and body of a 2xx
// reply, or an *Error.
func (c *Client) do(ctx context.Context, method, path string, in any) (int, []byte, error) {
	if c.BaseURL == "" {
		return 0, nil, errors.New("cinevault cache: no base URL set")
	}
	var reqBody io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, nil, err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method,
		strings.TrimRight(c.BaseURL, "/")+"/"+APIVersion+path, reqBody)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	hc := c.HTTP
	if hc == nil {
		hc = defaultHTTP
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("cinevault cache: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("cinevault cache: reading reply to %s %s: %w", method, path, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, body, nil
	}

	e := &Error{StatusCode: resp.StatusCode, RetryAfter: retryAfter(resp.Header.Get("Retry-After"))}
	var eb ErrorBody
	if json.Unmarshal(body, &eb) == nil {
		e.Code, e.Message = eb.Error.Code, eb.Error.Message
	}
	return resp.StatusCode, nil, e
}

// retryAfter parses a Retry-After header given in seconds.
func retryAfter(v string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}
