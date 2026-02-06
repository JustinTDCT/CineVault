package stream

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func ServeDirectFile(w http.ResponseWriter, r *http.Request, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	fileSize := stat.Size()

	// Determine content type
	contentType := "application/octet-stream"
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".mp4"):
		contentType = "video/mp4"
	case strings.HasSuffix(lower, ".mkv"):
		contentType = "video/x-matroska"
	case strings.HasSuffix(lower, ".webm"):
		contentType = "video/webm"
	case strings.HasSuffix(lower, ".avi"):
		contentType = "video/x-msvideo"
	case strings.HasSuffix(lower, ".mp3"):
		contentType = "audio/mpeg"
	case strings.HasSuffix(lower, ".flac"):
		contentType = "audio/flac"
	case strings.HasSuffix(lower, ".m4a"):
		contentType = "audio/mp4"
	}

	// Handle range requests
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		return serveRange(w, file, fileSize, rangeHeader, contentType)
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, file)
	return nil
}

func serveRange(w http.ResponseWriter, file *os.File, fileSize int64, rangeHeader, contentType string) error {
	// Parse "bytes=start-end"
	rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.SplitN(rangeHeader, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid range header")
	}

	var start, end int64
	if parts[0] != "" {
		s, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return err
		}
		start = s
	}
	if parts[1] != "" {
		e, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return err
		}
		end = e
	} else {
		end = fileSize - 1
	}

	if start >= fileSize {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return nil
	}

	if end >= fileSize {
		end = fileSize - 1
	}

	length := end - start + 1
	file.Seek(start, io.SeekStart)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusPartialContent)

	io.CopyN(w, file, length)
	return nil
}
