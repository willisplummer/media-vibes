package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
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
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Client:   &http.Client{},
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

	cookies := resp.Header.Get("Set-Cookie")
	if cookies != "" {
		q.Cookie = strings.Split(cookies, ";")[0]
	}

	return nil
}

// AddTorrent adds a new torrent to qBittorrent
func (q *QBittorrentService) AddTorrent(magnetURL, savePath string) error {
	addURL := fmt.Sprintf("%s/api/v2/torrents/add", q.BaseURL)

	data := url.Values{}
	data.Set("urls", magnetURL)
	if savePath != "" {
		data.Set("savepath", savePath)
	}

	req, err := http.NewRequest("POST", addURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create add torrent request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", q.Cookie)

	resp, err := q.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add torrent: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("add torrent failed with status: %d", resp.StatusCode)
	}

	return nil
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
