package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type dirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	requestedPath := r.URL.Query().Get("path")
	if requestedPath == "" {
		requestedPath = "/media"
	}

	// Clean and validate the path - restrict to /media
	requestedPath = filepath.Clean(requestedPath)
	if !strings.HasPrefix(requestedPath, "/media") {
		requestedPath = "/media"
	}

	entries, err := os.ReadDir(requestedPath)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{
			Success: true,
			Data: map[string]interface{}{
				"path":    requestedPath,
				"parent":  filepath.Dir(requestedPath),
				"entries": []dirEntry{},
				"error":   "Cannot read directory",
			},
		})
		return
	}

	dirs := []dirEntry{}
	for _, e := range entries {
		// Skip hidden files/dirs
		if strings.HasPrefix(e.Name(), ".") || strings.HasPrefix(e.Name(), "@") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, dirEntry{
				Name:  e.Name(),
				Path:  filepath.Join(requestedPath, e.Name()),
				IsDir: true,
			})
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	parent := ""
	if requestedPath != "/media" {
		parent = filepath.Dir(requestedPath)
		if !strings.HasPrefix(parent, "/media") {
			parent = "/media"
		}
	}

	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"path":    requestedPath,
			"parent":  parent,
			"entries": dirs,
		},
	})
}
