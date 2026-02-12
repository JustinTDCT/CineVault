package version

import (
	"encoding/json"
	"os"
	"sync"
)

// Info holds version metadata from version.json
type Info struct {
	Version  string `json:"version"`
	Phase    int    `json:"phase"`
	Released string `json:"released"`
	Notes    string `json:"notes"`
}

var (
	current Info
	once    sync.Once
)

// Get returns the current version info, reading from version.json on first call.
func Get() Info {
	once.Do(func() {
		data, err := os.ReadFile("version.json")
		if err != nil {
			current = Info{Version: "0.0.00", Phase: 0}
			return
		}
		json.Unmarshal(data, &current)
	})
	return current
}
