package cacheapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The golden files pin the wire format. Each one must decode into its type
// and encode back to the same JSON: a renamed, dropped or added field fails
// here, in both the media server and the cache service.
func TestGoldenFilesRoundTrip(t *testing.T) {
	cases := map[string]func() any{
		"record_aliens.json":       func() any { return new(Record) },
		"record_withdrawn.json":    func() any { return new(Record) },
		"lookup_pending.json":      func() any { return new(Pending) },
		"sync_request.json":        func() any { return new(SyncRequest) },
		"sync_response.json":       func() any { return new(SyncResponse) },
		"records_request.json":     func() any { return new(RecordsRequest) },
		"report.json":              func() any { return new(Report) },
		"report_response.json":     func() any { return new(ReportResponse) },
		"error_token_revoked.json": func() any { return new(ErrorBody) },
		"subscription.json":        func() any { return new(Subscription) },
		"status.json":              func() any { return new(ServiceStatus) },
		"contribute_request.json":  func() any { return new(ContributeRequest) },
		"contribute_response.json": func() any { return new(ContributeResponse) },
	}
	files, err := filepath.Glob("testdata/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(cases) {
		t.Errorf("%d golden files but %d cases: add the new file to this test", len(files), len(cases))
	}
	for name, newValue := range cases {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			v := newValue()
			dec := json.NewDecoder(bytesReader(raw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(v); err != nil {
				t.Fatalf("decode: %v", err)
			}
			again, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(again, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Errorf("round trip changed the JSON\nfile: %s\ngot:  %s", compact(raw), again)
			}
		})
	}
}

func TestGoldenValuesAreValid(t *testing.T) {
	for _, name := range []string{"record_aliens.json", "record_withdrawn.json"} {
		var r Record
		readGolden(t, name, &r)
		if err := r.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	var s SyncRequest
	readGolden(t, "sync_request.json", &s)
	if err := s.Validate(); err != nil {
		t.Errorf("sync_request.json: %v", err)
	}
	var rr RecordsRequest
	readGolden(t, "records_request.json", &rr)
	if err := rr.Validate(); err != nil {
		t.Errorf("records_request.json: %v", err)
	}
	var rep Report
	readGolden(t, "report.json", &rep)
	if err := rep.Validate(); err != nil {
		t.Errorf("report.json: %v", err)
	}
}

func readGolden(t *testing.T, name string, v any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}
