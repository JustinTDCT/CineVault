package detection

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// Detector provides skip segment detection for media items using FFmpeg.
type Detector struct {
	ffmpegPath string
}

func NewDetector(ffmpegPath string) *Detector {
	return &Detector{ffmpegPath: ffmpegPath}
}

// ──────────────────── 1. Intro Detection (Cross-Episode Audio Fingerprint) ────────────────────

// AudioSegmentFingerprint represents a fingerprint of a short audio window.
type AudioSegmentFingerprint struct {
	StartSec float64
	EndSec   float64
	Hash     string
}

// DetectIntros compares audio fingerprints across episodes in a season to find
// the repeated intro sequence. Returns detected intro segments for each episode.
func (d *Detector) DetectIntros(episodes []*models.MediaItem) map[uuid.UUID]*models.MediaSegment {
	if len(episodes) < 2 {
		return nil
	}

	// Extract fingerprints for the first 5 minutes of each episode in 15-second windows
	type episodeFP struct {
		item         *models.MediaItem
		fingerprints []AudioSegmentFingerprint
	}

	var epFPs []episodeFP
	for _, ep := range episodes {
		fps := d.extractAudioFingerprints(ep.FilePath, 0, 300, 15)
		if len(fps) > 0 {
			epFPs = append(epFPs, episodeFP{item: ep, fingerprints: fps})
		}
	}

	if len(epFPs) < 2 {
		return nil
	}

	// Compare fingerprints between episode pairs to find repeated segments
	type matchInfo struct {
		startSec float64
		endSec   float64
		count    int
	}

	// For each fingerprint window in each episode, count how many other episodes share it
	hashCounts := make(map[string][]struct {
		epIdx    int
		startSec float64
		endSec   float64
	})

	for epIdx, ep := range epFPs {
		for _, fp := range ep.fingerprints {
			hashCounts[fp.Hash] = append(hashCounts[fp.Hash], struct {
				epIdx    int
				startSec float64
				endSec   float64
			}{epIdx, fp.StartSec, fp.EndSec})
		}
	}

	// Find hashes that appear in at least 60% of episodes (likely intro)
	threshold := int(math.Ceil(float64(len(epFPs)) * 0.6))
	var commonWindows []matchInfo
	seen := make(map[string]bool)

	for hash, entries := range hashCounts {
		// Count unique episodes
		epSet := make(map[int]bool)
		for _, e := range entries {
			epSet[e.epIdx] = true
		}
		if len(epSet) >= threshold && !seen[hash] {
			seen[hash] = true
			// Use the first episode's timing as reference
			commonWindows = append(commonWindows, matchInfo{
				startSec: entries[0].startSec,
				endSec:   entries[0].endSec,
				count:    len(epSet),
			})
		}
	}

	if len(commonWindows) == 0 {
		return nil
	}

	// Sort by start time and merge contiguous windows into one intro region
	sort.Slice(commonWindows, func(i, j int) bool {
		return commonWindows[i].startSec < commonWindows[j].startSec
	})

	introStart := commonWindows[0].startSec
	introEnd := commonWindows[0].endSec
	for _, w := range commonWindows[1:] {
		if w.startSec <= introEnd+2 { // Allow 2-second gap for merge
			if w.endSec > introEnd {
				introEnd = w.endSec
			}
		}
	}

	// Only report if intro is at least 15 seconds long
	if introEnd-introStart < 15 {
		return nil
	}

	confidence := float64(len(commonWindows)) / float64(len(epFPs[0].fingerprints))
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Apply the detected intro region to all episodes
	results := make(map[uuid.UUID]*models.MediaSegment)
	for _, ep := range epFPs {
		results[ep.item.ID] = &models.MediaSegment{
			ID:           uuid.New(),
			MediaItemID:  ep.item.ID,
			SegmentType:  models.SegmentIntro,
			StartSeconds: introStart,
			EndSeconds:   introEnd,
			Confidence:   confidence,
			Source:       models.SegmentSourceAuto,
		}
	}

	return results
}

