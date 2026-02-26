package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// OMDbRatings holds the ratings fetched from the OMDb API.
type OMDbRatings struct {
	IMDBRating    *float64 // e.g. 7.8
	RTScore       *int     // Rotten Tomatoes critic % (0-100)
	AudienceScore *int     // Rotten Tomatoes audience % (0-100) – mapped from "Internet Movie Database" audience or Metacritic
}

// FetchOMDbRatings calls the OMDb API with the given IMDB ID and API key,
// returning IMDB rating, Rotten Tomatoes score, and audience score.
func FetchOMDbRatings(imdbID, apiKey string) (*OMDbRatings, error) {
	if imdbID == "" || apiKey == "" {
		return nil, fmt.Errorf("imdb_id and api_key are required")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	reqURL := fmt.Sprintf("http://www.omdbapi.com/?i=%s&apikey=%s", url.QueryEscape(imdbID), url.QueryEscape(apiKey))

	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var omdb struct {
		Response string `json:"Response"`
		Error    string `json:"Error"`
		IMDBRating string `json:"imdbRating"`
		Ratings []struct {
			Source string `json:"Source"`
			Value  string `json:"Value"`
		} `json:"Ratings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&omdb); err != nil {
		return nil, err
	}
	if omdb.Response == "False" {
		return nil, fmt.Errorf("OMDb error: %s", omdb.Error)
	}

	result := &OMDbRatings{}

	// Parse IMDB rating (e.g. "7.8")
	if omdb.IMDBRating != "" && omdb.IMDBRating != "N/A" {
		var r float64
		fmt.Sscanf(omdb.IMDBRating, "%f", &r)
		result.IMDBRating = &r
	}

	// Parse ratings array
	for _, rating := range omdb.Ratings {
		switch rating.Source {
		case "Rotten Tomatoes":
			// Value is like "92%"
			var pct int
			fmt.Sscanf(rating.Value, "%d%%", &pct)
			result.RTScore = &pct
		case "Metacritic":
			// Value is like "76/100" – use as audience score fallback
			if result.AudienceScore == nil {
				var score int
				fmt.Sscanf(rating.Value, "%d/", &score)
				result.AudienceScore = &score
			}
		}
	}

	return result, nil
}
