package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// QBittorrentService handles interactions with qBittorrent WebUI API
type QBittorrentService struct {
	BaseURL  string
	Username string
	Password string
	Client   *http.Client
	Cookie   string
}

// QBTorrent represents a torrent in qBittorrent
type QBTorrent struct {
	Hash     string  `json:"hash"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	State    string  `json:"state"`
	Priority int     `json:"priority"`
	SavePath string  `json:"save_path"`
}

// NewQBittorrentService creates a new qBittorrent service instance
func NewQBittorrentService(baseURL, username, password string) *QBittorrentService {
	return &QBittorrentService{
		BaseURL:  strings.TrimSuffix(baseURL, "/"),
		Username: username,
		Password: password,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Login authenticates with qBittorrent WebUI
func (q *QBittorrentService) Login() error {
	loginURL := fmt.Sprintf("%s/api/v2/auth/login", q.BaseURL)

	data := url.Values{}
	data.Set("username", q.Username)
	data.Set("password", q.Password)

	resp, err := q.Client.PostForm(loginURL, data)
	if err != nil {
		return fmt.Errorf("failed to login to qBittorrent: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qBittorrent login failed with status: %d", resp.StatusCode)
	}

	// Extract session cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "SID" {
			q.Cookie = cookie.String()
			break
		}
	}

	if q.Cookie == "" {
		return fmt.Errorf("failed to get session cookie from qBittorrent")
	}

	log.Println("Successfully logged into qBittorrent")
	return nil
}

// AddTorrentFile adds a torrent file to qBittorrent using the URL
func (q *QBittorrentService) AddTorrentFile(torrentURL, category, savePath string) (string, error) {
	// For now, use the same method as AddTorrent but with torrent file URL
	// qBittorrent can download torrent files from URLs just like magnet URIs
	log.Printf("Adding torrent file to qBittorrent: %s", torrentURL)
	return q.AddTorrent(torrentURL, category, savePath)
}

// AddTorrent adds a new torrent to qBittorrent and returns the torrent hash
func (q *QBittorrentService) AddTorrent(magnetURL, category, savePath string) (string, error) {
	if q.Cookie == "" {
		if err := q.Login(); err != nil {
			return "", fmt.Errorf("failed to login: %w", err)
		}
	}

	addURL := fmt.Sprintf("%s/api/v2/torrents/add", q.BaseURL)

	data := url.Values{}
	data.Set("urls", magnetURL)
	if category != "" {
		data.Set("category", category)
	}
	if savePath != "" {
		data.Set("savepath", savePath)
	}
	// Add tag to identify downloads from our Go application
	data.Set("tags", "go-movies")

	log.Printf("Adding torrent to qBittorrent: %s", magnetURL)

	req, err := http.NewRequest("POST", addURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create add torrent request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", q.Cookie)

	resp, err := q.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to add torrent: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode == http.StatusForbidden {
		// Session expired, try to login again
		if err := q.Login(); err != nil {
			return "", fmt.Errorf("failed to re-login: %w", err)
		}
		return q.AddTorrent(magnetURL, category, savePath)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("add torrent failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Read and log the response body for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Could not read response body: %v", err)
	} else {
		log.Printf("qBittorrent response: %s", string(body))
	}

	log.Printf("Successfully added torrent to qBittorrent")
	
	// Wait a moment for the torrent to be processed
	time.Sleep(2 * time.Second)
	
	// Get the torrent hash by searching for recently added torrents with our tag
	torrents, err := q.GetTorrentsByTag("go-movies")
	if err != nil {
		log.Printf("Warning: could not get torrent hash: %v", err)
		return "", nil
	}
	
	// Find the most recently added torrent (should be the one we just added)
	var latestHash string
	for _, torrent := range torrents {
		// qBittorrent doesn't provide added_on in the info API, so we'll use the first one
		// In practice, this should work since we're adding torrents one at a time
		if latestHash == "" {
			latestHash = torrent.Hash
			break
		}
	}
	
	if latestHash != "" {
		log.Printf("Torrent added with hash: %s", latestHash)
	}
	
	return latestHash, nil
}

// GetTorrents retrieves list of all torrents
func (q *QBittorrentService) GetTorrents() ([]QBTorrent, error) {
	listURL := fmt.Sprintf("%s/api/v2/torrents/info", q.BaseURL)

	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get torrents request: %w", err)
	}

	req.Header.Set("Cookie", q.Cookie)

	resp, err := q.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrents: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get torrents failed with status: %d", resp.StatusCode)
	}

	var torrents []QBTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, fmt.Errorf("failed to decode torrents response: %w", err)
	}

	return torrents, nil
}

// GetTorrentsByTag retrieves list of torrents with a specific tag
func (q *QBittorrentService) GetTorrentsByTag(tag string) ([]QBTorrent, error) {
	listURL := fmt.Sprintf("%s/api/v2/torrents/info?tag=%s", q.BaseURL, url.QueryEscape(tag))

	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get torrents request: %w", err)
	}

	req.Header.Set("Cookie", q.Cookie)

	resp, err := q.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrents: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get torrents failed with status: %d", resp.StatusCode)
	}

	var torrents []QBTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, fmt.Errorf("failed to decode torrents response: %w", err)
	}

	return torrents, nil
}

// RemoveTorrent removes a torrent from qBittorrent
func (q *QBittorrentService) RemoveTorrent(hash string, deleteFiles bool) error {
	if q.Cookie == "" {
		if err := q.Login(); err != nil {
			return fmt.Errorf("failed to login: %w", err)
		}
	}

	deleteURL := fmt.Sprintf("%s/api/v2/torrents/delete", q.BaseURL)

	data := url.Values{}
	data.Set("hashes", hash)
	data.Set("deleteFiles", fmt.Sprintf("%t", deleteFiles))

	req, err := http.NewRequest("POST", deleteURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create delete torrent request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", q.Cookie)

	resp, err := q.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete torrent: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode == http.StatusForbidden {
		// Session expired, try to login again
		if err := q.Login(); err != nil {
			return fmt.Errorf("failed to re-login: %w", err)
		}
		return q.RemoveTorrent(hash, deleteFiles)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete torrent failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully removed torrent %s from qBittorrent (deleteFiles=%t)", hash, deleteFiles)
	return nil
}

// TestConnection tests the connection to qBittorrent
func (q *QBittorrentService) TestConnection() error {
	if err := q.Login(); err != nil {
		return err
	}

	// Try to get application version as a test
	versionURL := fmt.Sprintf("%s/api/v2/app/version", q.BaseURL)

	req, err := http.NewRequest("GET", versionURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Cookie", q.Cookie)

	resp, err := q.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to test connection: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qBittorrent connection test failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	log.Printf("qBittorrent connection successful, version: %s", string(body))
	return nil
}