// extractAudioFingerprints generates audio fingerprints in fixed windows.
func (d *Detector) extractAudioFingerprints(filePath string, startSec, endSec float64, windowSec float64) []AudioSegmentFingerprint {
	var fingerprints []AudioSegmentFingerprint

	for t := startSec; t+windowSec <= endSec; t += windowSec {
		// Extract audio stats for this window
		cmd := exec.Command(d.ffmpegPath,
			"-ss", fmt.Sprintf("%.1f", t),
			"-t", fmt.Sprintf("%.1f", windowSec),
			"-i", filePath,
			"-af", "astats=metadata=1:reset=1",
			"-vn", "-f", "null", "-",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}

		// Hash the audio stats output to create a fingerprint
		hash := md5.Sum(output)
		fingerprints = append(fingerprints, AudioSegmentFingerprint{
			StartSec: t,
			EndSec:   t + windowSec,
			Hash:     fmt.Sprintf("%x", hash),
		})
	}

	return fingerprints
}

// ──────────────────── 2. Credits Detection (Black Frame + Silence) ────────────────────

// blackFrameEvent represents a detected black frame region.
type blackFrameEvent struct {
	Start    float64
	End      float64
	Duration float64
}

// silenceEvent represents a detected silence region.
type silenceEvent struct {
	Start    float64
	End      float64
	Duration float64
}

// DetectCredits uses FFmpeg's blackdetect and silencedetect filters to find
// the start of end credits. Typically credits follow a fade-to-black pattern.
func (d *Detector) DetectCredits(item *models.MediaItem) *models.MediaSegment {
	if item.DurationSeconds == nil || *item.DurationSeconds < 120 {
		return nil
	}
	duration := float64(*item.DurationSeconds)

	// Only analyze the last 20% of the video (credits are at the end)
	analyzeStart := duration * 0.80
	analyzeLen := duration - analyzeStart

	// Detect black frames
	blackFrames := d.detectBlackFrames(item.FilePath, analyzeStart, analyzeLen)

	// Detect silence
	silences := d.detectSilence(item.FilePath, analyzeStart, analyzeLen)

	// Find a black frame + silence combination near each other (within 5 seconds)
	// that marks the start of credits
	var creditsStart float64

	if len(blackFrames) > 0 {
		// Look for the earliest significant black frame (>0.5s) in the last portion
		for _, bf := range blackFrames {
			if bf.Duration >= 0.5 {
				// Check if there's silence nearby
				hasSilence := false
				for _, s := range silences {
					if math.Abs(s.Start-bf.Start) < 5.0 || math.Abs(s.End-bf.End) < 5.0 {
						hasSilence = true
						break
					}
				}
				if hasSilence || bf.Duration >= 1.0 {
					creditsStart = bf.Start + analyzeStart
					break
				}
			}
		}
	}

	// Fallback: use the first significant silence in the last portion
	if creditsStart == 0 && len(silences) > 0 {
		for _, s := range silences {
			if s.Duration >= 2.0 {
				creditsStart = s.Start + analyzeStart
				break
			}
		}
	}

	if creditsStart == 0 {
		return nil
	}

	// Snap credits to keyframe (round to nearest second)
	creditsStart = math.Floor(creditsStart)

	// Credits must be at least 30 seconds from the end and at most 10 minutes
	remaining := duration - creditsStart
	if remaining < 30 || remaining > 600 {
		return nil
	}

	confidence := 0.7
	if len(blackFrames) > 0 && len(silences) > 0 {
		confidence = 0.85
	}

	return &models.MediaSegment{
		ID:           uuid.New(),
		MediaItemID:  item.ID,
		SegmentType:  models.SegmentCredits,
		StartSeconds: creditsStart,
		EndSeconds:   duration,
		Confidence:   confidence,
		Source:       models.SegmentSourceAuto,
	}
}

