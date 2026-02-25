package version

import (
	"encoding/json"
	"log"
	"os"
)

type Info struct {
	Version string `json:"version"`
}

func Load() Info {
	data, err := os.ReadFile("version.json")
	if err != nil {
		log.Printf("warning: could not read version.json: %v", err)
		return Info{Version: "0.0.0"}
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		log.Printf("warning: could not parse version.json: %v", err)
		return Info{Version: "0.0.0"}
	}
	return info
}
