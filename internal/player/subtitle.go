package player

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
)

type SubtitleTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Codec    string `json:"codec"`
	Forced   bool   `json:"forced"`
}

func ListSubtitles(ffprobePath, filePath string) ([]SubtitleTrack, error) {
	cmd := exec.Command(ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s",
		filePath)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var data struct {
		Streams []struct {
			Index     int               `json:"index"`
			CodecName string            `json:"codec_name"`
			Tags      map[string]string `json:"tags"`
			Disposition struct {
				Forced int `json:"forced"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, err
	}

	var tracks []SubtitleTrack
	for _, s := range data.Streams {
		tracks = append(tracks, SubtitleTrack{
			Index:    s.Index,
			Language: s.Tags["language"],
			Title:    s.Tags["title"],
			Codec:    s.CodecName,
			Forced:   s.Disposition.Forced == 1,
		})
	}
	return tracks, nil
}

func ExtractSubtitle(ffmpegPath, filePath string, trackIndex int, outDir string) (string, error) {
	outFile := filepath.Join(outDir, "sub.vtt")
	cmd := exec.Command(ffmpegPath,
		"-i", filePath,
		"-map", "0:"+strings.TrimSpace(string(rune('0'+trackIndex))),
		"-c:s", "webvtt",
		"-y", outFile)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return outFile, nil
}