// detectBlackFrames uses FFmpeg blackdetect filter.
func (d *Detector) detectBlackFrames(filePath string, startSec, durationSec float64) []blackFrameEvent {
	cmd := exec.Command(d.ffmpegPath,
		"-ss", fmt.Sprintf("%.1f", startSec),
		"-t", fmt.Sprintf("%.1f", durationSec),
		"-i", filePath,
		"-vf", "blackdetect=d=0.5:pix_th=0.10",
		"-an", "-f", "null", "-",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	return parseBlackDetect(string(output))
}

// detectSilence uses FFmpeg silencedetect filter.
func (d *Detector) detectSilence(filePath string, startSec, durationSec float64) []silenceEvent {
	cmd := exec.Command(d.ffmpegPath,
		"-ss", fmt.Sprintf("%.1f", startSec),
		"-t", fmt.Sprintf("%.1f", durationSec),
		"-i", filePath,
		"-af", "silencedetect=n=-50dB:d=2",
		"-vn", "-f", "null", "-",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	return parseSilenceDetect(string(output))
}

var blackStartRe = regexp.MustCompile(`black_start:(\d+\.?\d*)`)
var blackEndRe = regexp.MustCompile(`black_end:(\d+\.?\d*)`)
var blackDurRe = regexp.MustCompile(`black_duration:(\d+\.?\d*)`)

func parseBlackDetect(output string) []blackFrameEvent {
	var events []blackFrameEvent
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "black_start") {
			continue
		}
		startMatch := blackStartRe.FindStringSubmatch(line)
		endMatch := blackEndRe.FindStringSubmatch(line)
		durMatch := blackDurRe.FindStringSubmatch(line)
		if len(startMatch) < 2 || len(endMatch) < 2 {
			continue
		}
		start, _ := strconv.ParseFloat(startMatch[1], 64)
		end, _ := strconv.ParseFloat(endMatch[1], 64)
		dur := end - start
		if len(durMatch) >= 2 {
			dur, _ = strconv.ParseFloat(durMatch[1], 64)
		}
		events = append(events, blackFrameEvent{Start: start, End: end, Duration: dur})
	}
	return events
}

var silenceStartRe = regexp.MustCompile(`silence_start:\s*(\d+\.?\d*)`)
var silenceEndRe = regexp.MustCompile(`silence_end:\s*(\d+\.?\d*)`)
var silenceDurRe = regexp.MustCompile(`silence_duration:\s*(\d+\.?\d*)`)

func parseSilenceDetect(output string) []silenceEvent {
	var events []silenceEvent
	lines := strings.Split(output, "\n")

	var currentStart float64
	startSet := false

	for _, line := range lines {
		if sm := silenceStartRe.FindStringSubmatch(line); len(sm) >= 2 {
			currentStart, _ = strconv.ParseFloat(sm[1], 64)
			startSet = true
		}
		if em := silenceEndRe.FindStringSubmatch(line); len(em) >= 2 && startSet {
			end, _ := strconv.ParseFloat(em[1], 64)
			dur := end - currentStart
			if dm := silenceDurRe.FindStringSubmatch(line); len(dm) >= 2 {
				dur, _ = strconv.ParseFloat(dm[1], 64)
			}
			events = append(events, silenceEvent{Start: currentStart, End: end, Duration: dur})
			startSet = false
		}
	}
	return events
}

// ──────────────────── 3. Anime OP/ED Detection ────────────────────

// DetectAnimeSegments uses genre/keyword heuristics combined with timing patterns
// to detect anime opening (OP) and ending (ED) sequences.
// Anime OPs are typically 85-95 seconds, usually in the first 3 minutes.
// Anime EDs are typically 85-95 seconds, usually in the last 5 minutes.
func (d *Detector) DetectAnimeSegments(item *models.MediaItem) []*models.MediaSegment {
	if item.DurationSeconds == nil || *item.DurationSeconds < 300 {
		return nil
	}
	duration := float64(*item.DurationSeconds)

	var segments []*models.MediaSegment

	// ── Detect OP: Scan first 4 minutes for a ~90-second music segment ──
	opSegment := d.detectAnimeOP(item, duration)
	if opSegment != nil {
		segments = append(segments, opSegment)
	}

	// ── Detect ED: Scan last 6 minutes for a ~90-second music segment ──
	edSegment := d.detectAnimeED(item, duration)
	if edSegment != nil {
		segments = append(segments, edSegment)
	}

	return segments
}

