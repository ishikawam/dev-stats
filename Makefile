# Makefile for dev-stats

# Default target
help:
	@echo "Available targets:"
	@echo "  help         - Show this help message"
	@echo "  install      - Install dependencies"
	@echo "  run-github   - Run GitHub analysis"
	@echo "  run-backlog  - Run Backlog analysis"
	@echo "  run-calendar - Run Calendar analysis"
	@echo "  run-notion   - Run Notion analysis"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  check        - Run fmt, vet, and test"

# Install dependencies
install:
	go mod tidy
	go mod download

# Run GitHub analysis
run-github:
	go run cmd/github/main.go

# Run Backlog analysis
run-backlog:
	go run cmd/backlog/main.go

# Run Calendar analysis
run-calendar:
	go run cmd/calendar/main.go

# Run Notion analysis
run-notion:
	go run cmd/notion/main.go

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run all checks
check: fmt vet
	@echo "All checks passed!"
