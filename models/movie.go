package models

import "time"

// Movie represents a movie in the media library
type Movie struct {
	ID          int         `json:"id"`
	Title       string      `json:"title"`
	Status      MediaStatus `json:"status"`
	IMDBID      string      `json:"imdb_id,omitempty"`
	TMDBID      int         `json:"tmdb_id,omitempty"`
	Year        int         `json:"year,omitempty"`
	Genre       string      `json:"genre,omitempty"`
	Description string      `json:"description,omitempty"`
	Poster      string      `json:"poster,omitempty"`
	Rating      float64     `json:"rating,omitempty"`
	Runtime     int         `json:"runtime,omitempty"` // in minutes
	Director    string      `json:"director,omitempty"`
	FilePath    string      `json:"file_path,omitempty"`
	FileSize    int64       `json:"file_size,omitempty"`
	Quality     string      `json:"quality,omitempty"` // 1080p, 4K, etc.
	TorrentHash string      `json:"torrent_hash,omitempty"` // qBittorrent info hash
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