func (d *Detector) detectAnimeOP(item *models.MediaItem, duration float64) *models.MediaSegment {
	// Anime OPs typically start between 0:00-3:00 and last 85-95 seconds
	// We look for a silence boundary in the first 4 minutes that marks the end of the OP

	silences := d.detectSilence(item.FilePath, 0, math.Min(240, duration))
	if len(silences) == 0 {
		return nil
	}

	// Look for a silence event around the 85-95 second mark (end of OP)
	// or around 170-195 seconds (OP with cold open that starts around 1:20-1:40)
	for _, s := range silences {
		// Standard OP: starts at 0, ends around 85-95s
		if s.Start >= 80 && s.Start <= 100 {
			return &models.MediaSegment{
				ID:           uuid.New(),
				MediaItemID:  item.ID,
				SegmentType:  models.SegmentIntro,
				StartSeconds: 0,
				EndSeconds:   math.Floor(s.Start),
				Confidence:   0.75,
				Source:       models.SegmentSourceAuto,
			}
		}

		// Cold-open OP: cold open ends with silence, then OP starts
		if s.Start >= 30 && s.Start <= 120 {
			// Check for another silence ~90 seconds after this one (end of OP)
			opStart := s.Start
			for _, s2 := range silences {
				opLen := s2.Start - opStart
				if opLen >= 80 && opLen <= 100 {
					return &models.MediaSegment{
						ID:           uuid.New(),
						MediaItemID:  item.ID,
						SegmentType:  models.SegmentIntro,
						StartSeconds: math.Floor(opStart),
						EndSeconds:   math.Floor(s2.Start),
						Confidence:   0.70,
						Source:       models.SegmentSourceAuto,
					}
				}
			}
		}
	}

	return nil
}

func (d *Detector) detectAnimeED(item *models.MediaItem, duration float64) *models.MediaSegment {
	// Anime EDs typically start 90-120 seconds before the end
	analyzeStart := math.Max(0, duration-360)
	analyzeLen := duration - analyzeStart

	silences := d.detectSilence(item.FilePath, analyzeStart, analyzeLen)
	if len(silences) == 0 {
		return nil
	}

	// Look for a silence event that marks the start of the ED
	// ED is typically ~90 seconds before the end
	for _, s := range silences {
		absTime := s.Start + analyzeStart
		remaining := duration - absTime
		if remaining >= 80 && remaining <= 120 {
			return &models.MediaSegment{
				ID:           uuid.New(),
				MediaItemID:  item.ID,
				SegmentType:  models.SegmentCredits,
				StartSeconds: math.Floor(absTime),
				EndSeconds:   duration,
				Confidence:   0.70,
				Source:       models.SegmentSourceAuto,
			}
		}
	}

	return nil
}

// IsAnimeContent checks if a media item is likely anime based on genres/keywords.
func IsAnimeContent(item *models.MediaItem) bool {
	if item.Keywords != nil {
		kw := strings.ToLower(*item.Keywords)
		if strings.Contains(kw, "anime") || strings.Contains(kw, "animation") {
			return true
		}
	}
	if item.OriginalLanguage != nil {
		lang := strings.ToLower(*item.OriginalLanguage)
		if lang == "ja" || lang == "japanese" {
			return true
		}
	}
	return false
}

// ──────────────────── 4. Recap Detection ────────────────────

// SceneChange represents a detected scene change.
type SceneChange struct {
	Timestamp float64
	Score     float64
}

