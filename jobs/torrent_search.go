// Package jobs provides background job processing functionality.
package jobs

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"media/models"
	"media/repository"
	"media/services"
)

// TorrentSearchJob handles searching for torrents for a movie
type TorrentSearchJob struct {
	movieRepo          *repository.MovieRepository
	movieEventRepo     *repository.MovieEventRepository
	jackettService     *services.JackettService
	qbittorrentService *services.QBittorrentService
}

// TorrentResult represents a processed torrent search result
type TorrentResult struct {
	Title       string
	Size        int64
	Seeders     int
	Peers       int
	MagnetURI   string
	DownloadURL string
	InfoHash    string
	Quality     string
	Score       int
}

// NewTorrentSearchJob creates a new torrent search job
func NewTorrentSearchJob(movieRepo *repository.MovieRepository, movieEventRepo *repository.MovieEventRepository, jackettService *services.JackettService, qbittorrentService *services.QBittorrentService) *TorrentSearchJob {
	return &TorrentSearchJob{
		movieRepo:          movieRepo,
		movieEventRepo:     movieEventRepo,
		jackettService:     jackettService,
		qbittorrentService: qbittorrentService,
	}
}

// SearchForMovie searches for torrents for a specific movie
func (j *TorrentSearchJob) SearchForMovie(movieID int) error {
	log.Printf("Starting torrent search for movie ID: %d", movieID)

	// Get the movie from the database
	movie, err := j.movieRepo.GetByID(movieID)
	if err != nil {
		return fmt.Errorf("failed to get movie: %w", err)
	}

	// Update status to searching
	movie.Status = models.StatusSearching
	if err := j.movieRepo.Update(movie); err != nil {
		log.Printf("Failed to update movie status to searching: %v", err)
	}

	// Log search start event
	if j.movieEventRepo != nil {
		if err := j.movieEventRepo.Create(movieID, models.EventSearchStarted,
			fmt.Sprintf("Starting torrent search for '%s' (%d)", movie.Title, movie.Year), nil); err != nil {
			log.Printf("Failed to log search start event: %v", err)
		}
	}

	// Build search queries
	queries := j.buildSearchQueries(movie)

	var allResults []TorrentResult

	// Search using each query with movie-specific search
	for _, query := range queries {
		log.Printf("Searching Jackett for movie: '%s'", query)

		// Use movie-specific search with proper parameters
		movieCategories := []string{"2000", "2010", "2020", "2030", "2040", "2050", "2060"} // Various movie categories
		var results []services.JackettSearchResult
		var err error

		// First try with movie-specific search using TMDB ID and IMDB ID if available
		if movie.TMDBID > 0 || movie.IMDBID != "" {
			log.Printf("Trying movie search with IDs: TMDB=%d, IMDB=%s", movie.TMDBID, movie.IMDBID)
			results, err = j.jackettService.SearchMovies("", movie.Year, movie.IMDBID, movie.TMDBID, "2000")
			if err != nil {
				log.Printf("Movie search by ID failed: %v", err)
			} else if len(results) > 0 {
				log.Printf("Found %d results using movie IDs", len(results))
				// Process these results and continue to next query
				processedResults := j.processResults(results, movie)
				allResults = append(allResults, processedResults...)
				continue
			}
		}

		// Try movie search with title and year
		for _, category := range movieCategories {
			log.Printf("Trying movie search with category %s for query '%s'", category, query)
			results, err = j.jackettService.SearchMovies(query, movie.Year, "", 0, category)
			if err != nil {
				log.Printf("Movie search failed for category %s: %v", category, err)
				continue
			}
			if len(results) > 0 {
				log.Printf("Found %d results in category %s", len(results), category)
				break
			}
		}

		// If no results with movie search, try fallback to general search
		if len(results) == 0 {
			log.Printf("No results with movie search, trying general search...")
			results, err = j.jackettService.Search(query, "2000")
			if err != nil {
				log.Printf("General search failed for query '%s': %v", query, err)
				continue
			}
			log.Printf("Found %d results with general search", len(results))
		}

		// Process and score results
		processedResults := j.processResults(results, movie)
		allResults = append(allResults, processedResults...)
	}

	// Deduplicate and sort results
	bestResults := j.selectBestResults(allResults)

	log.Printf("Found %d potential torrents for '%s' (%d)", len(bestResults), movie.Title, movie.Year)

	// Log search completion and update status
	if j.movieEventRepo != nil {
		if len(bestResults) > 0 {
			if err := j.movieEventRepo.Create(movieID, models.EventSearchCompleted,
				fmt.Sprintf("Found %d torrents for '%s'", len(bestResults), movie.Title),
				map[string]interface{}{"torrent_count": len(bestResults)}); err != nil {
				log.Printf("Failed to log search completion: %v", err)
			}
		} else {
			// No torrents found - set status to not_found with detailed reason
			movie.Status = models.StatusNotFound
			if err := j.movieRepo.Update(movie); err != nil {
				log.Printf("Failed to update movie status to not_found: %v", err)
			}

			if err := j.movieEventRepo.Create(movieID, models.EventSearchFailed,
				fmt.Sprintf("No suitable torrents found for '%s' (%d)", movie.Title, movie.Year),
				map[string]interface{}{
					"search_queries":      len(queries),
					"total_results_found": len(allResults),
					"reason":              "no_quality_torrents_after_filtering",
				}); err != nil {
				log.Printf("Failed to log search failure: %v", err)
			}
		}
	}

	if len(bestResults) > 0 {
		best := bestResults[0]
		log.Printf("Best torrent: %s (Seeders: %d, Size: %.2f GB, Score: %d)",
			best.Title, best.Seeders, float64(best.Size)/(1024*1024*1024), best.Score)

		// Log best torrent found
		if j.movieEventRepo != nil {
			torrentDetails := map[string]interface{}{
				"title":   best.Title,
				"seeders": best.Seeders,
				"size_gb": float64(best.Size) / (1024 * 1024 * 1024),
				"score":   best.Score,
				"quality": best.Quality,
			}
			if err := j.movieEventRepo.Create(movieID, models.EventTorrentFound,
				fmt.Sprintf("Best torrent: %s (Score: %d)", best.Title, best.Score), torrentDetails); err != nil {
				log.Printf("Failed to log torrent found: %v", err)
			}
		}

		// Download the torrent if qBittorrent is available
		if j.qbittorrentService != nil {
			// Log download start
			if j.movieEventRepo != nil {
				if err := j.movieEventRepo.Create(movieID, models.EventDownloadStarted,
					fmt.Sprintf("Starting download of '%s'", best.Title), nil); err != nil {
					log.Printf("Failed to log download start: %v", err)
				}
			}

			if err := j.downloadTorrent(best, movie); err != nil {
				log.Printf("Failed to download torrent for '%s': %v", movie.Title, err)
				// Log download failure
				if j.movieEventRepo != nil {
					if err := j.movieEventRepo.Create(movieID, models.EventDownloadFailed,
						fmt.Sprintf("Download failed: %v", err), nil); err != nil {
						log.Printf("Failed to log download failure: %v", err)
					}
				}
			} else {
				// Update movie status to downloading
				movie.Status = models.StatusDownloading
				if err := j.movieRepo.Update(movie); err != nil {
					log.Printf("Failed to update movie status: %v", err)
				}
				log.Printf("Successfully initiated download for '%s'", movie.Title)

				// Log status change and download success
				if j.movieEventRepo != nil {
					if err := j.movieEventRepo.Create(movieID, models.EventStatusChanged,
						fmt.Sprintf("Status changed to: %s", models.StatusDownloading),
						map[string]interface{}{"old_status": movie.Status, "new_status": models.StatusDownloading}); err != nil {
						log.Printf("Failed to log status change: %v", err)
					}
					if err := j.movieEventRepo.Create(movieID, models.EventDownloadStarted,
						fmt.Sprintf("Download initiated for '%s'", best.Title), nil); err != nil {
						log.Printf("Failed to log download success: %v", err)
					}
				}
			}
		} else {
			log.Printf("qBittorrent service not available - skipping download")
		}
	} else {
		log.Printf("No suitable torrents found for '%s' (%d)", movie.Title, movie.Year)
	}

	return nil
}

