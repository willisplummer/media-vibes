// Package services provides external service integrations.
package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"media/models"
)

// TMDBService handles interactions with The Movie Database API
type TMDBService struct {
	apiKey string
	client *http.Client
}

// TMDBMovie represents a movie response from TMDB API
type TMDBMovie struct {
	ID          int         `json:"id"`
	Title       string      `json:"title"`
	Overview    string      `json:"overview"`
	ReleaseDate string      `json:"release_date"`
	PosterPath  string      `json:"poster_path"`
	VoteAverage float64     `json:"vote_average"`
	Runtime     int         `json:"runtime"`
	Genres      []Genre     `json:"genres"`
	Credits     Credits     `json:"credits"`
	ExternalIDs ExternalIDs `json:"external_ids"`
}

// Genre represents a movie genre from TMDB
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Credits contains cast and crew information
type Credits struct {
	Crew []CrewMember `json:"crew"`
}

// CrewMember represents a crew member in a movie
type CrewMember struct {
	Job  string `json:"job"`
	Name string `json:"name"`
}

// ExternalIDs contains external IDs for a movie
type ExternalIDs struct {
	IMDBID string `json:"imdb_id"`
}

// NewTMDBService creates a new TMDB service instance
func NewTMDBService(apiKey string) *TMDBService {
	return &TMDBService{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetMovie fetches movie details from TMDB by ID
// GetMovie fetches movie details from TMDB by ID
func (t *TMDBService) GetMovie(tmdbID int) (*models.Movie, error) {
	url := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?api_key=%s&"+
		"append_to_response=credits,external_ids", tmdbID, t.apiKey)

	resp, err := t.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch movie from TMDB: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API returned status %d", resp.StatusCode)
	}

	var tmdbMovie TMDBMovie
	if err := json.NewDecoder(resp.Body).Decode(&tmdbMovie); err != nil {
		return nil, fmt.Errorf("failed to decode TMDB response: %w", err)
	}

	return t.convertToMovie(tmdbMovie), nil
}

func (t *TMDBService) convertToMovie(tmdbMovie TMDBMovie) *models.Movie {
	movie := &models.Movie{
		Title:       tmdbMovie.Title,
		TMDBID:      tmdbMovie.ID,
		Description: tmdbMovie.Overview,
		Rating:      tmdbMovie.VoteAverage,
		Runtime:     tmdbMovie.Runtime,
		Status:      models.StatusWanted,
		IMDBID:      tmdbMovie.ExternalIDs.IMDBID,
	}

	// Parse release year
	if tmdbMovie.ReleaseDate != "" && len(tmdbMovie.ReleaseDate) >= 4 {
		var year int
		if _, err := fmt.Sscanf(tmdbMovie.ReleaseDate[:4], "%d", &year); err != nil {
			log.Printf("Failed to parse year from release date: %v", err)
		}
		movie.Year = year
	}

	// Set poster URL
	if tmdbMovie.PosterPath != "" {
		movie.Poster = fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", tmdbMovie.PosterPath)
	}

	// Extract genres
	if len(tmdbMovie.Genres) > 0 {
		genre := tmdbMovie.Genres[0].Name
		for i := 1; i < len(tmdbMovie.Genres) && i < 3; i++ {
			genre += ", " + tmdbMovie.Genres[i].Name
		}
		movie.Genre = genre
	}

	// Extract director
	for _, crew := range tmdbMovie.Credits.Crew {
		if crew.Job == "Director" {
			movie.Director = crew.Name
			break
		}
	}

	return movie
}
