// Package database provides database connectivity and schema management.
package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3" // Import sqlite3 driver
)

// DB wraps the SQL database connection
type DB struct {
	*sql.DB
}

// NewDB creates a new database connection
func NewDB(dataSourceName string) (*DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// InitSchema initializes the database schema
func (db *DB) InitSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS movies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		status TEXT NOT NULL,
		imdb_id TEXT,
		tmdb_id INTEGER,
		year INTEGER,
		genre TEXT,
		description TEXT,
		poster TEXT,
		rating REAL,
		runtime INTEGER,
		director TEXT,
		file_path TEXT,
		file_size INTEGER,
		quality TEXT,
		torrent_hash TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_movies_title ON movies(title);
	CREATE INDEX IF NOT EXISTS idx_movies_status ON movies(status);
	CREATE INDEX IF NOT EXISTS idx_movies_year ON movies(year);
	CREATE INDEX IF NOT EXISTS idx_movies_imdb_id ON movies(imdb_id);
	CREATE INDEX IF NOT EXISTS idx_movies_tmdb_id ON movies(tmdb_id);
	CREATE INDEX IF NOT EXISTS idx_movies_torrent_hash ON movies(torrent_hash);

	CREATE TABLE IF NOT EXISTS movie_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		movie_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		message TEXT NOT NULL,
		details TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_movie_events_movie_id ON movie_events(movie_id);
	CREATE INDEX IF NOT EXISTS idx_movie_events_type ON movie_events(type);
	CREATE INDEX IF NOT EXISTS idx_movie_events_created_at ON movie_events(created_at);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Add migration for existing databases
	migration := `
	ALTER TABLE movies ADD COLUMN torrent_hash TEXT;
	`
	
	// Try to add the column, ignore error if it already exists
	db.Exec(migration)

	log.Println("Database schema initialized")
	return nil
}
