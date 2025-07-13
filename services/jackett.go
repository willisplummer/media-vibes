// Package services provides external service integrations.
package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

// JackettService handles interactions with Jackett torrent indexer
type JackettService struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// JackettSearchResult represents a single search result from Jackett
type JackettSearchResult struct {
	Title        string `json:"Title"`
	CategoryDesc string `json:"CategoryDesc"`
	Size         int64  `json:"Size"`
	Link         string `json:"Link"`
	Seeders      int    `json:"Seeders"`
	Peers        int    `json:"Peers"`
	InfoHash     string `json:"InfoHash"`
	MagnetURI    string `json:"MagnetUri"`
}

// JackettResponse represents the response from Jackett API
type JackettResponse struct {
	Results []JackettSearchResult `json:"Results"`
}

// NewJackettService creates a new Jackett service instance
func NewJackettService(baseURL, apiKey string) *JackettService {
	return &JackettService{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Client:  &http.Client{},
	}
}

// Search performs a search query on Jackett
func (j *JackettService) Search(query string, category string) ([]JackettSearchResult, error) {
	params := url.Values{}
	params.Set("apikey", j.APIKey)
	params.Set("t", "search")
	params.Set("q", query)
	if category != "" {
		params.Set("cat", category)
	}

	searchURL := fmt.Sprintf("%s/api/v2.0/indexers/all/results?%s", j.BaseURL, params.Encode())

	resp, err := j.Client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search jackett: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jackett search failed with status: %d", resp.StatusCode)
	}

	var jackettResp JackettResponse
	if err := json.NewDecoder(resp.Body).Decode(&jackettResp); err != nil {
		return nil, fmt.Errorf("failed to decode jackett response: %w", err)
	}

	return jackettResp.Results, nil
}
