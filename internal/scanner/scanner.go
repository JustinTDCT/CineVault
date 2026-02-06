package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"github.com/JustinTDCT/CineVault/internal/ffmpeg"
	"github.com/JustinTDCT/CineVault/internal/models"
)

type Scanner struct{ ffprobe *ffmpeg.FFprobe }
type ScanResult struct {
	FilesFound   int
	FilesAdded   int
	Errors       []error
}

var videoExtensions = map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".mov": true}

func NewScanner(ffprobePath string) *Scanner {
	return &Scanner{ffprobe: ffmpeg.NewFFprobe(ffprobePath)}
}

func (s *Scanner) ScanLibrary(library *models.Library) (*ScanResult, error) {
	result := &ScanResult{Errors: make([]error, 0)}
	err := filepath.Walk(library.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !videoExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		result.FilesFound++
		_, err = s.ffprobe.Probe(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to probe %s: %w", path, err))
			return nil
		}
		result.FilesAdded++
		return nil
	})
	return result, err
}
