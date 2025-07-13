package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func TestCreateMovieHandler_Success(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movieJSON := `{
		"title": "Test Movie",
		"year": 2023,
		"genre": "Action",
		"description": "A test movie description",
		"rating": 8.5,
		"runtime": 120,
		"director": "Test Director"
	}`

	req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var createdMovie models.Movie
	err = json.Unmarshal(rr.Body.Bytes(), &createdMovie)
	assert.NoError(t, err)
	assert.Equal(t, "Test Movie", createdMovie.Title)
	assert.Equal(t, 2023, createdMovie.Year)
	assert.Equal(t, "Action", createdMovie.Genre)
	assert.Equal(t, models.StatusWanted, createdMovie.Status) // Default status
	assert.NotZero(t, createdMovie.ID)
	assert.NotZero(t, createdMovie.CreatedAt)
	assert.NotZero(t, createdMovie.UpdatedAt)
}

func TestCreateMovieHandler_WithStatus(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movieJSON := `{
		"title": "Test Movie with Status",
		"status": "downloaded"
	}`

	req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var createdMovie models.Movie
	err = json.Unmarshal(rr.Body.Bytes(), &createdMovie)
	assert.NoError(t, err)
	assert.Equal(t, models.StatusDownloaded, createdMovie.Status)
}

