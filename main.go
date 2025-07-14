// Package main provides the main entry point for the media management application.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"media/database"
	"media/jobs"
	"media/models"
	"media/repository"
	"media/services"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

// App represents the application with its dependencies
type App struct {
	movieRepo          *repository.MovieRepository
	movieEventRepo     *repository.MovieEventRepository
	tmdbService        *services.TMDBService
	jackettService     *services.JackettService
	qbittorrentService *services.QBittorrentService
	jobManager         *jobs.JobManager
}

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Initialize database
	db, err := database.NewDB("media.db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	// Initialize schema
	if err := db.InitSchema(); err != nil {
		log.Fatal("Failed to initialize schema:", err)
	}

	// Initialize repositories
	movieRepo := repository.NewMovieRepository(db)
	movieEventRepo := repository.NewMovieEventRepository(db)

	// Initialize TMDB service
	tmdbAPIKey := os.Getenv("TMDB_API_KEY")
	if tmdbAPIKey == "" {
		log.Fatal("TMDB_API_KEY environment variable is required")
	}
	tmdbService := services.NewTMDBService(tmdbAPIKey)

	// Initialize Jackett service
	jackettURL := os.Getenv("JACKETT_URL")
	if jackettURL == "" {
		jackettURL = "http://localhost:9117" // Default Jackett URL
	}
	jackettAPIKey := os.Getenv("JACKETT_API_KEY")
	var jackettService *services.JackettService
	var qbittorrentService *services.QBittorrentService
	var jobManager *jobs.JobManager

	if jackettAPIKey != "" {
		jackettService = services.NewJackettService(jackettURL, jackettAPIKey)
		log.Println("Jackett integration enabled")
	} else {
		log.Println("Warning: JACKETT_API_KEY not set - torrent search will be disabled")
	}

	// Initialize qBittorrent service
	qbittorrentURL := os.Getenv("QBITTORRENT_URL")
	if qbittorrentURL == "" {
		qbittorrentURL = "http://localhost:8081" // Default qBittorrent WebUI URL
	}
	qbittorrentUsername := os.Getenv("QBITTORRENT_USERNAME")
	qbittorrentPassword := os.Getenv("QBITTORRENT_PASSWORD")

	if qbittorrentUsername != "" && qbittorrentPassword != "" {
		qbittorrentService = services.NewQBittorrentService(qbittorrentURL, qbittorrentUsername, qbittorrentPassword)

		// Test qBittorrent connection
		if err := qbittorrentService.TestConnection(); err != nil {
			log.Printf("Warning: qBittorrent connection failed: %v", err)
			log.Println("Torrents will be found but not automatically downloaded")
			qbittorrentService = nil
		} else {
			log.Println("qBittorrent integration enabled")
		}
	} else {
		log.Println("Warning: qBittorrent credentials not set - torrents will not be downloaded automatically")
	}

	// Initialize job system if Jackett is available
	if jackettService != nil {
		torrentSearchJob := jobs.NewTorrentSearchJob(movieRepo, movieEventRepo, jackettService, qbittorrentService)
		jobManager = jobs.NewJobManager(torrentSearchJob)

		// Start job manager
		jobManager.Start()
	}

	app := &App{
		movieRepo:          movieRepo,
		movieEventRepo:     movieEventRepo,
		tmdbService:        tmdbService,
		jackettService:     jackettService,
		qbittorrentService: qbittorrentService,
		jobManager:         jobManager,
	}

	r := mux.NewRouter()

	// Health check endpoint
	r.HandleFunc("/health", healthHandler).Methods("GET")

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Movie endpoints
	api.HandleFunc("/movies", app.getMoviesHandler).Methods("GET")
	api.HandleFunc("/movies/{id}", app.getMovieByIDHandler).Methods("GET")
	api.HandleFunc("/movies/{id}/details", app.getMovieDetailsHandler).Methods("GET")
	api.HandleFunc("/movies/{id}/restart-job", app.restartMovieJobHandler).Methods("POST")
	api.HandleFunc("/movies/{id}/cancel-job", app.cancelMovieJobHandler).Methods("POST")
	api.HandleFunc("/movies/{id}", app.deleteMovieHandler).Methods("DELETE")
	api.HandleFunc("/movies", app.createMovieHandler).Methods("POST")
	api.HandleFunc("/movies/tmdb/{tmdb_id}", app.addMovieFromTMDBHandler).Methods("POST")

	// Generic media endpoints (still stubbed)
	api.HandleFunc("/media", getMediaHandler).Methods("GET")
	api.HandleFunc("/media", createMediaHandler).Methods("POST")
	api.HandleFunc("/media/{id}", getMediaByIDHandler).Methods("GET")
	api.HandleFunc("/media/{id}", updateMediaHandler).Methods("PUT")
	api.HandleFunc("/media/{id}", deleteMediaHandler).Methods("DELETE")

	// Search and release endpoints
	api.HandleFunc("/search", searchHandler).Methods("GET")
	api.HandleFunc("/releases", getReleasesHandler).Methods("GET")
	api.HandleFunc("/download", requestDownloadHandler).Methods("POST")

	log.Println("Server starting on :8080")
	server := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	defer func() {
		if jobManager != nil {
			jobManager.Stop()
		}
	}()

	log.Fatal(server.ListenAndServe())
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func (app *App) getMoviesHandler(w http.ResponseWriter, _ *http.Request) {
	movies, err := app.movieRepo.GetAll()
	if err != nil {
		log.Printf("Error getting movies: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(movies); err != nil {
		log.Printf("Error encoding movies: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (app *App) getMovieByIDHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Parse ID (simplified for now)
	var movieID int
	if _, err := fmt.Sscanf(id, "%d", &movieID); err != nil {
		http.Error(w, "Invalid movie ID", http.StatusBadRequest)
		return
	}

	movie, err := app.movieRepo.GetByID(movieID)
	if err != nil {
		log.Printf("Error getting movie by ID: %v", err)
		http.Error(w, "Movie not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(movie); err != nil {
		log.Printf("Error encoding movie: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (app *App) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	var movie models.Movie

	// Decode the request body
	if err := json.NewDecoder(r.Body).Decode(&movie); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if strings.TrimSpace(movie.Title) == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	// Set default status if not provided
	if movie.Status == "" {
		movie.Status = models.StatusWanted
	}

	// Create the movie
	if err := app.movieRepo.Create(&movie); err != nil {
		log.Printf("Error creating movie: %v", err)
		http.Error(w, "Failed to create movie", http.StatusInternalServerError)
		return
	}

	// Trigger torrent search if Jackett is available
	if app.jobManager != nil {
		log.Printf("Triggering torrent search for newly created movie: %s (%d)", movie.Title, movie.Year)
		app.jobManager.TriggerTorrentSearchForMovie(movie.ID)
	}

	// Return the created movie
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(movie); err != nil {
		log.Printf("Error encoding movie response: %v", err)
	}
}

func (app *App) addMovieFromTMDBHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tmdbIDStr := vars["tmdb_id"]

	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil {
		http.Error(w, "Invalid TMDB ID", http.StatusBadRequest)
		return
	}

	// Check if movie already exists
	movies, err := app.movieRepo.GetAll()
	if err != nil {
		log.Printf("Error checking existing movies: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	for _, movie := range movies {
		if movie.TMDBID == tmdbID {
			http.Error(w, "Movie already exists in library", http.StatusConflict)
			return
		}
	}

	// Fetch movie data from TMDB
	movie, err := app.tmdbService.GetMovie(tmdbID)
	if err != nil {
		log.Printf("Error fetching movie from TMDB: %v", err)
		http.Error(w, "Failed to fetch movie from TMDB", http.StatusBadRequest)
		return
	}

	// Save movie to database
	if err := app.movieRepo.Create(movie); err != nil {
		log.Printf("Error creating movie: %v", err)
		http.Error(w, "Failed to save movie", http.StatusInternalServerError)
		return
	}

	// Trigger torrent search if Jackett is available
	if app.jobManager != nil {
		log.Printf("Triggering torrent search for newly added movie: %s (%d)", movie.Title, movie.Year)
		app.jobManager.TriggerTorrentSearchForMovie(movie.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(movie); err != nil {
		log.Printf("Error encoding movie response: %v", err)
	}
}

// Generic media handlers (still stubbed)
func getMediaHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	if _, err := w.Write([]byte("Not implemented")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func createMediaHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	if _, err := w.Write([]byte("Not implemented")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func getMediaByIDHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	if _, err := w.Write([]byte("Not implemented")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func updateMediaHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	if _, err := w.Write([]byte("Not implemented")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func deleteMediaHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	if _, err := w.Write([]byte("Not implemented")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func searchHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	if _, err := w.Write([]byte("Not implemented")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func getReleasesHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	if _, err := w.Write([]byte("Not implemented")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func requestDownloadHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	if _, err := w.Write([]byte("Not implemented")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// getMovieDetailsHandler returns detailed information about a movie including events and job control
func (app *App) getMovieDetailsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	movieID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid movie ID", http.StatusBadRequest)
		return
	}

	// Get the movie
	movie, err := app.movieRepo.GetByID(movieID)
	if err != nil {
		http.Error(w, "Movie not found", http.StatusNotFound)
		return
	}

	// Get movie events
	var events []models.MovieEvent
	if app.movieEventRepo != nil {
		events, err = app.movieEventRepo.GetByMovieID(movieID)
		if err != nil {
			log.Printf("Failed to get movie events: %v", err)
			events = []models.MovieEvent{} // Empty slice on error
		}
	}

	// Get statistics
	var stats *models.MovieStats
	if app.movieEventRepo != nil {
		stats, err = app.movieEventRepo.GetStatistics(movieID)
		if err != nil {
			log.Printf("Failed to get movie statistics: %v", err)
			stats = &models.MovieStats{} // Empty stats on error
		}
	} else {
		stats = &models.MovieStats{}
	}

	// Build job control
	jobControl := &models.JobControl{
		CanCancel:  movie.Status == models.StatusDownloading || movie.Status == models.StatusSearching,
		CanRestart: movie.Status == models.StatusNotFound || movie.Status == models.StatusWanted || movie.Status == models.StatusFailed,
		CancelURL:  fmt.Sprintf("/api/v1/movies/%d/cancel-job", movieID),
		RestartURL: fmt.Sprintf("/api/v1/movies/%d/restart-job", movieID),
		CurrentJob: string(movie.Status),
	}

	// Determine if there's actually an active job based on recent events
	if len(events) > 0 {
		recentEvent := events[0] // Most recent event
		// If the most recent event was within the last 5 minutes and indicates active work
		if time.Since(recentEvent.CreatedAt) < 5*time.Minute {
			switch recentEvent.Type {
			case models.EventSearchStarted:
				jobControl.CurrentJob = "searching"
				jobControl.CanCancel = true // Allow canceling active search
			case models.EventDownloadStarted:
				jobControl.CurrentJob = "downloading"
				jobControl.CanCancel = true
				jobControl.CanRestart = false
			}
		}
	}

	// Add last job time if available
	if len(events) > 0 {
		jobControl.LastJobTime = events[0].CreatedAt.Format(time.RFC3339)
	}

	response := &models.DetailedMovieResponse{
		Movie:      movie,
		Events:     events,
		JobControl: jobControl,
		Statistics: stats,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// restartMovieJobHandler restarts the torrent search job for a movie
func (app *App) restartMovieJobHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	movieID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid movie ID", http.StatusBadRequest)
		return
	}

	// Check if movie exists
	movie, err := app.movieRepo.GetByID(movieID)
	if err != nil {
		http.Error(w, "Movie not found", http.StatusNotFound)
		return
	}

	// Log job restart event
	if app.movieEventRepo != nil {
		if err := app.movieEventRepo.Create(movieID, models.EventStatusChanged,
			"Job restarted manually", map[string]interface{}{"action": "manual_restart"}); err != nil {
			log.Printf("Failed to log job restart: %v", err)
		}
	}

	// Update movie status to wanted to trigger new search
	movie.Status = models.StatusWanted
	if err := app.movieRepo.Update(movie); err != nil {
		log.Printf("Failed to update movie status: %v", err)
		http.Error(w, "Failed to restart job", http.StatusInternalServerError)
		return
	}

	// Queue the search job immediately if job manager is available
	if app.jobManager != nil {
		app.jobManager.TriggerTorrentSearchForMovie(movieID)
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"message":  "Job restarted successfully",
		"movie_id": movieID,
		"status":   "wanted",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// cancelMovieJobHandler cancels any active job for a movie
func (app *App) cancelMovieJobHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	movieID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid movie ID", http.StatusBadRequest)
		return
	}

	// Check if movie exists
	movie, err := app.movieRepo.GetByID(movieID)
	if err != nil {
		http.Error(w, "Movie not found", http.StatusNotFound)
		return
	}

	// Check if there's actually something to cancel
	if movie.Status != models.StatusDownloading && movie.Status != models.StatusSearching {
		http.Error(w, "No active job to cancel", http.StatusBadRequest)
		return
	}

	// Log job cancellation
	if app.movieEventRepo != nil {
		if err := app.movieEventRepo.Create(movieID, models.EventJobCancelled,
			"Download cancelled manually", map[string]interface{}{
				"action":           "manual_cancel",
				"cancelled_status": movie.Status,
			}); err != nil {
			log.Printf("Failed to log job cancellation: %v", err)
		}
	}

	// Update movie status to indicate cancellation
	oldStatus := movie.Status
	movie.Status = models.StatusWanted // Reset to wanted so it can be searched again later
	if err := app.movieRepo.Update(movie); err != nil {
		log.Printf("Failed to update movie status: %v", err)
		http.Error(w, "Failed to cancel job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"message":    "Job cancelled successfully",
		"movie_id":   movieID,
		"old_status": oldStatus,
		"new_status": movie.Status,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// deleteMovieHandler deletes a movie and cancels any associated jobs
func (app *App) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	movieID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid movie ID", http.StatusBadRequest)
		return
	}

	// Check if movie exists
	movie, err := app.movieRepo.GetByID(movieID)
	if err != nil {
		http.Error(w, "Movie not found", http.StatusNotFound)
		return
	}

	// Parse query parameter for torrent deletion
	deleteTorrent := r.URL.Query().Get("delete_torrent") == "true"
	
	// Cancel any active jobs first
	if app.jobManager != nil && (movie.Status == models.StatusDownloading || movie.Status == models.StatusSearching) {
		app.jobManager.CancelJobsForMovie(movieID)

		// Log job cancellation
		if app.movieEventRepo != nil {
			if err := app.movieEventRepo.Create(movieID, models.EventJobCancelled,
				"Job cancelled due to movie deletion", map[string]interface{}{
					"action":           "delete_movie",
					"cancelled_status": movie.Status,
				}); err != nil {
				log.Printf("Failed to log job cancellation during deletion: %v", err)
			}
		}
	}

	// Handle torrent deletion if requested and torrent hash exists
	torrentDeleted := false
	if deleteTorrent && movie.TorrentHash != "" && app.qbittorrentService != nil {
		log.Printf("Deleting torrent %s for movie %s", movie.TorrentHash, movie.Title)
		if err := app.qbittorrentService.RemoveTorrent(movie.TorrentHash, true); err != nil {
			log.Printf("Warning: failed to delete torrent: %v", err)
			// Continue with movie deletion even if torrent deletion fails
		} else {
			torrentDeleted = true
		}
	}

	// Delete the movie from the database
	if err := app.movieRepo.Delete(movieID); err != nil {
		log.Printf("Failed to delete movie: %v", err)
		http.Error(w, "Failed to delete movie", http.StatusInternalServerError)
		return
	}

	// Log deletion event
	if app.movieEventRepo != nil {
		details := map[string]interface{}{"action": "delete"}
		if torrentDeleted {
			details["torrent_deleted"] = true
			details["torrent_hash"] = movie.TorrentHash
		}
		if err := app.movieEventRepo.Create(movieID, models.EventStatusChanged,
			fmt.Sprintf("Movie '%s' deleted from library", movie.Title),
			details); err != nil {
			log.Printf("Failed to log movie deletion: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"message":    "Movie deleted successfully",
		"movie_id":   movieID,
		"title":      movie.Title,
		"has_torrent": movie.TorrentHash != "",
	}
	if torrentDeleted {
		response["torrent_deleted"] = true
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
