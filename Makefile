# Makefile for dev-stats

# Default target
help:
	@echo "Available targets:"
	@echo "  help         - Show this help message"
	@echo "  install      - Install dependencies"
	@echo "  build        - Build the unified dev-stats command"
	@echo "  run-github   - Run GitHub analysis"
	@echo "  run-backlog  - Run Backlog analysis"
	@echo "  run-calendar - Run Calendar analysis"
	@echo "  run-notion   - Run Notion analysis"
	@echo "  run-all      - Run all analyzers"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  check        - Run fmt, vet, and test"

# Install dependencies
install:
	go mod tidy
	go mod download

# Build the unified dev-stats command
build:
	go build -o dev-stats cmd/dev-stats/main.go

# Run GitHub analysis
run-github: build
	./dev-stats -analyzer github

# Run Backlog analysis
run-backlog: build
	./dev-stats -analyzer backlog

# Run Calendar analysis
run-calendar: build
	./dev-stats -analyzer calendar

# Run Notion analysis
run-notion: build
	./dev-stats -analyzer notion

# Run all analyzers
run-all: build
	./dev-stats -analyzer all


# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run all checks
check: fmt vet
	@echo "All checks passed!"