// DetectRecap attempts to find recap segments by analyzing scene-change density.
// Recaps typically have rapid cuts (high scene-change frequency) in the first few minutes,
// before the intro. This is the hardest detection type and has lower confidence.
func (d *Detector) DetectRecap(item *models.MediaItem, introStart float64) *models.MediaSegment {
	if item.DurationSeconds == nil || *item.DurationSeconds < 300 {
		return nil
	}

	// Only look before the intro (recaps come before intros)
	// If no intro detected, analyze first 3 minutes
	analyzeEnd := introStart
	if analyzeEnd <= 0 {
		analyzeEnd = 180
	}
	if analyzeEnd < 30 {
		return nil // Not enough content before intro for a recap
	}

	// Detect scene changes in the pre-intro section
	changes := d.detectSceneChanges(item.FilePath, 0, analyzeEnd)
	if len(changes) < 5 {
		return nil // Too few scene changes for a recap
	}

	// Calculate scene-change density (changes per second)
	// Recaps typically have >0.5 changes/second (rapid cuts)
	density := float64(len(changes)) / analyzeEnd

	if density < 0.3 {
		return nil // Not dense enough to be a recap
	}

	// Find the region with the highest density (sliding 30-second window)
	type window struct {
		start   float64
		end     float64
		count   int
		density float64
	}

	var bestWindow *window
	windowSize := 30.0

	for start := 0.0; start+windowSize <= analyzeEnd; start += 5.0 {
		count := 0
		for _, sc := range changes {
			if sc.Timestamp >= start && sc.Timestamp <= start+windowSize {
				count++
			}
		}
		d := float64(count) / windowSize
		if bestWindow == nil || d > bestWindow.density {
			bestWindow = &window{start: start, end: start + windowSize, count: count, density: d}
		}
	}

	if bestWindow == nil || bestWindow.density < 0.4 {
		return nil
	}

	// Extend the recap region to include rapid-cut sections
	recapStart := bestWindow.start
	recapEnd := bestWindow.end

	// Extend backwards
	for _, sc := range changes {
		if sc.Timestamp < recapStart && recapStart-sc.Timestamp < 10 {
			recapStart = sc.Timestamp
		}
	}

	// Recap must be at least 10 seconds
	if recapEnd-recapStart < 10 {
		return nil
	}

	confidence := math.Min(0.6, bestWindow.density/1.0)

	return &models.MediaSegment{
		ID:           uuid.New(),
		MediaItemID:  item.ID,
		SegmentType:  models.SegmentRecap,
		StartSeconds: math.Floor(recapStart),
		EndSeconds:   math.Floor(recapEnd),
		Confidence:   confidence,
		Source:       models.SegmentSourceAuto,
	}
}

// detectSceneChanges uses FFmpeg's scene detection filter.
func (d *Detector) detectSceneChanges(filePath string, startSec, durationSec float64) []SceneChange {
	cmd := exec.Command(d.ffmpegPath,
		"-ss", fmt.Sprintf("%.1f", startSec),
		"-t", fmt.Sprintf("%.1f", durationSec),
		"-i", filePath,
		"-vf", "select='gt(scene,0.3)',showinfo",
		"-vsync", "vfr",
		"-f", "null", "-",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Scene change detection failed for %s: %v", filePath, err)
		return nil
	}

	return parseSceneChanges(string(output))
}

var ptsTimeRe = regexp.MustCompile(`pts_time:(\d+\.?\d*)`)

func parseSceneChanges(output string) []SceneChange {
	var changes []SceneChange
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "pts_time") {
			continue
		}
		m := ptsTimeRe.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		ts, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		changes = append(changes, SceneChange{Timestamp: ts, Score: 0.3})
	}
	return changes
}

// ──────────────────── Detection Result ────────────────────

// DetectionResult holds all detected segments for a media item.
type DetectionResult struct {
	MediaItemID uuid.UUID                `json:"media_item_id"`
	Segments    []*models.MediaSegment   `json:"segments"`
	IsAnime     bool                     `json:"is_anime"`
}

// DetectAll runs all applicable detection algorithms on a single media item.
// For anime content, uses anime-specific OP/ED detection instead of generic intro detection.
func (d *Detector) DetectAll(item *models.MediaItem) *DetectionResult {
	result := &DetectionResult{
		MediaItemID: item.ID,
		IsAnime:     IsAnimeContent(item),
	}

	if result.IsAnime {
		// Use anime-specific detection
		animeSegs := d.DetectAnimeSegments(item)
		result.Segments = append(result.Segments, animeSegs...)
	}

	// Credits detection (works for all content types)
	creditsSeg := d.DetectCredits(item)
	if creditsSeg != nil {
		// Don't add credits if anime ED already detected at a similar position
		addCredits := true
		for _, seg := range result.Segments {
			if seg.SegmentType == models.SegmentCredits {
				addCredits = false
				break
			}
		}
		if addCredits {
			result.Segments = append(result.Segments, creditsSeg)
		}
	}

	// Recap detection (only for TV episodes, not first episodes)
	if item.EpisodeNumber != nil && *item.EpisodeNumber > 1 {
		var introStart float64
		for _, seg := range result.Segments {
			if seg.SegmentType == models.SegmentIntro {
				introStart = seg.StartSeconds
				break
			}
		}
		recapSeg := d.DetectRecap(item, introStart)
		if recapSeg != nil {
			result.Segments = append(result.Segments, recapSeg)
		}
	}

	return result
}

// Suppress unused import warnings
var _ = json.Marshal
