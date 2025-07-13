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
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_movies_title ON movies(title);
	CREATE INDEX IF NOT EXISTS idx_movies_status ON movies(status);
	CREATE INDEX IF NOT EXISTS idx_movies_year ON movies(year);
	CREATE INDEX IF NOT EXISTS idx_movies_imdb_id ON movies(imdb_id);
	CREATE INDEX IF NOT EXISTS idx_movies_tmdb_id ON movies(tmdb_id);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	log.Println("Database schema initialized")
	return nil
}
