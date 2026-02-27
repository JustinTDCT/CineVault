package metadata

import (
	"regexp"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
)

// punctRe strips everything except letters, digits, and whitespace for title normalization.
var punctRe = regexp.MustCompile(`[^a-z0-9\s]+`)

type Scraper interface {
	Search(query string, mediaType models.MediaType, year *int) ([]*models.MetadataMatch, error)
	GetDetails(externalID string) (*models.MetadataMatch, error)
	Name() string
}

// articles are common words stripped before similarity comparison.
var articles = map[string]bool{"a": true, "an": true, "the": true}

// stripArticles removes common articles from a word list.
func stripArticles(words []string) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		if !articles[w] {
			out = append(out, w)
		}
	}
	return out
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			m := ins
			if del < m {
				m = del
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// fuzzyWordMatch returns true if two words are close enough to be considered a match.
func fuzzyWordMatch(a, b string) bool {
	if a == b {
		return true
	}
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen <= 2 {
		return a == b
	}
	threshold := 1
	if maxLen > 5 {
		threshold = 2
	}
	return levenshtein(a, b) <= threshold
}

// titleSimilarity computes a confidence score between a search query and a result title.
// Uses word overlap with fuzzy matching, article stripping, and containment scoring
// so that partial title matches (e.g. "American Tale Vol 1" vs "An American Tail")
// produce meaningful scores instead of near-zero.
func titleSimilarity(query, result string) float64 {
	q := strings.ToLower(strings.TrimSpace(query))
	r := strings.ToLower(strings.TrimSpace(result))

	if q == r {
		return 1.0
	}

	// Normalize punctuation so "Spider-Man" matches "Spiderman"
	q = punctRe.ReplaceAllString(q, " ")
	r = punctRe.ReplaceAllString(r, " ")

	qWords := stripArticles(strings.Fields(q))
	rWords := stripArticles(strings.Fields(r))
	if len(qWords) == 0 || len(rWords) == 0 {
		return 0.0
	}

	// After article stripping, re-check exact match
	qJoined := strings.Join(qWords, " ")
	rJoined := strings.Join(rWords, " ")
	if qJoined == rJoined {
		return 1.0
	}

	// Count fuzzy word matches
	matched := 0
	usedR := make([]bool, len(rWords))
	for _, qw := range qWords {
		for j, rw := range rWords {
			if !usedR[j] && fuzzyWordMatch(qw, rw) {
				matched++
				usedR[j] = true
				break
			}
		}
	}

	if matched == 0 {
		// Fall back to character-level similarity
		maxLen := len(qJoined)
		if len(rJoined) > maxLen {
			maxLen = len(rJoined)
		}
		if maxLen == 0 {
			return 0.0
		}
		return float64(maxLen-levenshtein(qJoined, rJoined)) / float64(maxLen) * 0.6
	}

	// Containment: how much of the shorter title is found in the longer
	minWords := len(qWords)
	if len(rWords) < minWords {
		minWords = len(rWords)
	}
	maxWords := len(qWords)
	if len(rWords) > maxWords {
		maxWords = len(rWords)
	}

	containment := float64(matched) / float64(minWords)
	jaccard := float64(matched) / float64(maxWords)

	// Weighted blend: containment matters more (handles subset matches)
	score := 0.65*containment + 0.35*jaccard

	// Mild penalty for large length differences
	lengthRatio := float64(minWords) / float64(maxWords)
	if lengthRatio < 0.5 {
		score *= 0.7 + 0.3*lengthRatio
	}

	if score > 1.0 {
		score = 1.0
	}
	return score
}
