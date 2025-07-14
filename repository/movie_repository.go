// Package repository provides data access layer for the media application.
package repository

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"media/database"
	"media/models"
)

// MovieRepository handles database operations for movies
type MovieRepository struct {
	db *database.DB
}

// NewMovieRepository creates a new movie repository
func NewMovieRepository(db *database.DB) *MovieRepository {
	return &MovieRepository{db: db}
}

// GetAll retrieves all movies from the database
func (r *MovieRepository) GetAll() ([]models.Movie, error) {
	query := `
		SELECT id, title, status, imdb_id, tmdb_id, year, genre, description, 
			   poster, rating, runtime, director, file_path, file_size, quality,
			   torrent_hash, created_at, updated_at
		FROM movies
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query movies: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Failed to close rows: %v", err)
		}
	}()

	var movies []models.Movie
	for rows.Next() {
		var movie models.Movie
		var imdbID, genre, description, poster, director, filePath, quality, torrentHash sql.NullString
		var tmdbID, year, runtime sql.NullInt64
		var rating sql.NullFloat64
		var fileSize sql.NullInt64

		err := rows.Scan(
			&movie.ID, &movie.Title, &movie.Status,
			&imdbID, &tmdbID, &year, &genre, &description,
			&poster, &rating, &runtime, &director,
			&filePath, &fileSize, &quality, &torrentHash,
			&movie.CreatedAt, &movie.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan movie: %w", err)
		}

		// Handle nullable fields
		if imdbID.Valid {
			movie.IMDBID = imdbID.String
		}
		if tmdbID.Valid {
			movie.TMDBID = int(tmdbID.Int64)
		}
		if year.Valid {
			movie.Year = int(year.Int64)
		}
		if genre.Valid {
			movie.Genre = genre.String
		}
		if description.Valid {
			movie.Description = description.String
		}
		if poster.Valid {
			movie.Poster = poster.String
		}
		if rating.Valid {
			movie.Rating = rating.Float64
		}
		if runtime.Valid {
			movie.Runtime = int(runtime.Int64)
		}
		if director.Valid {
			movie.Director = director.String
		}
		if filePath.Valid {
			movie.FilePath = filePath.String
		}
		if fileSize.Valid {
			movie.FileSize = fileSize.Int64
		}
		if quality.Valid {
			movie.Quality = quality.String
		}
		if torrentHash.Valid {
			movie.TorrentHash = torrentHash.String
		}

		movies = append(movies, movie)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return movies, nil
}

// GetByID retrieves a movie by its ID
func (r *MovieRepository) GetByID(id int) (*models.Movie, error) {
	query := `
		SELECT id, title, status, imdb_id, tmdb_id, year, genre, description, 
			   poster, rating, runtime, director, file_path, file_size, quality,
			   torrent_hash, created_at, updated_at
		FROM movies
		WHERE id = ?
	`

	var movie models.Movie
	var imdbID, genre, description, poster, director, filePath, quality, torrentHash sql.NullString
	var tmdbID, year, runtime sql.NullInt64
	var rating sql.NullFloat64
	var fileSize sql.NullInt64

	err := r.db.QueryRow(query, id).Scan(
		&movie.ID, &movie.Title, &movie.Status,
		&imdbID, &tmdbID, &year, &genre, &description,
		&poster, &rating, &runtime, &director,
		&filePath, &fileSize, &quality, &torrentHash,
		&movie.CreatedAt, &movie.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("movie with id %d not found", id)
		}
		return nil, fmt.Errorf("failed to get movie: %w", err)
	}

	// Handle nullable fields (same as GetAll)
	if imdbID.Valid {
		movie.IMDBID = imdbID.String
	}
	if tmdbID.Valid {
		movie.TMDBID = int(tmdbID.Int64)
	}
	if year.Valid {
		movie.Year = int(year.Int64)
	}
	if genre.Valid {
		movie.Genre = genre.String
	}
	if description.Valid {
		movie.Description = description.String
	}
	if poster.Valid {
		movie.Poster = poster.String
	}
	if rating.Valid {
		movie.Rating = rating.Float64
	}
	if runtime.Valid {
		movie.Runtime = int(runtime.Int64)
	}
	if director.Valid {
		movie.Director = director.String
	}
	if filePath.Valid {
		movie.FilePath = filePath.String
	}
	if fileSize.Valid {
		movie.FileSize = fileSize.Int64
	}
	if quality.Valid {
		movie.Quality = quality.String
	}
	if torrentHash.Valid {
		movie.TorrentHash = torrentHash.String
	}

	return &movie, nil
}

// Create inserts a new movie into the database
func (r *MovieRepository) Create(movie *models.Movie) error {
	query := `
		INSERT INTO movies (title, status, imdb_id, tmdb_id, year, genre, description,
							poster, rating, runtime, director, file_path, file_size, quality, torrent_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	movie.CreatedAt = time.Now()
	movie.UpdatedAt = time.Now()

	result, err := r.db.Exec(query,
		movie.Title, movie.Status, nullString(movie.IMDBID), nullInt(movie.TMDBID),
		nullInt(movie.Year), nullString(movie.Genre), nullString(movie.Description),
		nullString(movie.Poster), nullFloat64(movie.Rating), nullInt(movie.Runtime),
		nullString(movie.Director), nullString(movie.FilePath), nullInt64(movie.FileSize),
		nullString(movie.Quality), nullString(movie.TorrentHash),
	)

	if err != nil {
		return fmt.Errorf("failed to create movie: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	movie.ID = int(id)
	return nil
}

// Update updates an existing movie in the database
func (r *MovieRepository) Update(movie *models.Movie) error {
	query := `
		UPDATE movies 
		SET title = ?, status = ?, imdb_id = ?, tmdb_id = ?, year = ?, genre = ?, description = ?,
			poster = ?, rating = ?, runtime = ?, director = ?, file_path = ?, file_size = ?, quality = ?,
			torrent_hash = ?, updated_at = ?
		WHERE id = ?
	`

	movie.UpdatedAt = time.Now()

	_, err := r.db.Exec(query,
		movie.Title, movie.Status, nullString(movie.IMDBID), nullInt(movie.TMDBID),
		nullInt(movie.Year), nullString(movie.Genre), nullString(movie.Description),
		nullString(movie.Poster), nullFloat64(movie.Rating), nullInt(movie.Runtime),
		nullString(movie.Director), nullString(movie.FilePath), nullInt64(movie.FileSize),
		nullString(movie.Quality), nullString(movie.TorrentHash), movie.UpdatedAt, movie.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update movie: %w", err)
	}

	return nil
}

// Delete removes a movie from the database
func (r *MovieRepository) Delete(id int) error {
	query := `DELETE FROM movies WHERE id = ?`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete movie: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("movie with id %d not found", id)
	}

	return nil
}

// GetByStatus retrieves all movies with a specific status
func (r *MovieRepository) GetByStatus(status models.MediaStatus) ([]models.Movie, error) {
	query := `
		SELECT id, title, status, imdb_id, tmdb_id, year, genre, description, 
			   poster, rating, runtime, director, file_path, file_size, quality,
			   torrent_hash, created_at, updated_at
		FROM movies
		WHERE status = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query movies by status: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Failed to close rows: %v", err)
		}
	}()

	var movies []models.Movie
	for rows.Next() {
		var movie models.Movie
		var imdbID, genre, description, poster, director, filePath, quality, torrentHash sql.NullString
		var tmdbID, year, runtime sql.NullInt64
		var rating sql.NullFloat64
		var fileSize sql.NullInt64

		err := rows.Scan(
			&movie.ID, &movie.Title, &movie.Status,
			&imdbID, &tmdbID, &year, &genre, &description,
			&poster, &rating, &runtime, &director,
			&filePath, &fileSize, &quality, &torrentHash,
			&movie.CreatedAt, &movie.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan movie: %w", err)
		}

		// Handle nullable fields
		if imdbID.Valid {
			movie.IMDBID = imdbID.String
		}
		if tmdbID.Valid {
			movie.TMDBID = int(tmdbID.Int64)
		}
		if year.Valid {
			movie.Year = int(year.Int64)
		}
		if genre.Valid {
			movie.Genre = genre.String
		}
		if description.Valid {
			movie.Description = description.String
		}
		if poster.Valid {
			movie.Poster = poster.String
		}
		if rating.Valid {
			movie.Rating = rating.Float64
		}
		if runtime.Valid {
			movie.Runtime = int(runtime.Int64)
		}
		if director.Valid {
			movie.Director = director.String
		}
		if filePath.Valid {
			movie.FilePath = filePath.String
		}
		if fileSize.Valid {
			movie.FileSize = fileSize.Int64
		}
		if quality.Valid {
			movie.Quality = quality.String
		}
		if torrentHash.Valid {
			movie.TorrentHash = torrentHash.String
		}

		movies = append(movies, movie)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return movies, nil
}

// Helper functions for handling null values
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt(i int) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(i), Valid: true}
}

func nullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}

func nullFloat64(f float64) sql.NullFloat64 {
	if f == 0.0 {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}
