package models

import "time"

// MovieEventType represents the type of movie event
type MovieEventType string

const (
	EventSearchStarted     MovieEventType = "search_started"
	EventSearchCompleted   MovieEventType = "search_completed"
	EventSearchFailed      MovieEventType = "search_failed"
	EventTorrentFound      MovieEventType = "torrent_found"
	EventDownloadStarted   MovieEventType = "download_started"
	EventDownloadCompleted MovieEventType = "download_completed"
	EventDownloadFailed    MovieEventType = "download_failed"
	EventJobCancelled      MovieEventType = "job_cancelled"
	EventStatusChanged     MovieEventType = "status_changed"
)

// MovieEvent represents an event in the movie download process
type MovieEvent struct {
	ID        int            `json:"id"`
	MovieID   int            `json:"movie_id"`
	Type      MovieEventType `json:"type"`
	Message   string         `json:"message"`
	Details   string         `json:"details,omitempty"` // JSON string for additional data
	CreatedAt time.Time      `json:"created_at"`
}

// DetailedMovieResponse represents a detailed view of a movie with events and job control
type DetailedMovieResponse struct {
	Movie      *Movie       `json:"movie"`
	Events     []MovieEvent `json:"events"`
	JobControl *JobControl  `json:"job_control"`
	Statistics *MovieStats  `json:"statistics"`
}

// JobControl provides job management actions
type JobControl struct {
	CanCancel   bool   `json:"can_cancel"`
	CanRestart  bool   `json:"can_restart"`
	CancelURL   string `json:"cancel_url,omitempty"`
	RestartURL  string `json:"restart_url,omitempty"`
	CurrentJob  string `json:"current_job,omitempty"`
	LastJobTime string `json:"last_job_time,omitempty"`
}

// MovieStats provides statistics about the movie's download process
type MovieStats struct {
	TotalSearches    int     `json:"total_searches"`
	TotalTorrents    int     `json:"total_torrents_found"`
	LastSearchTime   string  `json:"last_search_time,omitempty"`
	BestTorrentScore int     `json:"best_torrent_score,omitempty"`
	AvgSearchTime    float64 `json:"avg_search_time_seconds,omitempty"`
}
