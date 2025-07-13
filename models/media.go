// Package models defines the data structures used throughout the application.
package models

import (
	"time"
)

// MediaType represents the type of media content
type MediaType string

// Media type constants
const (
	MediaTypeMovie MediaType = "movie"
	MediaTypeTV    MediaType = "tv"
	MediaTypeBook  MediaType = "book"
	MediaTypeMusic MediaType = "music"
	MediaTypeGame  MediaType = "game"
)

// MediaStatus represents the current status of media
type MediaStatus string

// Media status constants
const (
	StatusWanted      MediaStatus = "wanted"
	StatusDownloading MediaStatus = "downloading"
	StatusDownloaded  MediaStatus = "downloaded"
	StatusProcessing  MediaStatus = "processing"
	StatusReady       MediaStatus = "ready"
	StatusFailed      MediaStatus = "failed"
)

// Media represents a generic media item
type Media struct {
	ID          int         `json:"id" db:"id"`
	Title       string      `json:"title" db:"title"`
	Type        MediaType   `json:"type" db:"type"`
	Status      MediaStatus `json:"status" db:"status"`
	IMDBID      string      `json:"imdb_id,omitempty" db:"imdb_id"`
	TMDBID      int         `json:"tmdb_id,omitempty" db:"tmdb_id"`
	Year        int         `json:"year,omitempty" db:"year"`
	Genre       string      `json:"genre,omitempty" db:"genre"`
	Description string      `json:"description,omitempty" db:"description"`
	Poster      string      `json:"poster,omitempty" db:"poster"`
	Rating      float64     `json:"rating,omitempty" db:"rating"`
	FilePath    string      `json:"file_path,omitempty" db:"file_path"`
	FileSize    int64       `json:"file_size,omitempty" db:"file_size"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

// TVShow represents a TV show with seasons
type TVShow struct {
	Media
	Seasons []Season `json:"seasons,omitempty"`
}

// Season represents a TV show season
type Season struct {
	ID       int       `json:"id" db:"id"`
	MediaID  int       `json:"media_id" db:"media_id"`
	Number   int       `json:"number" db:"number"`
	Episodes []Episode `json:"episodes,omitempty"`
}

// Episode represents a TV show episode
type Episode struct {
	ID          int       `json:"id" db:"id"`
	SeasonID    int       `json:"season_id" db:"season_id"`
	Number      int       `json:"number" db:"number"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description,omitempty" db:"description"`
	AirDate     time.Time `json:"air_date,omitempty" db:"air_date"`
	FilePath    string    `json:"file_path,omitempty" db:"file_path"`
	FileSize    int64     `json:"file_size,omitempty" db:"file_size"`
	Downloaded  bool      `json:"downloaded" db:"downloaded"`
}

// DownloadRequest represents a download task
type DownloadRequest struct {
	ID         int       `json:"id" db:"id"`
	MediaID    int       `json:"media_id" db:"media_id"`
	TorrentURL string    `json:"torrent_url" db:"torrent_url"`
	Quality    string    `json:"quality" db:"quality"`
	Status     string    `json:"status" db:"status"`
	Progress   float64   `json:"progress" db:"progress"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}
