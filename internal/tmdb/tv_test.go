package tmdb

import (
	"context"
	"net/http"
	"testing"
)

// Trimmed copies of real TMDb responses for 3 Body Problem.
const (
	searchTBP = `{"page":1,"results":[{"id":108545,"name":"3 Body Problem","original_name":"3 Body Problem","first_air_date":"2024-03-21","popularity":90.5}],"total_results":1}`
	showTBP   = `{"id":108545,"name":"3 Body Problem","original_name":"3 Body Problem","overview":"A young woman's fateful decision...",
	  "tagline":"","first_air_date":"2024-03-21","episode_run_time":[],"poster_path":"/ykZ7hlShkdRQaL2aiieXdEMmrLb.jpg",
	  "backdrop_path":"/bd.jpg","vote_average":7.5,"status":"Returning Series","genres":[{"id":10765,"name":"Sci-Fi & Fantasy"},{"id":18,"name":"Drama"}],
	  "external_ids":{"imdb_id":"tt13016388"},"seasons":[{"season_number":1,"name":"Season 1","episode_count":8}]}`
	seasonTBP = `{"season_number":1,"name":"Season 1","overview":"","poster_path":"/s1.jpg","air_date":"2024-03-21","episodes":[
	  {"id":3768381,"season_number":1,"episode_number":1,"name":"Countdown","overview":"A physicist...","air_date":"2024-03-21","runtime":60,"still_path":"/e1.jpg","vote_average":7.6},
	  {"id":4977041,"season_number":1,"episode_number":2,"name":"Red Coast","overview":"","air_date":"2024-03-21","runtime":52,"still_path":"/e2.jpg","vote_average":7.4}]}`
)

func TestTV(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/search/tv":
			if r.URL.Query().Get("query") != "3 Body Problem" || r.URL.Query().Get("first_air_date_year") != "2024" {
				t.Errorf("search query = %s", r.URL.RawQuery)
			}
			w.Write([]byte(searchTBP))
		case "/3/tv/108545":
			if r.URL.Query().Get("append_to_response") != "external_ids" {
				t.Errorf("details query = %s", r.URL.RawQuery)
			}
			w.Write([]byte(showTBP))
		case "/3/tv/108545/season/1":
			w.Write([]byte(seasonTBP))
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	res, err := c.SearchTV(ctx, "3 Body Problem", 2024)
	if err != nil || len(res) != 1 || res[0].Title != "3 Body Problem" || res[0].Year() != 2024 {
		t.Fatalf("search = %+v, %v", res, err)
	}
	show, err := c.TV(ctx, 108545)
	if err != nil || show.ExternalIDs.IMDbID != "tt13016388" || len(show.Seasons) != 1 || show.Seasons[0].EpisodeCount != 8 {
		t.Fatalf("show = %+v, %v", show, err)
	}
	m := show.AsMovie()
	if m.Title != "3 Body Problem" || m.Year() != 2024 || m.IMDbID != "tt13016388" || len(m.GenreNames()) != 2 || m.Runtime != 0 {
		t.Errorf("as movie = %+v", m)
	}
	season, err := c.Season(ctx, 108545, 1)
	if err != nil || len(season.Episodes) != 2 || season.Episodes[1].Name != "Red Coast" || season.Episodes[0].Runtime != 60 {
		t.Errorf("season = %+v, %v", season, err)
	}
	if _, err := c.Season(ctx, 108545, 9); err != ErrNotFound {
		t.Errorf("missing season: %v", err)
	}
}
