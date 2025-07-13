# Media Library Manager

A Go web service for managing your media library with automated downloading and metadata organization.

## Features

- **Movie Management**: Track movies with metadata (title, year, genre, rating, etc.)
- **Download Integration**: Connect with Jackett for torrent search and qBittorrent for downloads
- **Status Tracking**: Monitor media from "wanted" → "downloading" → "ready"
- **SQLite Database**: Lightweight local storage
- **REST API**: JSON endpoints for all operations

## Quick Start

### Prerequisites

- Go 1.24+ installed
- SQLite3 (included with Go SQLite driver)

### Installation & Running

1. **Clone and setup**:
   ```bash
   git clone <your-repo>
   cd media
   go mod download
   ```

2. **Run the server**:
   ```bash
   go run main.go
   ```

3. **Verify it's working**:
   ```bash
   curl http://localhost:8080/health
   # Should return: OK
   ```

## API Endpoints

### Movies
- `GET /api/v1/movies` - List all movies
- `GET /api/v1/movies/{id}` - Get specific movie
- `POST /api/v1/movies` - Add new movie (not implemented)

### Health
- `GET /health` - Service health check

### Example Usage

```bash
# Get all movies (empty initially)
curl http://localhost:8080/api/v1/movies

# Check health
curl http://localhost:8080/health
```

## Development

### Run Tests
```bash
go test -v
```

### Project Structure
```
├── main.go              # Web server and handlers
├── models/              # Data structures
│   ├── media.go         # Base media types
│   └── movie.go         # Movie-specific model
├── database/            # Database connection and schema
│   └── database.go
├── repository/          # Data access layer
│   └── movie_repository.go
└── services/            # External integrations
    ├── jackett.go       # Torrent search
    └── qbittorrent.go   # Download management
```

## Configuration

The service uses SQLite with a local `media.db` file that's created automatically on first run.

## Planned Features

- [ ] Add movie creation endpoint
- [ ] TV show support with seasons/episodes
- [ ] Music library management
- [ ] Jackett search integration
- [ ] qBittorrent download automation
- [ ] Metadata fetching from TMDB/IMDB
- [ ] File organization and renaming
- [ ] Web UI

## Contributing

1. Write tests for new features
2. Run `go test` before submitting
3. Follow existing code patterns