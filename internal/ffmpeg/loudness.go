package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const TargetLUFS = -14.0

type LoudnessResult struct {
	InputI  float64 // Measured integrated loudness (LUFS)
	GainDB  float64 // Gain to apply to reach TargetLUFS
}

// AnalyzeLoudness runs FFmpeg's loudnorm filter in measurement-only mode
// and returns the measured integrated loudness and computed gain offset.
func AnalyzeLoudness(ffmpegPath, filePath string) (*LoudnessResult, error) {
	args := []string{
		"-hide_banner",
		"-i", filePath,
		"-af", "loudnorm=I=-14:TP=-1.5:LRA=11:print_format=json",
		"-f", "null",
		"-",
	}

	cmd := exec.Command(ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg loudnorm: %w (output: %s)", err, lastLines(string(output), 20))
	}

	// The loudnorm JSON block is in stderr output; parse it
	inputI, err := parseLoudnormJSON(string(output))
	if err != nil {
		return nil, fmt.Errorf("parse loudnorm output: %w", err)
	}

	gainDB := TargetLUFS - inputI

	return &LoudnessResult{
		InputI: inputI,
		GainDB: gainDB,
	}, nil
}

type loudnormOutput struct {
	InputI string `json:"input_i"`
}

func parseLoudnormJSON(output string) (float64, error) {
	// Find the JSON block in the output — it starts with { and contains "input_i"
	idx := strings.LastIndex(output, "{")
	if idx < 0 {
		return 0, fmt.Errorf("no JSON block found in loudnorm output")
	}
	endIdx := strings.Index(output[idx:], "}")
	if endIdx < 0 {
		return 0, fmt.Errorf("no closing brace in loudnorm JSON")
	}
	jsonStr := output[idx : idx+endIdx+1]

	var parsed loudnormOutput
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return 0, fmt.Errorf("unmarshal loudnorm JSON: %w", err)
	}

	var inputI float64
	if _, err := fmt.Sscanf(parsed.InputI, "%f", &inputI); err != nil {
		return 0, fmt.Errorf("parse input_i %q: %w", parsed.InputI, err)
	}

	return inputI, nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
