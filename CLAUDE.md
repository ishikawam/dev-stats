# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go-based tool that analyzes GitHub, Backlog, and Calendar productivity by fetching and summarizing activity data within specified date ranges. The tool provides statistics on pull requests, issues, activities, and calendar events across different repositories, organizations, and time periods.

## Architecture

The project uses a command-based structure with three main entry points:
- `cmd/github/main.go` - Analyzes GitHub pull request activity using the GitHub Search API
- `cmd/backlog/main.go` - Analyzes Backlog issue and activity data using the Backlog REST API
- `cmd/calendar/main.go` - Analyzes calendar events from ICS files with comprehensive statistics

All commands follow a similar pattern:
1. Load environment variables from `.env` file using `godotenv`
2. Fetch data from respective APIs with pagination support
3. Process and aggregate the data by organization/repository or activity type
4. Output detailed summaries and statistics

## Environment Setup

The application requires environment variables to be set in a `.env` file:

**GitHub analysis:**
- `GITHUB_TOKEN` - Personal access token with `repo` and `read:org` scopes
- `GITHUB_USERNAME` - GitHub username to analyze
- `START_DATE` / `END_DATE` - Date range in YYYY-MM-DD format

**Backlog analysis:**
- `BACKLOG_API_KEY` - API key from Backlog space settings
- `BACKLOG_SPACE_NAME` - Backlog space name
- `BACKLOG_USER_ID` - User ID (integer)
- `BACKLOG_PROJECT_ID` - Project ID (integer)
- `START_DATE` / `END_DATE` - Date range in YYYY-MM-DD format

**Calendar analysis:**
- `START_DATE` / `END_DATE` - Date range in YYYY-MM-DD format for filtering events
- ICS files should be placed in `storage/calendar/` directory

Use `.env.example` as a template.

## Common Commands

**Install dependencies:**
```bash
make install
# or: go mod tidy
```

**Run analysis:**
```bash
make run-github    # GitHub analysis
make run-backlog   # Backlog analysis
make run-calendar  # Calendar analysis
```

**Alternative direct execution:**
```bash
go run cmd/github/main.go
go run cmd/backlog/main.go
go run cmd/calendar/main.go
```

**Code quality checks:**
```bash
make fmt    # Format code
make vet    # Run go vet
make check  # Run all checks
```

## Key Implementation Details

**GitHub API Integration:**
- Uses GitHub Search API (`/search/issues`) with query parameters for PR filtering
- Handles pagination automatically (100 PRs per page)
- Fetches both "involves" (PRs you participated in) and "author" (PRs you created) data
- Aggregates data by organization and repository for summary statistics

**Backlog API Integration:**
- Uses Backlog REST API v2 for issues and user activities
- Implements activity pagination using `maxId` parameter
- Tracks unique issues across different activity types
- Maps activity type integers to human-readable descriptions

**Calendar Analysis Integration:**
- Parses ICS (iCalendar) files from `storage/calendar/` directory
- Supports multiple datetime formats: UTC (`YYYYMMDDTHHMMSSZ`), timezone-aware (`DTSTART;TZID=Asia/Tokyo`), and date-only (`VALUE=DATE`)
- Detects all-day events using both `VALUE=DATE` format and duration-based heuristics (24-hour or multiples)
- Comprehensive error handling with detailed logging for parsing issues
- Provides three ranking systems: event count, duration (excluding all-day), and all-day event days

**Data Processing:**
- Deduplicates PRs/issues using URL/ID as unique keys
- Sorts output alphabetically by organization/repository names or by ranking criteria
- Provides both detailed listings and comprehensive statistics
- Filters activities/events by date range during processing
- Calendar events display duration indicators with special handling for all-day events (`(-)` marker)