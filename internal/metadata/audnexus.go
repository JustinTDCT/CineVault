package metadata

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
)

const audnexusBaseURL = "https://api.audnex.us"

type AudnexusScraper struct {
	client *http.Client
}

func NewAudnexusScraper() *AudnexusScraper {
	return &AudnexusScraper{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *AudnexusScraper) Name() string { return "audnexus" }

type audnexusBook struct {
	ASIN          string           `json:"asin"`
	Title         string           `json:"title"`
	Authors       []audnexusPerson `json:"authors"`
	Narrators     []audnexusPerson `json:"narrators"`
	Description   string           `json:"description"`
	Summary       string           `json:"summary"`
	Image         string           `json:"image"`
	Genres        []audnexusGenre  `json:"genres"`
	Rating        string           `json:"rating"`
	ReleaseDate   string           `json:"releaseDate"`
	RuntimeMin    int              `json:"runtimeLengthMin"`
	PublisherName string           `json:"publisherName"`
	Language      string           `json:"language"`
	ISBN          string           `json:"isbn"`
	Copyright     int              `json:"copyright"`
	FormatType    string           `json:"formatType"`
	IsAdult       bool             `json:"isAdult"`
}

type audnexusPerson struct {
	ASIN string `json:"asin"`
	Name string `json:"name"`
}

type audnexusGenre struct {
	ASIN string `json:"asin"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type audnexusAuthor struct {
	ASIN        string          `json:"asin"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Image       string          `json:"image"`
	Genres      []audnexusGenre `json:"genres"`
}

// Search is a no-op since Audnexus has no book-by-title search.
// Audnexus is used via direct ASIN lookup and as an enrichment layer.
func (s *AudnexusScraper) Search(query string, mediaType models.MediaType, year *int) ([]*models.MetadataMatch, error) {
	return nil, nil
}

// GetDetails fetches a book by its ASIN directly.
func (s *AudnexusScraper) GetDetails(externalID string) (*models.MetadataMatch, error) {
	book, err := s.getBook(externalID)
	if err != nil {
		return nil, err
	}
	return s.bookToMatch(book, 1.0), nil
}

// LookupByASIN fetches book details directly by ASIN.
func (s *AudnexusScraper) LookupByASIN(asin string) (*models.MetadataMatch, error) {
	return s.GetDetails(asin)
}

// SearchAuthors searches for authors by name.
func (s *AudnexusScraper) SearchAuthors(name string) ([]audnexusAuthor, error) {
	reqURL := fmt.Sprintf("%s/authors?name=%s&region=us", audnexusBaseURL, url.QueryEscape(name))
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("audnexus author search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("audnexus author search returned %d", resp.StatusCode)
	}

	var authors []audnexusAuthor
	if err := json.NewDecoder(resp.Body).Decode(&authors); err != nil {
		return nil, fmt.Errorf("audnexus author decode: %w", err)
	}
	return authors, nil
}

// EnrichMatch attempts to enrich an existing match with Audnexus data
// by searching for the author and looking up matching books.
func (s *AudnexusScraper) EnrichMatch(match *models.MetadataMatch) *models.MetadataMatch {
	if match == nil || match.ArtistName == "" {
		return match
	}
	log.Printf("Audnexus enrich: searching for author %q", match.ArtistName)
	_, err := s.SearchAuthors(match.ArtistName)
	if err != nil {
		log.Printf("Audnexus enrich: author search failed: %v", err)
	}
	return match
}

func (s *AudnexusScraper) getBook(asin string) (*audnexusBook, error) {
	reqURL := fmt.Sprintf("%s/books/%s?region=us", audnexusBaseURL, url.PathEscape(asin))
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("audnexus book lookup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("audnexus book lookup returned %d", resp.StatusCode)
	}

	var book audnexusBook
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return nil, fmt.Errorf("audnexus book decode: %w", err)
	}
	return &book, nil
}

func (s *AudnexusScraper) bookToMatch(book *audnexusBook, confidence float64) *models.MetadataMatch {
	match := &models.MetadataMatch{
		Source:      "audnexus",
		ExternalID:  book.ASIN,
		Title:       book.Title,
		Confidence:  confidence,
		ASIN:        book.ASIN,
		RuntimeMins: book.RuntimeMin,
		Publisher:   book.PublisherName,
	}

	desc := book.Summary
	if desc == "" {
		desc = book.Description
	}
	if desc != "" {
		match.Description = &desc
	}

	if book.Image != "" {
		match.PosterURL = &book.Image
	}

	if book.Rating != "" {
		var r float64
		fmt.Sscanf(book.Rating, "%f", &r)
		if r > 0 {
			match.Rating = &r
		}
	}

	if book.Copyright > 0 {
		y := book.Copyright
		match.Year = &y
	} else if len(book.ReleaseDate) >= 4 {
		var y int
		fmt.Sscanf(book.ReleaseDate[:4], "%d", &y)
		if y > 0 {
			match.Year = &y
		}
	}

	for _, g := range book.Genres {
		if g.Name != "" {
			match.Genres = append(match.Genres, g.Name)
		}
	}

	var authorNames []string
	for _, a := range book.Authors {
		if a.Name != "" {
			authorNames = append(authorNames, a.Name)
		}
	}
	if len(authorNames) > 0 {
		match.ArtistName = strings.Join(authorNames, ", ")
	}

	var narratorNames []string
	for _, n := range book.Narrators {
		if n.Name != "" {
			narratorNames = append(narratorNames, n.Name)
		}
	}
	if len(narratorNames) > 0 {
		match.Narrator = strings.Join(narratorNames, ", ")
	}

	if book.Language != "" {
		match.OriginalLanguage = &book.Language
	}

	return match
}
