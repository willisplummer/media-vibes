package jobs

import (
	"testing"
	"time"

	"media/database"
	"media/repository"

	"github.com/stretchr/testify/assert"
)

func setupTestJobManager(t *testing.T) (*JobManager, func()) {
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
	movieEventRepo := repository.NewMovieEventRepository(testDB)

	// Create a real TorrentSearchJob but with nil services for testing
	torrentSearchJob := NewTorrentSearchJob(movieRepo, movieEventRepo, nil, nil)
	jm := NewJobManager(torrentSearchJob)

	// Return cleanup function
	cleanup := func() {
		if jm.IsRunning() {
			jm.Stop()
		}
		if err := testDB.Close(); err != nil {
			t.Logf("Failed to close test database: %v", err)
		}
	}

	return jm, cleanup
}

func TestJobManager_NewJobManager(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	assert.NotNil(t, jm)
	assert.NotNil(t, jm.torrentSearchJob)
	assert.False(t, jm.IsRunning())
	assert.NotNil(t, jm.ctx)
	assert.NotNil(t, jm.cancel)
}

func TestJobManager_IsRunning(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	// Initially not running
	assert.False(t, jm.IsRunning())

	// After start, should be running
	jm.Start()
	assert.True(t, jm.IsRunning())

	// After stop, should not be running
	jm.Stop()
	assert.False(t, jm.IsRunning())
}

func TestJobManager_StartStop(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	// Start the job manager
	jm.Start()
	assert.True(t, jm.IsRunning())

	// Stop the job manager
	jm.Stop()
	assert.False(t, jm.IsRunning())
}

func TestJobManager_DoubleStart(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	// Start twice should not cause issues
	jm.Start()
	assert.True(t, jm.IsRunning())

	jm.Start() // Second start should be ignored
	assert.True(t, jm.IsRunning())

	jm.Stop()
	assert.False(t, jm.IsRunning())
}

func TestJobManager_DoubleStop(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	jm.Start()
	assert.True(t, jm.IsRunning())

	// Stop twice should not cause issues
	jm.Stop()
	assert.False(t, jm.IsRunning())

	jm.Stop() // Second stop should be ignored
	assert.False(t, jm.IsRunning())
}

func TestJobManager_StopWithoutStart(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	// Stop without start should not cause issues
	jm.Stop()
	assert.False(t, jm.IsRunning())
}

func TestJobManager_TriggerTorrentSearchForMovie(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	movieID := 123

	// Trigger search for a movie
	// Note: This will fail because we don't have Jackett service, but it shouldn't panic
	jm.TriggerTorrentSearchForMovie(movieID)

	// Wait for goroutine to complete (it will error but complete)
	jm.wg.Wait()

	// Test passes if no panic occurred
}

func TestJobManager_TriggerMultipleSearches(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	movieIDs := []int{1, 2, 3}

	// Trigger searches for multiple movies
	for _, id := range movieIDs {
		jm.TriggerTorrentSearchForMovie(id)
	}

	// Wait for all goroutines to complete
	jm.wg.Wait()

	// Test passes if no panic occurred
}

func TestJobManager_CancelJobsForMovie(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	movieID := 456

	// Test cancellation when not running
	jm.CancelJobsForMovie(movieID)
	// Should not panic or cause issues

	// Test cancellation when running
	jm.Start()
	jm.CancelJobsForMovie(movieID)
	// Should not panic or cause issues

	jm.Stop()
}

func TestJobManager_CancelJobsForMovieMultiple(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	jm.Start()

	// Cancel jobs for multiple movies
	movieIDs := []int{1, 2, 3}
	for _, id := range movieIDs {
		jm.CancelJobsForMovie(id)
	}

	// Should not cause any issues
	assert.True(t, jm.IsRunning())

	jm.Stop()
}

func TestJobManager_ConcurrentOperations(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	jm.Start()

	// Perform concurrent operations
	done := make(chan bool, 6)

	// Concurrent triggers
	go func() {
		jm.TriggerTorrentSearchForMovie(1)
		done <- true
	}()

	go func() {
		jm.TriggerTorrentSearchForMovie(2)
		done <- true
	}()

	// Concurrent cancellations
	go func() {
		jm.CancelJobsForMovie(3)
		done <- true
	}()

	go func() {
		jm.CancelJobsForMovie(4)
		done <- true
	}()

	// Concurrent status checks
	go func() {
		_ = jm.IsRunning()
		done <- true
	}()

	go func() {
		_ = jm.IsRunning()
		done <- true
	}()

	// Wait for all operations to complete
	for i := 0; i < 6; i++ {
		select {
		case <-done:
			// Operation completed
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out waiting for concurrent operations")
		}
	}

	// Give triggered searches a moment to complete
	time.Sleep(100 * time.Millisecond)

	// Test passes if no panic occurred

	jm.Stop()
}

func TestJobManager_StartStopCycle(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	// Multiple start/stop cycles
	for i := 0; i < 3; i++ {
		jm.Start()
		assert.True(t, jm.IsRunning())

		jm.TriggerTorrentSearchForMovie(i)

		jm.Stop()
		assert.False(t, jm.IsRunning())
	}

	// Wait for any remaining operations
	jm.wg.Wait()
}

func TestJobManager_EdgeCases(t *testing.T) {
	jm, cleanup := setupTestJobManager(t)
	defer cleanup()

	// Test with zero movie ID
	jm.TriggerTorrentSearchForMovie(0)
	jm.CancelJobsForMovie(0)

	// Test with negative movie ID
	jm.TriggerTorrentSearchForMovie(-1)
	jm.CancelJobsForMovie(-1)

	// Test with very large movie ID
	jm.TriggerTorrentSearchForMovie(999999999)
	jm.CancelJobsForMovie(999999999)

	// Wait for operations to complete
	jm.wg.Wait()

	// Test passes if no panic occurred
}

func TestJobManager_NilTorrentSearchJob(t *testing.T) {
	// Test that creating a job manager with nil torrent search job doesn't crash
	jm := NewJobManager(nil)
	assert.NotNil(t, jm)
	assert.Nil(t, jm.torrentSearchJob)

	// These operations should not panic even with nil job
	jm.Start()
	jm.CancelJobsForMovie(1)
	jm.Stop()

	// TriggerTorrentSearchForMovie might panic with nil job, which is expected behavior
	// We don't test it here as it would be a programming error
}