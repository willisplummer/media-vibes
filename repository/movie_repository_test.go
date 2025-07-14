package repository

import (
	"testing"
	"time"

	"media/database"
	"media/models"

	"github.com/stretchr/testify/assert"
)

func setupTestRepo(t *testing.T) (*MovieRepository, func()) {
	// Create a temporary test database
	testDB, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Initialize schema
	if err := testDB.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize test schema: %v", err)
	}

	repo := NewMovieRepository(testDB)

	// Return cleanup function
	cleanup := func() {
		if err := testDB.Close(); err != nil {
			t.Logf("Failed to close test database: %v", err)
		}
	}

	return repo, cleanup
}

func createTestMovieForRepo(repo *MovieRepository, title string) (*models.Movie, error) {
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

func TestMovieRepository_Delete_Success(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a test movie
	movie, err := createTestMovieForRepo(repo, "Movie to Delete")
	assert.NoError(t, err)
	assert.NotZero(t, movie.ID)

	// Verify movie exists
	retrievedMovie, err := repo.GetByID(movie.ID)
	assert.NoError(t, err)
	assert.Equal(t, movie.Title, retrievedMovie.Title)

	// Delete the movie
	err = repo.Delete(movie.ID)
	assert.NoError(t, err)

	// Verify movie no longer exists
	_, err = repo.GetByID(movie.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMovieRepository_Delete_NotFound(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Try to delete a non-existent movie
	err := repo.Delete(999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "movie with id 999 not found")
}

func TestMovieRepository_Delete_InvalidID(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Try to delete with invalid IDs
	testCases := []int{0, -1, -999}

	for _, movieID := range testCases {
		t.Run(string(rune(movieID)), func(t *testing.T) {
			err := repo.Delete(movieID)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not found")
		})
	}
}

func TestMovieRepository_Delete_MultipleMovies(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create multiple test movies
	movies := make([]*models.Movie, 3)
	for i := 0; i < 3; i++ {
		movie, err := createTestMovieForRepo(repo, "Movie "+string(rune('A'+i)))
		assert.NoError(t, err)
		movies[i] = movie
	}

	// Delete the middle movie
	err := repo.Delete(movies[1].ID)
	assert.NoError(t, err)

	// Verify first and third movies still exist
	_, err = repo.GetByID(movies[0].ID)
	assert.NoError(t, err)

	_, err = repo.GetByID(movies[2].ID)
	assert.NoError(t, err)

	// Verify middle movie is gone
	_, err = repo.GetByID(movies[1].ID)
	assert.Error(t, err)

	// Verify GetAll returns only 2 movies
	allMovies, err := repo.GetAll()
	assert.NoError(t, err)
	assert.Len(t, allMovies, 2)
}

func TestMovieRepository_Delete_DoubleDelete(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a test movie
	movie, err := createTestMovieForRepo(repo, "Double Delete Test")
	assert.NoError(t, err)

	// Delete the movie first time
	err = repo.Delete(movie.ID)
	assert.NoError(t, err)

	// Try to delete again - should fail
	err = repo.Delete(movie.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMovieRepository_Delete_AfterUpdate(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a test movie
	movie, err := createTestMovieForRepo(repo, "Update Then Delete Test")
	assert.NoError(t, err)

	// Update the movie
	movie.Title = "Updated Title"
	movie.Status = models.StatusDownloaded
	err = repo.Update(movie)
	assert.NoError(t, err)

	// Verify update worked
	updatedMovie, err := repo.GetByID(movie.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Title", updatedMovie.Title)
	assert.Equal(t, models.StatusDownloaded, updatedMovie.Status)

	// Delete the updated movie
	err = repo.Delete(movie.ID)
	assert.NoError(t, err)

	// Verify movie is gone
	_, err = repo.GetByID(movie.ID)
	assert.Error(t, err)
}

func TestMovieRepository_Delete_ComplexMovie(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a movie with all fields populated
	movie := &models.Movie{
		Title:       "Complex Movie",
		Status:      models.StatusReady,
		IMDBID:      "tt1234567",
		TMDBID:      12345,
		Year:        2023,
		Genre:       "Action/Adventure",
		Description: "A complex movie with all fields",
		Poster:      "https://example.com/poster.jpg",
		Rating:      9.0,
		Runtime:     150,
		Director:    "Famous Director",
		FilePath:    "/movies/complex_movie.mp4",
		FileSize:    4294967296,
		Quality:     "4K",
	}

	err := repo.Create(movie)
	assert.NoError(t, err)

	// Verify movie was created with all fields
	retrievedMovie, err := repo.GetByID(movie.ID)
	assert.NoError(t, err)
	assert.Equal(t, movie.Title, retrievedMovie.Title)
	assert.Equal(t, movie.IMDBID, retrievedMovie.IMDBID)
	assert.Equal(t, movie.FileSize, retrievedMovie.FileSize)

	// Delete the complex movie
	err = repo.Delete(movie.ID)
	assert.NoError(t, err)

	// Verify movie is gone
	_, err = repo.GetByID(movie.ID)
	assert.Error(t, err)
}

func TestMovieRepository_Delete_ConcurrentAccess(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a test movie
	movie, err := createTestMovieForRepo(repo, "Concurrent Access Test")
	assert.NoError(t, err)

	// Try to delete the same movie from multiple goroutines
	// Note: SQLite handles this by serializing transactions
	concurrency := 3
	results := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			err := repo.Delete(movie.ID)
			results <- err
		}()
	}

	// Collect results
	var errors []error
	for i := 0; i < concurrency; i++ {
		err := <-results
		errors = append(errors, err)
	}

	// One should succeed, others should fail
	successCount := 0
	errorCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	assert.Equal(t, 1, successCount, "Exactly one deletion should succeed")
	assert.Equal(t, concurrency-1, errorCount, "The rest should fail")

	// Verify movie is actually deleted
	_, err = repo.GetByID(movie.ID)
	assert.Error(t, err)
}

func TestMovieRepository_Delete_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create many movies
	numMovies := 100
	movieIDs := make([]int, numMovies)

	for i := 0; i < numMovies; i++ {
		movie, err := createTestMovieForRepo(repo, "Stress Test Movie "+string(rune('A'+i%26)))
		assert.NoError(t, err)
		movieIDs[i] = movie.ID
	}

	// Verify all movies exist
	allMovies, err := repo.GetAll()
	assert.NoError(t, err)
	assert.Len(t, allMovies, numMovies)

	// Delete all movies
	for _, id := range movieIDs {
		err := repo.Delete(id)
		assert.NoError(t, err)
	}

	// Verify all movies are gone
	allMovies, err = repo.GetAll()
	assert.NoError(t, err)
	assert.Len(t, allMovies, 0)
}

func TestMovieRepository_Delete_DatabaseIntegrity(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create movies
	movie1, err := createTestMovieForRepo(repo, "Movie 1")
	assert.NoError(t, err)

	movie2, err := createTestMovieForRepo(repo, "Movie 2")
	assert.NoError(t, err)

	movie3, err := createTestMovieForRepo(repo, "Movie 3")
	assert.NoError(t, err)

	// Delete middle movie
	err = repo.Delete(movie2.ID)
	assert.NoError(t, err)

	// Create a new movie - should get a new ID, not reuse the deleted one
	movie4, err := createTestMovieForRepo(repo, "Movie 4")
	assert.NoError(t, err)
	assert.NotEqual(t, movie2.ID, movie4.ID)

	// Verify database integrity
	allMovies, err := repo.GetAll()
	assert.NoError(t, err)
	assert.Len(t, allMovies, 3) // movies 1, 3, and 4

	// Verify correct movies exist
	existingIDs := make([]int, len(allMovies))
	for i, movie := range allMovies {
		existingIDs[i] = movie.ID
	}

	assert.Contains(t, existingIDs, movie1.ID)
	assert.NotContains(t, existingIDs, movie2.ID) // deleted
	assert.Contains(t, existingIDs, movie3.ID)
	assert.Contains(t, existingIDs, movie4.ID)
}

func TestMovieRepository_Delete_EdgeCaseIDs(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	edgeCaseIDs := []int{
		0,
		-1,
		-999999999,
		999999999,
		2147483647,  // max int32
		-2147483648, // min int32
	}

	for _, id := range edgeCaseIDs {
		t.Run(string(rune(id)), func(t *testing.T) {
			err := repo.Delete(id)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not found")
		})
	}
}