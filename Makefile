.PHONY: build test lint fmt clean run kill dev

# Build the application
build:
	go build -o media .

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Install golangci-lint if not present
install-lint:
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linters
lint: install-lint
	@if which golangci-lint > /dev/null; then \
		golangci-lint run || true; \
	else \
		~/go/bin/golangci-lint run || true; \
	fi

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .

# Clean build artifacts
clean:
	rm -f media
	rm -f coverage.out coverage.html

# Run the application (requires TMDB_API_KEY env var)
run: build
	./media

# Quick check - format, lint, and test
check: fmt lint test

# Install dependencies
deps:
	go mod download
	go mod tidy

# Update dependencies
update-deps:
	go get -u ./...
	go mod tidy

# Kill the running server
kill:
	@if [ -f app.pid ]; then \
		echo "Killing server with PID $$(cat app.pid)..."; \
		kill $$(cat app.pid) 2>/dev/null || true; \
		rm -f app.pid; \
	else \
		echo "No app.pid file found, trying to kill by port..."; \
		lsof -ti:8080 | xargs kill -9 2>/dev/null || true; \
	fi
	@echo "Server stopped"

# Start server in development mode (with auto-restart)
dev: kill
	@echo "Starting server in development mode..."
	nohup go run main.go > app.log 2>&1 & echo $$! > app.pid
	@echo "Server started with PID $$(cat app.pid)"
	@echo "Logs: tail -f app.log"