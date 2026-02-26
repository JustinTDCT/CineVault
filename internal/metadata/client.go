package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/JustinTDCT/CineVault/internal/config"
)

type CacheClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewCacheClient(cfg *config.Config) *CacheClient {
	return &CacheClient{
		baseURL: config.CacheServerURL,
		apiKey:  cfg.CacheServerKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type LookupResult struct {
	Status     string                 `json:"status"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Error      *LookupError           `json:"error,omitempty"`
	CacheHit   bool
	Confidence float64
	CacheID    string
	RecordAge  int
}

type LookupError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *CacheClient) Lookup(recordType string, params map[string]string) (*LookupResult, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/v1/lookup/%s", c.baseURL, recordType))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	var resp *http.Response
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("cache server request: %w", err)
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			break
		}
		resp.Body.Close()
		time.Sleep(time.Duration(2<<uint(attempt)) * time.Second)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return &LookupResult{Status: "error"}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result := &LookupResult{}
	if err := json.Unmarshal(body, result); err != nil {
		return nil, fmt.Errorf("parse cache response: %w", err)
	}

	result.CacheHit = resp.Header.Get("X-Cache-Status") == "hit"

	if conf := resp.Header.Get("X-Match-Confidence"); conf != "" {
		fmt.Sscanf(conf, "%f", &result.Confidence)
	}
	if age := resp.Header.Get("X-Record-Age"); age != "" {
		fmt.Sscanf(age, "%d", &result.RecordAge)
	}
	if result.Data != nil {
		if cid, ok := result.Data["cache_id"].(string); ok {
			result.CacheID = cid
		}
	}

	return result, nil
}

func (c *CacheClient) GetRecord(cacheID string) (*LookupResult, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/record/%s", c.baseURL, cacheID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	result := &LookupResult{}
	json.Unmarshal(body, result)
	return result, nil
}

func (c *CacheClient) BatchGet(cacheIDs []string) (map[string]*LookupResult, error) {
	if len(cacheIDs) == 0 {
		return map[string]*LookupResult{}, nil
	}

	q := url.Values{}
	ids := ""
	for i, id := range cacheIDs {
		if i > 0 {
			ids += ","
		}
		ids += id
	}
	q.Set("ids", ids)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/records/batch?%s", c.baseURL, q.Encode()), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Status string `json:"status"`
		Data   struct {
			Records   []map[string]interface{} `json:"records"`
			Redirects []struct {
				CacheID    string `json:"cache_id"`
				MergedInto string `json:"merged_into"`
			} `json:"redirects"`
			Gone []struct {
				CacheID string `json:"cache_id"`
			} `json:"gone"`
		} `json:"data"`
	}
	json.Unmarshal(body, &envelope)

	results := make(map[string]*LookupResult)
	for _, rec := range envelope.Data.Records {
		cid, _ := rec["cache_id"].(string)
		results[cid] = &LookupResult{Status: "ok", Data: rec}
	}
	for _, rd := range envelope.Data.Redirects {
		results[rd.CacheID] = &LookupResult{Status: "redirect", CacheID: rd.MergedInto}
	}
	for _, g := range envelope.Data.Gone {
		results[g.CacheID] = &LookupResult{Status: "gone"}
	}
	return results, nil
}

func (c *CacheClient) OverrideField(cacheID string, fields map[string]interface{}) error {
	body, _ := json.Marshal(fields)
	req, err := http.NewRequest("PATCH",
		fmt.Sprintf("%s/api/v1/record/%s", c.baseURL, cacheID),
		jsonReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *CacheClient) RefreshRecord(cacheID string) error {
	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/api/v1/record/%s/refresh", c.baseURL, cacheID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

type jsonReaderType []byte

func jsonReader(b []byte) io.Reader {
	return (*jsonReaderWrapper)(&b)
}

type jsonReaderWrapper []byte

func (j *jsonReaderWrapper) Read(p []byte) (int, error) {
	n := copy(p, *j)
	*j = (*j)[n:]
	if len(*j) == 0 {
		return n, io.EOF
	}
	return n, nil
}
