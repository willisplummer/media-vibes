package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"media/database"
	"media/models"
	"time"
)

// MovieEventRepository handles movie event data operations
type MovieEventRepository struct {
	db *database.DB
}

// NewMovieEventRepository creates a new movie event repository
func NewMovieEventRepository(db *database.DB) *MovieEventRepository {
	return &MovieEventRepository{db: db}
}

// Create adds a new movie event
func (r *MovieEventRepository) Create(movieID int, eventType models.MovieEventType, message string, details interface{}) error {
	var detailsJSON string
	if details != nil {
		detailsBytes, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("failed to marshal event details: %w", err)
		}
		detailsJSON = string(detailsBytes)
	}

	query := `INSERT INTO movie_events (movie_id, type, message, details) VALUES (?, ?, ?, ?)`
	_, err := r.db.Exec(query, movieID, string(eventType), message, detailsJSON)
	if err != nil {
		return fmt.Errorf("failed to create movie event: %w", err)
	}

	return nil
}

// GetByMovieID returns all events for a specific movie
func (r *MovieEventRepository) GetByMovieID(movieID int) ([]models.MovieEvent, error) {
	query := `SELECT id, movie_id, type, message, details, created_at 
			  FROM movie_events 
			  WHERE movie_id = ? 
			  ORDER BY created_at DESC`

	rows, err := r.db.Query(query, movieID)
	if err != nil {
		return nil, fmt.Errorf("failed to query movie events: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			fmt.Printf("Failed to close rows: %v\n", cerr)
		}
	}()

	var events []models.MovieEvent
	for rows.Next() {
		var event models.MovieEvent
		var details sql.NullString
		var createdAt string

		err := rows.Scan(&event.ID, &event.MovieID, &event.Type, &event.Message, &details, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan movie event: %w", err)
		}

		if details.Valid {
			event.Details = details.String
		}

		if parsedTime, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			event.CreatedAt = parsedTime
		}

		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating movie events: %w", err)
	}

	return events, nil
}

// GetStatistics returns statistics about movie events
func (r *MovieEventRepository) GetStatistics(movieID int) (*models.MovieStats, error) {
	stats := &models.MovieStats{}

	// Count total searches
	err := r.db.QueryRow(`SELECT COUNT(*) FROM movie_events WHERE movie_id = ? AND type = ?`,
		movieID, models.EventSearchStarted).Scan(&stats.TotalSearches)
	if err != nil {
		return nil, fmt.Errorf("failed to count searches: %w", err)
	}

	// Count total torrents found
	err = r.db.QueryRow(`SELECT COUNT(*) FROM movie_events WHERE movie_id = ? AND type = ?`,
		movieID, models.EventTorrentFound).Scan(&stats.TotalTorrents)
	if err != nil {
		return nil, fmt.Errorf("failed to count torrents: %w", err)
	}

	// Get last search time
	var lastSearchTime sql.NullString
	err = r.db.QueryRow(`SELECT created_at FROM movie_events WHERE movie_id = ? AND type = ? ORDER BY created_at DESC LIMIT 1`,
		movieID, models.EventSearchStarted).Scan(&lastSearchTime)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get last search time: %w", err)
	}
	if lastSearchTime.Valid {
		stats.LastSearchTime = lastSearchTime.String
	}

	// Get best torrent score from details
	rows, err := r.db.Query(`SELECT details FROM movie_events WHERE movie_id = ? AND type = ? AND details IS NOT NULL`,
		movieID, models.EventTorrentFound)
	if err != nil {
		return nil, fmt.Errorf("failed to query torrent details: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			fmt.Printf("Failed to close rows: %v\n", cerr)
		}
	}()

	bestScore := 0
	for rows.Next() {
		var detailsJSON string
		if err := rows.Scan(&detailsJSON); err != nil {
			continue
		}

		var details map[string]interface{}
		if err := json.Unmarshal([]byte(detailsJSON), &details); err != nil {
			continue
		}

		if score, ok := details["score"].(float64); ok && int(score) > bestScore {
			bestScore = int(score)
		}
	}
	stats.BestTorrentScore = bestScore

	return stats, nil
}

// DeleteOldEvents removes events older than the specified duration
func (r *MovieEventRepository) DeleteOldEvents(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	query := `DELETE FROM movie_events WHERE created_at < ?`
	_, err := r.db.Exec(query, cutoff.Format("2006-01-02 15:04:05"))
	if err != nil {
		return fmt.Errorf("failed to delete old events: %w", err)
	}
	return nil
}