func TestCreateMovieHandler_InvalidJSON(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	invalidJSON := `{"title": "Test Movie", "year": "not a number"}`

	req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(invalidJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid request body")
}

func TestCreateMovieHandler_MissingTitle(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movieJSON := `{
		"year": 2023,
		"genre": "Action"
	}`

	req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Title is required")
}

func TestCreateMovieHandler_EmptyTitle(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movieJSON := `{
		"title": "",
		"year": 2023
	}`

	req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Title is required")
}

func TestCreateMovieHandler_CompleteMovie(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movieJSON := `{
		"title": "Complete Test Movie",
		"status": "ready",
		"imdb_id": "tt1234567",
		"tmdb_id": 12345,
		"year": 2023,
		"genre": "Action/Adventure",
		"description": "A complete test movie with all fields",
		"poster": "https://example.com/poster.jpg",
		"rating": 9.0,
		"runtime": 150,
		"director": "Famous Director",
		"file_path": "/movies/complete_test_movie.mp4",
		"file_size": 4294967296,
		"quality": "1080p"
	}`

	req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var createdMovie models.Movie
	err = json.Unmarshal(rr.Body.Bytes(), &createdMovie)
	assert.NoError(t, err)
	assert.Equal(t, "Complete Test Movie", createdMovie.Title)
	assert.Equal(t, models.StatusReady, createdMovie.Status)
	assert.Equal(t, "tt1234567", createdMovie.IMDBID)
	assert.Equal(t, 12345, createdMovie.TMDBID)
	assert.Equal(t, 2023, createdMovie.Year)
	assert.Equal(t, "Action/Adventure", createdMovie.Genre)
	assert.Equal(t, "A complete test movie with all fields", createdMovie.Description)
	assert.Equal(t, "https://example.com/poster.jpg", createdMovie.Poster)
	assert.Equal(t, 9.0, createdMovie.Rating)
	assert.Equal(t, 150, createdMovie.Runtime)
	assert.Equal(t, "Famous Director", createdMovie.Director)
	assert.Equal(t, "/movies/complete_test_movie.mp4", createdMovie.FilePath)
	assert.Equal(t, int64(4294967296), createdMovie.FileSize)
	assert.Equal(t, "1080p", createdMovie.Quality)
}

func TestMovieRepository_Create_Success(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movie := &models.Movie{
		Title:       "Repository Test Movie",
		Status:      models.StatusWanted,
		Year:        2023,
		Genre:       "Comedy",
		Description: "A test for repository create method",
		Rating:      7.8,
		Runtime:     95,
		Director:    "Test Director",
	}

	err := app.movieRepo.Create(movie)
	assert.NoError(t, err)
	
	// Check that the movie was assigned an ID
	assert.NotZero(t, movie.ID)
	assert.NotZero(t, movie.CreatedAt)
	assert.NotZero(t, movie.UpdatedAt)

	// Verify the movie was actually saved by retrieving it
	retrievedMovie, err := app.movieRepo.GetByID(movie.ID)
	assert.NoError(t, err)
	assert.Equal(t, movie.Title, retrievedMovie.Title)
	assert.Equal(t, movie.Status, retrievedMovie.Status)
	assert.Equal(t, movie.Year, retrievedMovie.Year)
}

func TestMovieRepository_Create_WithAllFields(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movie := &models.Movie{
		Title:       "Complete Repository Test",
		Status:      models.StatusReady,
		IMDBID:      "tt9876543",
		TMDBID:      54321,
		Year:        2022,
		Genre:       "Drama/Thriller",
		Description: "A complete movie with all fields for repository testing",
		Poster:      "https://example.com/poster.jpg",
		Rating:      8.9,
		Runtime:     140,
		Director:    "Famous Director",
		FilePath:    "/movies/complete_repo_test.mp4",
		FileSize:    2147483648,
		Quality:     "4K",
	}

	err := app.movieRepo.Create(movie)
	assert.NoError(t, err)
	assert.NotZero(t, movie.ID)

	// Verify all fields were saved correctly
	retrievedMovie, err := app.movieRepo.GetByID(movie.ID)
	assert.NoError(t, err)
	assert.Equal(t, movie.Title, retrievedMovie.Title)
	assert.Equal(t, movie.Status, retrievedMovie.Status)
	assert.Equal(t, movie.IMDBID, retrievedMovie.IMDBID)
	assert.Equal(t, movie.TMDBID, retrievedMovie.TMDBID)
	assert.Equal(t, movie.Year, retrievedMovie.Year)
	assert.Equal(t, movie.Genre, retrievedMovie.Genre)
	assert.Equal(t, movie.Description, retrievedMovie.Description)
	assert.Equal(t, movie.Poster, retrievedMovie.Poster)
	assert.Equal(t, movie.Rating, retrievedMovie.Rating)
	assert.Equal(t, movie.Runtime, retrievedMovie.Runtime)
	assert.Equal(t, movie.Director, retrievedMovie.Director)
	assert.Equal(t, movie.FilePath, retrievedMovie.FilePath)
	assert.Equal(t, movie.FileSize, retrievedMovie.FileSize)
	assert.Equal(t, movie.Quality, retrievedMovie.Quality)
}

func TestMovieRepository_Create_MinimalMovie(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movie := &models.Movie{
		Title:  "Minimal Movie",
		Status: models.StatusWanted,
	}

	err := app.movieRepo.Create(movie)
	assert.NoError(t, err)
	assert.NotZero(t, movie.ID)

	// Verify the minimal movie was saved
	retrievedMovie, err := app.movieRepo.GetByID(movie.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Minimal Movie", retrievedMovie.Title)
	assert.Equal(t, models.StatusWanted, retrievedMovie.Status)
	
	// Check that optional fields are zero values
	assert.Empty(t, retrievedMovie.IMDBID)
	assert.Zero(t, retrievedMovie.TMDBID)
	assert.Zero(t, retrievedMovie.Year)
	assert.Empty(t, retrievedMovie.Genre)
	assert.Empty(t, retrievedMovie.Description)
}

func TestMovieRepository_Create_MultipleMovies(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movies := []*models.Movie{
		{Title: "Movie 1", Status: models.StatusWanted},
		{Title: "Movie 2", Status: models.StatusDownloaded},
		{Title: "Movie 3", Status: models.StatusReady},
	}

	for _, movie := range movies {
		err := app.movieRepo.Create(movie)
		assert.NoError(t, err)
		assert.NotZero(t, movie.ID)
	}

	// Verify all movies were created with unique IDs
	ids := make(map[int]bool)
	for _, movie := range movies {
		assert.False(t, ids[movie.ID], "Duplicate ID found")
		ids[movie.ID] = true
	}

	// Verify we can retrieve all movies
	allMovies, err := app.movieRepo.GetAll()
	assert.NoError(t, err)
	assert.Len(t, allMovies, 3)
}

func TestMovieRepository_Create_TimestampsSet(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	before := time.Now()
	
	movie := &models.Movie{
		Title:  "Timestamp Test",
		Status: models.StatusWanted,
	}

	err := app.movieRepo.Create(movie)
	assert.NoError(t, err)
	
	after := time.Now()

	// Check that timestamps are set and within reasonable bounds
	assert.True(t, movie.CreatedAt.After(before) || movie.CreatedAt.Equal(before))
	assert.True(t, movie.CreatedAt.Before(after) || movie.CreatedAt.Equal(after))
	assert.True(t, movie.UpdatedAt.After(before) || movie.UpdatedAt.Equal(before))
	assert.True(t, movie.UpdatedAt.Before(after) || movie.UpdatedAt.Equal(after))
	
	// CreatedAt and UpdatedAt should be very close (same transaction)
	timeDiff := movie.UpdatedAt.Sub(movie.CreatedAt)
	assert.True(t, timeDiff >= 0 && timeDiff < time.Second)
}

func TestCreateMovieHandler_ValidationEdgeCases(t *testing.T) {
	testCases := []struct {
		name           string
		movieJSON      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "empty JSON object",
			movieJSON:      `{}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Title is required",
		},
		{
			name:           "only whitespace title",
			movieJSON:      `{"title": "   "}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Title is required",
		},
		{
			name:           "null title",
			movieJSON:      `{"title": null}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Title is required",
		},
		{
			name:           "very long title",
			movieJSON:      `{"title": "` + strings.Repeat("A", 1000) + `"}`,
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
		{
			name:           "special characters in title",
			movieJSON:      `{"title": "Movie with émojis 🎬 & special chars: àáâãäåæçèé"}`,
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
		{
			name:           "invalid status",
			movieJSON:      `{"title": "Test Movie", "status": "invalid_status"}`,
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
		{
			name:           "negative year",
			movieJSON:      `{"title": "Test Movie", "year": -1}`,
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
		{
			name:           "future year",
			movieJSON:      `{"title": "Test Movie", "year": 3000}`,
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
		{
			name:           "negative rating",
			movieJSON:      `{"title": "Test Movie", "rating": -5.0}`,
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
		{
			name:           "rating over 10",
			movieJSON:      `{"title": "Test Movie", "rating": 15.0}`,
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
		{
			name:           "negative runtime",
			movieJSON:      `{"title": "Test Movie", "runtime": -30}`,
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
		{
			name:           "extremely large runtime",
			movieJSON:      `{"title": "Test Movie", "runtime": 999999}`,
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
		{
			name:           "negative file size",
			movieJSON:      `{"title": "Test Movie", "file_size": -1}`,
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app, cleanup := setupTestApp(t)
			defer cleanup()

			req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(tc.movieJSON))
			assert.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			router := mux.NewRouter()
			router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
			router.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			if tc.expectedError != "" {
				assert.Contains(t, rr.Body.String(), tc.expectedError)
			}
		})
	}
}

func TestCreateMovieHandler_StatusValidation(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	validStatuses := []models.MediaStatus{
		models.StatusWanted,
		models.StatusDownloading,
		models.StatusDownloaded,
		models.StatusProcessing,
		models.StatusReady,
		models.StatusFailed,
	}

	for _, status := range validStatuses {
		t.Run(string(status), func(t *testing.T) {
			movieJSON := `{"title": "Status Test Movie", "status": "` + string(status) + `"}`

			req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
			assert.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			router := mux.NewRouter()
			router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			var createdMovie models.Movie
			err = json.Unmarshal(rr.Body.Bytes(), &createdMovie)
			assert.NoError(t, err)
			assert.Equal(t, status, createdMovie.Status)
		})
	}
}

func TestCreateMovieHandler_ContentTypeValidation(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movieJSON := `{"title": "Content Type Test"}`

	testCases := []struct {
		name        string
		contentType string
		expectError bool
	}{
		{
			name:        "valid JSON content type",
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "JSON with charset",
			contentType: "application/json; charset=utf-8",
			expectError: false,
		},
		{
			name:        "no content type",
			contentType: "",
			expectError: false, // Go's http package is lenient
		},
		{
			name:        "wrong content type",
			contentType: "text/plain",
			expectError: false, // Handler doesn't explicitly check content type
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
			assert.NoError(t, err)
			
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}

			rr := httptest.NewRecorder()

			router := mux.NewRouter()
			router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
			router.ServeHTTP(rr, req)

			if tc.expectError {
				assert.NotEqual(t, http.StatusCreated, rr.Code)
			} else {
				assert.Equal(t, http.StatusCreated, rr.Code)
			}
		})
	}
}

func TestCreateMovieHandler_MalformedJSON(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	malformedJSONTests := []struct {
		name string
		json string
	}{
		{"missing closing brace", `{"title": "Test Movie"`},
		{"missing quotes on key", `{title: "Test Movie"}`},
		{"trailing comma", `{"title": "Test Movie",}`},
		{"unescaped quotes", `{"title": "Test "Movie""}`},
		{"invalid unicode", `{"title": "Test Movie\u"}`},
		{"empty string", ``},
		{"only whitespace", `   `},
		{"array instead of object", `["title", "Test Movie"]`},
		{"string instead of object", `"Test Movie"`},
		{"number instead of object", `123`},
	}

	for _, test := range malformedJSONTests {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(test.json))
			assert.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			router := mux.NewRouter()
			router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), "Invalid request body")
		})
	}
}

func TestCreateMovieHandler_NullJSON(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader("null"))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
	router.ServeHTTP(rr, req)

	// null JSON unmarshals to an empty Movie struct, so it should fail validation
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Title is required")
}

func TestCreateMovieHandler_DatabaseErrorHandling(t *testing.T) {
	// Test what happens when we close the database connection
	app, cleanup := setupTestApp(t)
	
	// Close the database to simulate a database error
	cleanup()

	movieJSON := `{"title": "Database Error Test"}`

	req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Failed to create movie")
}

func TestCreateMovieHandler_EmptyBody(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(""))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid request body")
}

func TestCreateMovieHandler_LargePayload(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create a very large JSON payload
	largeTitle := strings.Repeat("A", 100000)
	movieJSON := `{"title": "` + largeTitle + `"}`

	req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
	router.ServeHTTP(rr, req)

	// Should still work, but test that it doesn't crash
	// The actual behavior depends on server limits
	assert.True(t, rr.Code == http.StatusCreated || rr.Code == http.StatusBadRequest)
}

func TestCreateMovieHandler_ConcurrentCreation(t *testing.T) {
	// Test that the handler can handle multiple requests without panicking
	// Each goroutine gets its own app instance to avoid SQLite concurrency issues
	
	concurrency := 3
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			// Each goroutine gets its own database connection
			app, cleanup := setupTestApp(t)
			defer cleanup()
			
			movieJSON := fmt.Sprintf(`{"title": "Concurrent Movie %d"}`, id)

			req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
			if err != nil {
				t.Errorf("Failed to create request: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			router := mux.NewRouter()
			router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
			router.ServeHTTP(rr, req)

			// Should succeed since each has its own database
			if rr.Code != http.StatusCreated {
				t.Errorf("Expected 201 Created, got %d", rr.Code)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < concurrency; i++ {
		select {
		case <-done:
			// Goroutine completed
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out waiting for goroutines")
		}
	}
}

func TestCreateMovieHandler_HTTPMethods(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	movieJSON := `{"title": "Method Test Movie"}`

	// Test invalid HTTP methods
	invalidMethods := []string{"GET", "PUT", "DELETE", "PATCH"}

	for _, method := range invalidMethods {
		t.Run(method, func(t *testing.T) {
			req, err := http.NewRequest(method, "/api/v1/movies", strings.NewReader(movieJSON))
			assert.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			router := mux.NewRouter()
			router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
			router.ServeHTTP(rr, req)

			// Should return 405 Method Not Allowed or 404 Not Found
			assert.True(t, rr.Code == http.StatusMethodNotAllowed || rr.Code == http.StatusNotFound)
		})
	}
}

func TestCreateMovieHandler_MaxFieldLengths(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Test extremely long values for various fields
	testCases := []struct {
		name  string
		field string
		value string
	}{
		{"long title", "title", strings.Repeat("T", 10000)},
		{"long genre", "genre", strings.Repeat("G", 5000)},
		{"long description", "description", strings.Repeat("D", 50000)},
		{"long director", "director", strings.Repeat("R", 1000)},
		{"long poster URL", "poster", "https://example.com/" + strings.Repeat("p", 5000)},
		{"long file path", "file_path", "/movies/" + strings.Repeat("f", 5000) + ".mp4"},
		{"long IMDB ID", "imdb_id", "tt" + strings.Repeat("1", 1000)},
		{"long quality", "quality", strings.Repeat("4", 1000) + "K"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			movieJSON := fmt.Sprintf(`{"%s": "%s"}`, tc.field, tc.value)

			req, err := http.NewRequest("POST", "/api/v1/movies", strings.NewReader(movieJSON))
			assert.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			router := mux.NewRouter()
			router.HandleFunc("/api/v1/movies", app.createMovieHandler).Methods("POST")
			router.ServeHTTP(rr, req)

			// Should handle gracefully - either succeed or fail with proper error
			assert.True(t, rr.Code == http.StatusCreated || rr.Code == http.StatusBadRequest || rr.Code == http.StatusInternalServerError)
		})
	}
}

func TestMain(m *testing.M) {
	// Setup code before tests
	code := m.Run()
	// Cleanup code after tests
	os.Exit(code)
}
