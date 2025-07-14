// Package jobs provides background job processing functionality.
package jobs

import (
	"context"
	"log"
	"sync"
	"time"
)

// JobManager handles background job execution
type JobManager struct {
	torrentSearchJob *TorrentSearchJob
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	running          bool
	mu               sync.RWMutex
}

// NewJobManager creates a new job manager
func NewJobManager(torrentSearchJob *TorrentSearchJob) *JobManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &JobManager{
		torrentSearchJob: torrentSearchJob,
		ctx:              ctx,
		cancel:           cancel,
		running:          false,
	}
}

// Start begins the job manager background processing
func (jm *JobManager) Start() {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if jm.running {
		log.Println("Job manager is already running")
		return
	}

	jm.running = true
	log.Println("Starting job manager...")

	// Start periodic torrent search job
	jm.wg.Add(1)
	go jm.runPeriodicTorrentSearch()
}

// Stop stops the job manager
func (jm *JobManager) Stop() {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if !jm.running {
		return
	}

	log.Println("Stopping job manager...")
	jm.cancel()
	jm.running = false

	// Wait for all jobs to finish
	jm.wg.Wait()
	log.Println("Job manager stopped")
}

// IsRunning returns whether the job manager is currently running
func (jm *JobManager) IsRunning() bool {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.running
}

// TriggerTorrentSearchForMovie immediately triggers a torrent search for a specific movie
func (jm *JobManager) TriggerTorrentSearchForMovie(movieID int) {
	if jm.torrentSearchJob == nil {
		log.Printf("Cannot trigger torrent search: no torrent search job configured")
		return
	}
	
	jm.wg.Add(1)
	go func() {
		defer jm.wg.Done()
		if err := jm.torrentSearchJob.SearchForMovie(movieID); err != nil {
			log.Printf("Failed to search for torrents for movie %d: %v", movieID, err)
		}
	}()
}

// CancelJobsForMovie cancels any active jobs for a specific movie
func (jm *JobManager) CancelJobsForMovie(movieID int) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	if !jm.running {
		return
	}

	log.Printf("Cancelling jobs for movie %d", movieID)
	
	// If we have a torrent search job, use it to access services
	if jm.torrentSearchJob != nil {
		// Get the movie to check if it has an active torrent
		if movie, err := jm.torrentSearchJob.GetMovieByID(movieID); err == nil {
			// If movie is downloading and has a torrent hash, we could optionally pause/remove it
			// For now, we'll just log the cancellation and let the status update handle it
			if movie.Status == "downloading" && movie.TorrentHash != "" {
				log.Printf("Movie %d has active torrent %s, considering for cancellation", movieID, movie.TorrentHash)
			}
			
			// Update movie status to cancel the job
			if movie.Status == "searching" || movie.Status == "downloading" {
				movie.Status = "failed" // Mark as failed to stop further processing
				if err := jm.torrentSearchJob.UpdateMovie(movie); err != nil {
					log.Printf("Warning: failed to update movie status during job cancellation: %v", err)
				}
			}
		}
	}
}

// runPeriodicTorrentSearch runs the torrent search job periodically
func (jm *JobManager) runPeriodicTorrentSearch() {
	defer jm.wg.Done()

	// Skip if no torrent search job is configured
	if jm.torrentSearchJob == nil {
		log.Println("No torrent search job configured, skipping periodic search")
		<-jm.ctx.Done()
		return
	}

	// Run immediately on startup
	if err := jm.torrentSearchJob.ProcessMovieQueue(); err != nil {
		log.Printf("Initial torrent search failed: %v", err)
	}

	// Then run every 30 minutes
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-jm.ctx.Done():
			log.Println("Periodic torrent search job stopped")
			return
		case <-ticker.C:
			log.Println("Running periodic torrent search...")
			if err := jm.torrentSearchJob.ProcessMovieQueue(); err != nil {
				log.Printf("Periodic torrent search failed: %v", err)
			}
		}
	}
}