// buildSearchQueries creates multiple search queries for better results
func (j *TorrentSearchJob) buildSearchQueries(movie *models.Movie) []string {
	var queries []string

	baseTitle := strings.TrimSpace(movie.Title)
	year := movie.Year

	log.Printf("Building search queries for: '%s' (%d)", baseTitle, year)

	// Primary query: "Title Year" (most specific)
	if year > 0 {
		primaryQuery := fmt.Sprintf("%s %d", baseTitle, year)
		queries = append(queries, primaryQuery)
		log.Printf("Added primary query: '%s'", primaryQuery)
	}

	// Clean up title for better matching
	cleanTitle := baseTitle
	// Remove colons and replace with spaces
	cleanTitle = strings.ReplaceAll(cleanTitle, ":", " ")
	// Replace hyphens with spaces
	cleanTitle = strings.ReplaceAll(cleanTitle, "-", " ")
	// Remove extra spaces
	cleanTitle = strings.TrimSpace(strings.ReplaceAll(cleanTitle, "  ", " "))

	// Add cleaned title with year if different from original
	if cleanTitle != baseTitle && year > 0 {
		cleanQuery := fmt.Sprintf("%s %d", cleanTitle, year)
		queries = append(queries, cleanQuery)
		log.Printf("Added clean query: '%s'", cleanQuery)
	}

	// For titles with "The", try without it
	if strings.HasPrefix(strings.ToUpper(baseTitle), "THE ") {
		withoutThe := strings.TrimSpace(baseTitle[4:])
		if year > 0 {
			withoutTheQuery := fmt.Sprintf("%s %d", withoutThe, year)
			queries = append(queries, withoutTheQuery)
			log.Printf("Added 'without The' query: '%s'", withoutTheQuery)
		}
	}

	// Last resort: just the title without year
	if len(queries) < 3 {
		queries = append(queries, baseTitle)
		log.Printf("Added title-only query: '%s'", baseTitle)
	}

	log.Printf("Final queries: %v", queries)
	return queries
}

