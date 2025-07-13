package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"media/database"
	"media/models"
	"media/repository"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func setupTestApp(t *testing.T) (*App, func()) {
	// Create a temporary test database
	testDB, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Initialize schema
	if err := testDB.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize test schema: %v", err)
	}

	movieRepo := repository.NewMovieRepository(testDB)
	app := &App{
		movieRepo: movieRepo,
	}

	// Return cleanup function
	cleanup := func() {
		if err := testDB.Close(); err != nil {
			t.Logf("Failed to close test database: %v", err)
		}
	}

	return app, cleanup
}

func createTestMovie(repo *repository.MovieRepository, title string) (*models.Movie, error) {
	movie := &models.Movie{
		Title:       title,
		Status:      models.StatusWanted,
		Year:        2023,
		Genre:       "Action",
		Description: "A test movie",
		Rating:      7.5,
		Runtime:     120,
		Director:    "Test Director",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := repo.Create(movie)
	return movie, err
}

func TestGetMoviesHandler_EmptyDatabase(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	req, err := http.NewRequest("GET", "/api/v1/movies", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.getMoviesHandler).Methods("GET")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var movies []models.Movie
	err = json.Unmarshal(rr.Body.Bytes(), &movies)
	assert.NoError(t, err)
	assert.Empty(t, movies)
}

func TestGetMoviesHandler_WithMovies(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create test movies
	movie1, err := createTestMovie(app.movieRepo, "Test Movie 1")
	assert.NoError(t, err)

	movie2, err := createTestMovie(app.movieRepo, "Test Movie 2")
	assert.NoError(t, err)

	req, err := http.NewRequest("GET", "/api/v1/movies", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.getMoviesHandler).Methods("GET")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var movies []models.Movie
	err = json.Unmarshal(rr.Body.Bytes(), &movies)
	assert.NoError(t, err)
	assert.Len(t, movies, 2)

	// Check that movies are returned (order might vary)
	titles := []string{movies[0].Title, movies[1].Title}
	assert.Contains(t, titles, movie1.Title)
	assert.Contains(t, titles, movie2.Title)
}

func TestGetMovieByIDHandler_ValidID(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create a test movie
	movie, err := createTestMovie(app.movieRepo, "Test Movie")
	assert.NoError(t, err)

	req, err := http.NewRequest("GET", "/api/v1/movies/1", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies/{id}", app.getMovieByIDHandler).Methods("GET")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var returnedMovie models.Movie
	err = json.Unmarshal(rr.Body.Bytes(), &returnedMovie)
	assert.NoError(t, err)
	assert.Equal(t, movie.Title, returnedMovie.Title)
	assert.Equal(t, movie.Status, returnedMovie.Status)
	assert.Equal(t, movie.Year, returnedMovie.Year)
}

func TestGetMovieByIDHandler_InvalidID(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	req, err := http.NewRequest("GET", "/api/v1/movies/invalid", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies/{id}", app.getMovieByIDHandler).Methods("GET")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetMovieByIDHandler_NotFound(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	req, err := http.NewRequest("GET", "/api/v1/movies/999", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies/{id}", app.getMovieByIDHandler).Methods("GET")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHealthHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}

func TestMain(m *testing.M) {
	// Setup code before tests
	code := m.Run()
	// Cleanup code after tests
	os.Exit(code)
}