// processResults processes raw Jackett results and scores them
func (j *TorrentSearchJob) processResults(results []services.JackettSearchResult, movie *models.Movie) []TorrentResult {
	var processed []TorrentResult
	var filteredCount = make(map[string]int)

	log.Printf("Processing %d raw results for '%s'", len(results), movie.Title)

	for _, result := range results {
		// Skip results with no seeders (critical for download success)
		if result.Seeders == 0 {
			filteredCount["no_seeders"]++
			continue
		}

		// Enhanced filtering for quality
		title := strings.ToUpper(result.Title)

		// Skip obvious low quality releases immediately
		if j.isLowQualityRelease(title) {
			filteredCount["low_quality"]++
			continue
		}

		// Strongly prefer torrents with magnet links
		hasMagnet := result.MagnetURI != "" && result.MagnetURI != "null" && strings.HasPrefix(result.MagnetURI, "magnet:")
		hasDownloadURL := result.Link != "" && result.Link != "null"

		// Skip torrents without any download method
		if !hasMagnet && !hasDownloadURL {
			filteredCount["no_download_method"]++
			continue
		}

		// Skip results that are too small (likely not full movies)
		if result.Size < 100*1024*1024 { // Reduced from 200MB to 100MB
			filteredCount["too_small"]++
			continue
		}

		// Skip results that are too large (likely uncompressed or fake)
		if result.Size > 100*1024*1024*1024 { // More than 100GB
			filteredCount["too_large"]++
			continue
		}

		// Basic title relevance check
		movieTitle := strings.ToUpper(movie.Title)
		if !j.isRelevantTitle(title, movieTitle, movie.Year) {
			filteredCount["not_relevant"]++
			log.Printf("Filtered out as not relevant: '%s' for movie '%s'", result.Title, movie.Title)
			continue
		}

		torrentResult := TorrentResult{
			Title:       result.Title,
			Size:        result.Size,
			Seeders:     result.Seeders,
			Peers:       result.Peers,
			MagnetURI:   result.MagnetURI,
			DownloadURL: result.Link,
			InfoHash:    result.InfoHash,
			Quality:     j.extractQuality(result.Title),
			Score:       j.scoreResult(result, movie),
		}

		processed = append(processed, torrentResult)
	}

	log.Printf("Filtering results: %d total -> %d kept. Filtered: %+v", len(results), len(processed), filteredCount)
	return processed
}

// isLowQualityRelease checks if a release is obviously low quality
func (j *TorrentSearchJob) isLowQualityRelease(title string) bool {
	lowQualityMarkers := []string{
		"CAM", "TS", "TELESYNC", "HDCAM", "HDTS", "TC", "TELECINE",
		"WORKPRINT", "WP", "SCREENER", "SCR", "DVDSCR", "BDSCR",
		"KORSUB", "HC", "HARDCODED", "HARDSUB", "R5", "R6",
	}

	for _, marker := range lowQualityMarkers {
		if strings.Contains(title, marker) {
			return true
		}
	}

	return false
}

// isRelevantTitle checks if the torrent title is relevant to the movie
func (j *TorrentSearchJob) isRelevantTitle(torrentTitle, movieTitle string, movieYear int) bool {
	// Normalize both titles for comparison
	torrentTitle = strings.ToUpper(torrentTitle)
	movieTitle = strings.ToUpper(movieTitle)

	// Remove common words that might not appear in torrent titles
	commonWords := []string{"THE", "A", "AN", "OF", "AND", "OR", "BUT", "IN", "ON", "AT", "TO", "FOR", "AS", "BY"}

	movieWords := strings.Fields(movieTitle)
	var significantWords []string

	for _, word := range movieWords {
		isCommon := false
		for _, common := range commonWords {
			if word == common {
				isCommon = true
				break
			}
		}
		if !isCommon && len(word) > 2 {
			significantWords = append(significantWords, word)
		}
	}

	// Count how many significant words match
	matchedWords := 0
	for _, word := range significantWords {
		if strings.Contains(torrentTitle, word) {
			matchedWords++
		}
	}

	// More lenient matching - require at least 1 significant word for short titles,
	// or at least 30% of significant words for longer titles
	if len(significantWords) == 0 {
		// If no significant words, fall back to checking if any movie word appears
		for _, word := range movieWords {
			if len(word) > 2 && strings.Contains(torrentTitle, word) {
				return true
			}
		}
		return false
	}

	if len(significantWords) <= 2 {
		// For short titles, require at least 1 match
		return matchedWords >= 1
	} else {
		// For longer titles, require at least 30% match (was 50%)
		required := max(1, len(significantWords)*3/10)
		return matchedWords >= required
	}
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// extractQuality attempts to extract quality information from torrent title
func (j *TorrentSearchJob) extractQuality(title string) string {
	title = strings.ToUpper(title)

	if strings.Contains(title, "2160P") || strings.Contains(title, "4K") {
		return "4K"
	}
	if strings.Contains(title, "1080P") {
		return "1080p"
	}
	if strings.Contains(title, "720P") {
		return "720p"
	}
	if strings.Contains(title, "480P") {
		return "480p"
	}

	return "Unknown"
}

// scoreResult scores a torrent result based on various factors
func (j *TorrentSearchJob) scoreResult(result services.JackettSearchResult, movie *models.Movie) int {
	score := 0
	title := strings.ToUpper(result.Title)
	originalTitle := result.Title
	movieTitle := strings.ToUpper(movie.Title)

	// Enhanced title matching with word boundaries
	movieWords := strings.Fields(movieTitle)
	titleMatchScore := 0
	significantMatches := 0

	// Remove common words for better matching
	commonWords := []string{"THE", "A", "AN", "OF", "AND", "OR", "BUT", "IN", "ON", "AT", "TO", "FOR", "AS", "BY"}

	for _, word := range movieWords {
		if len(word) > 2 {
			isCommon := false
			for _, common := range commonWords {
				if word == common {
					isCommon = true
					break
				}
			}

			if strings.Contains(title, word) {
				if !isCommon {
					titleMatchScore += 15 // Higher score for significant words
					significantMatches++
				} else {
					titleMatchScore += 5 // Lower score for common words
				}
			}
		}
	}

	// Bonus for having multiple significant word matches
	if significantMatches >= 2 {
		titleMatchScore += 20
	}

	// Exact title match bonus
	if strings.Contains(title, movieTitle) {
		titleMatchScore += 40
	}

	// Fuzzy match for title variations
	cleanMovieTitle := strings.ReplaceAll(movieTitle, ":", "")
	cleanMovieTitle = strings.ReplaceAll(cleanMovieTitle, "-", " ")
	cleanMovieTitle = strings.ReplaceAll(cleanMovieTitle, "  ", " ")
	if strings.Contains(title, cleanMovieTitle) {
		titleMatchScore += 25
	}

	score += titleMatchScore

	// Year matching with exact match bonus
	if movie.Year > 0 {
		yearStr := fmt.Sprintf("%d", movie.Year)
		if strings.Contains(title, yearStr) {
			score += 40
		}
	}

	// Enhanced seeders and peers scoring (major factor for download success)
	seedersScore := j.scoreSeedersPeers(result.Seeders, result.Peers)
	score += seedersScore

	// Release type hierarchy (most important quality factor)
	releaseScore := j.scoreReleaseType(title)
	score += releaseScore

	// Quality/resolution scoring
	qualityScore := j.scoreQuality(title)
	score += qualityScore

	// Encoding preference (x265/HEVC more efficient)
	if strings.Contains(title, "X265") || strings.Contains(title, "HEVC") || strings.Contains(title, "H265") {
		score += 15
	} else if strings.Contains(title, "X264") || strings.Contains(title, "H264") {
		score += 10
	}

	// Audio quality scoring
	audioScore := j.scoreAudio(title)
	score += audioScore

	// Trusted release groups
	groupScore := j.scoreTrustedGroups(originalTitle)
	score += groupScore

	// Magnet link availability (critical for download success)
	magnetScore := j.scoreMagnetAvailability(result.MagnetURI, result.Link)
	score += magnetScore

	// File size appropriateness
	sizeScore := j.scoreFitSize(result.Size, title)
	score += sizeScore

	// Penalize low quality releases heavily
	penaltyScore := j.applyQualityPenalties(title)
	score += penaltyScore

	// Language penalties
	if strings.Contains(title, "DUBBED") || strings.Contains(title, "FRENCH") ||
		strings.Contains(title, "GERMAN") || strings.Contains(title, "SPANISH") ||
		strings.Contains(title, "ITALIAN") {
		score -= 20
	}

	// Ensure minimum score for valid torrents
	if score < 0 {
		score = 0
	}

	return score
}

// scoreReleaseType scores based on release type hierarchy
func (j *TorrentSearchJob) scoreReleaseType(title string) int {
	if strings.Contains(title, "REMUX") {
		return 100 // Highest quality, uncompressed
	}
	if strings.Contains(title, "BLURAY") || strings.Contains(title, "BDR") || strings.Contains(title, "BD25") || strings.Contains(title, "BD50") {
		return 80
	}
	if strings.Contains(title, "WEB-DL") || strings.Contains(title, "WEBDL") {
		return 70
	}
	if strings.Contains(title, "WEBRIP") || strings.Contains(title, "WEB RIP") {
		return 60
	}
	if strings.Contains(title, "BRRIP") || strings.Contains(title, "BLURAY RIP") {
		return 55
	}
	if strings.Contains(title, "DVDRIP") || strings.Contains(title, "DVD RIP") {
		return 45
	}
	if strings.Contains(title, "HDTV") || strings.Contains(title, "PDTV") {
		return 40
	}
	if strings.Contains(title, "DVDSCR") || strings.Contains(title, "SCREENER") {
		return 30
	}
	return 0
}

// scoreQuality scores based on resolution and quality markers
func (j *TorrentSearchJob) scoreQuality(title string) int {
	if strings.Contains(title, "2160P") || strings.Contains(title, "4K") || strings.Contains(title, "UHD") {
		return 35 // High quality but large files
	}
	if strings.Contains(title, "1080P") {
		return 40 // Sweet spot for quality/size
	}
	if strings.Contains(title, "720P") {
		return 30
	}
	if strings.Contains(title, "480P") || strings.Contains(title, "SD") {
		return 10
	}
	return 0
}

// scoreAudio scores based on audio quality
func (j *TorrentSearchJob) scoreAudio(title string) int {
	if strings.Contains(title, "ATMOS") || strings.Contains(title, "TRUEHD") {
		return 20
	}
	if strings.Contains(title, "DTS-HD") || strings.Contains(title, "DTSHD") {
		return 15
	}
	if strings.Contains(title, "DTS") || strings.Contains(title, "DD5.1") || strings.Contains(title, "AC3") {
		return 10
	}
	if strings.Contains(title, "AAC") {
		return 5
	}
	return 0
}

// scoreTrustedGroups gives bonus points for known quality release groups
func (j *TorrentSearchJob) scoreTrustedGroups(title string) int {
	trustedGroups := []string{
		"SPARKS", "FGT", "CMRG", "EVO", "RARBG", "YTS", "YIFY",
		"PSA", "ION10", "AMZN", "NTG", "FLUX", "TOMMY", "DEFLATE",
		"QOQ", "TEHPARADOX", "GECKOS", "ROVERS", "DRONES", "STUTTERSHIT",
		"KINGDOM", "MZABI", "TayTO", "W4F", "LAZY", "ZQ", "KRaLiMaRKo",
		"PbK", "TAoE", "UTR", "MTeam", "CHD", "WiKi", "TDD", "Tigole",
		"Joy", "Pahe", "QxR", "HQMUX", "d3g", "playBD", "HDH", "ifi",
	}

	titleUpper := strings.ToUpper(title)
	for _, group := range trustedGroups {
		if strings.Contains(titleUpper, strings.ToUpper(group)) {
			switch group {
			case "SPARKS", "FGT", "RARBG", "PSA", "NTG":
				return 25 // Top tier groups
			case "EVO", "CMRG", "ION10", "QOQ":
				return 20 // High quality groups
			default:
				return 15 // Good groups
			}
		}
	}
	return 0
}

// scoreFitSize scores based on appropriate file size for quality
func (j *TorrentSearchJob) scoreFitSize(size int64, title string) int {
	sizeGB := float64(size) / (1024 * 1024 * 1024)

	// Expected size ranges based on quality
	if strings.Contains(title, "2160P") || strings.Contains(title, "4K") {
		if sizeGB >= 15 && sizeGB <= 80 {
			return 15
		} else if sizeGB > 5 && sizeGB < 15 {
			return 10
		}
	} else if strings.Contains(title, "1080P") {
		if sizeGB >= 3 && sizeGB <= 25 {
			return 15
		} else if sizeGB >= 1.5 && sizeGB < 3 {
			return 10
		}
	} else if strings.Contains(title, "720P") {
		if sizeGB >= 1 && sizeGB <= 8 {
			return 15
		} else if sizeGB >= 0.7 && sizeGB < 1 {
			return 10
		}
	}

	// Penalty for extremely small or large files
	if sizeGB < 0.5 || sizeGB > 100 {
		return -20
	}

	return 5 // Default reasonable size
}

// applyQualityPenalties penalizes poor quality releases
func (j *TorrentSearchJob) applyQualityPenalties(title string) int {
	penalty := 0

	// Heavy penalties for cam/telesync releases
	if strings.Contains(title, "CAM") || strings.Contains(title, "TS") ||
		strings.Contains(title, "HDCAM") || strings.Contains(title, "TELESYNC") ||
		strings.Contains(title, "HDTS") || strings.Contains(title, "TC") {
		penalty -= 100
	}

	// Penalties for workprint/unfinished releases
	if strings.Contains(title, "WORKPRINT") || strings.Contains(title, "WP") ||
		strings.Contains(title, "UNFINISHED") || strings.Contains(title, "LEAK") {
		penalty -= 50
	}

	// Penalties for scene/p2p indicators of lower quality
	if strings.Contains(title, "KORSUB") || strings.Contains(title, "HC") ||
		strings.Contains(title, "HARDCODED") {
		penalty -= 30
	}

	return penalty
}

// scoreSeedersPeers provides enhanced scoring for seeders and peers
func (j *TorrentSearchJob) scoreSeedersPeers(seeders, peers int) int {
	score := 0

	// Seeder scoring with exponential benefits for high seeder counts
	if seeders >= 1000 {
		score += 100 // Excellent availability
	} else if seeders >= 500 {
		score += 80
	} else if seeders >= 200 {
		score += 60
	} else if seeders >= 100 {
		score += 50
	} else if seeders >= 50 {
		score += 40
	} else if seeders >= 25 {
		score += 30
	} else if seeders >= 15 {
		score += 25
	} else if seeders >= 10 {
		score += 20
	} else if seeders >= 5 {
		score += 15
	} else if seeders >= 3 {
		score += 10
	} else if seeders >= 1 {
		score += 5
	} else {
		score -= 50 // Heavy penalty for zero seeders
	}

	// Peer/leecher scoring (indicates active download activity)
	if peers >= 100 {
		score += 20 // Very active torrent
	} else if peers >= 50 {
		score += 15
	} else if peers >= 20 {
		score += 12
	} else if peers >= 10 {
		score += 10
	} else if peers >= 5 {
		score += 8
	} else if peers >= 2 {
		score += 5
	}

	// Bonus for good seeder/peer ratio (healthy swarm)
	if seeders > 0 && peers > 0 {
		ratio := float64(seeders) / float64(peers)
		if ratio >= 2.0 {
			score += 15 // Excellent ratio
		} else if ratio >= 1.0 {
			score += 10 // Good ratio
		} else if ratio >= 0.5 {
			score += 5 // Decent ratio
		}
	}

	return score
}

// scoreMagnetAvailability scores based on magnet link availability
func (j *TorrentSearchJob) scoreMagnetAvailability(magnetURI, downloadURL string) int {
	score := 0

	// Magnet URI is preferred (direct download, no intermediate steps)
	if magnetURI != "" && magnetURI != "null" && strings.HasPrefix(magnetURI, "magnet:") {
		score += 50 // Major bonus for valid magnet link

		// Additional bonus for magnet links with trackers
		if strings.Contains(magnetURI, "tr=") {
			score += 10
		}

		// Bonus for magnet links with display name
		if strings.Contains(magnetURI, "dn=") {
			score += 5
		}
	} else {
		score -= 30 // Significant penalty for missing magnet link
	}

	// Backup download URL availability
	if downloadURL != "" && downloadURL != "null" {
		score += 10 // Small bonus for having alternative download method
	}

	return score
}

// selectBestResults removes duplicates and returns the best scored results
func (j *TorrentSearchJob) selectBestResults(results []TorrentResult) []TorrentResult {
	if len(results) == 0 {
		return results
	}

	// Remove duplicates based on InfoHash
	seen := make(map[string]bool)
	var unique []TorrentResult

	for _, result := range results {
		if result.InfoHash != "" && !seen[result.InfoHash] {
			seen[result.InfoHash] = true
			unique = append(unique, result)
		} else if result.InfoHash == "" {
			// If no InfoHash, use title as deduplication key
			titleKey := strings.ToLower(strings.TrimSpace(result.Title))
			if !seen[titleKey] {
				seen[titleKey] = true
				unique = append(unique, result)
			}
		}
	}

	// Sort by score (highest first), with magnet link availability as tiebreaker
	sort.Slice(unique, func(i, j int) bool {
		// Primary sort: score
		if unique[i].Score != unique[j].Score {
			return unique[i].Score > unique[j].Score
		}

		// Tiebreaker 1: prefer torrents with magnet links
		iHasMagnet := unique[i].MagnetURI != "" && unique[i].MagnetURI != "null" && strings.HasPrefix(unique[i].MagnetURI, "magnet:")
		jHasMagnet := unique[j].MagnetURI != "" && unique[j].MagnetURI != "null" && strings.HasPrefix(unique[j].MagnetURI, "magnet:")

		if iHasMagnet != jHasMagnet {
			return iHasMagnet
		}

		// Tiebreaker 2: prefer higher seeder count
		return unique[i].Seeders > unique[j].Seeders
	})

	// Return top 10 results
	if len(unique) > 10 {
		return unique[:10]
	}

	return unique
}

// ProcessMovieQueue processes movies that need torrent searches
func (j *TorrentSearchJob) ProcessMovieQueue() error {
	// Get movies that need searching
	movies, err := j.movieRepo.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get movies: %w", err)
	}

	for _, movie := range movies {
		// Search for movies that are wanted or not found (for retry)
		if movie.Status == models.StatusWanted || movie.Status == models.StatusNotFound {
			// Add a small delay between searches to be respectful to Jackett
			time.Sleep(2 * time.Second)

			if err := j.SearchForMovie(movie.ID); err != nil {
				log.Printf("Failed to search for movie %d (%s): %v", movie.ID, movie.Title, err)

				// Log the search error
				if j.movieEventRepo != nil {
					if err := j.movieEventRepo.Create(movie.ID, models.EventSearchFailed,
						fmt.Sprintf("Search error: %v", err),
						map[string]interface{}{"error": err.Error()}); err != nil {
						log.Printf("Failed to log search error: %v", err)
					}
				}
				continue
			}
		}
	}

	return nil
}

// downloadTorrent downloads a torrent using the best available method
func (j *TorrentSearchJob) downloadTorrent(result TorrentResult, movie *models.Movie) error {
	category := "movies"
	downloadPath := "" // Use qBittorrent default path
	
	var torrentHash string
	var err error

	// Try magnet URI first (preferred method)
	if result.MagnetURI != "" && result.MagnetURI != "null" {
		log.Printf("Downloading torrent via magnet URI for '%s'", movie.Title)
		torrentHash, err = j.qbittorrentService.AddTorrent(result.MagnetURI, category, downloadPath)
	} else if result.DownloadURL != "" {
		// Fall back to download URL and use torrent file method
		log.Printf("Downloading torrent via torrent file for '%s'", movie.Title)
		torrentHash, err = j.qbittorrentService.AddTorrentFile(result.DownloadURL, category, downloadPath)
	} else {
		return fmt.Errorf("no magnet URI or download URL available")
	}
	
	if err != nil {
		return err
	}
	
	// Store the torrent hash in the movie record
	if torrentHash != "" {
		movie.TorrentHash = torrentHash
		if err := j.movieRepo.Update(movie); err != nil {
			log.Printf("Warning: failed to update movie with torrent hash: %v", err)
		}
	}
	
	return nil
}

// GetMovieByID retrieves a movie by ID (for job manager access)
func (j *TorrentSearchJob) GetMovieByID(movieID int) (*models.Movie, error) {
	return j.movieRepo.GetByID(movieID)
}

// UpdateMovie updates a movie (for job manager access)
func (j *TorrentSearchJob) UpdateMovie(movie *models.Movie) error {
	return j.movieRepo.Update(movie)
}
