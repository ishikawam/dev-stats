# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## ⚠️ IMPORTANT RULES

**GIT COMMIT POLICY**
- **ALWAYS write commit messages in English** - This is an open source project with English documentation
- Use conventional commit format: `type: description` (e.g., `feat:`, `fix:`, `refactor:`, `docs:`)

**NEVER COMMIT CODE WITHOUT EXPLICIT USER REQUEST**
- Do NOT run `git commit` unless the user explicitly asks to commit
- Do NOT run `git add` and `git commit` automatically
- Always ask for confirmation before committing changes
- The user wants to review changes before they are committed to version control
- This rule is non-negotiable and must be followed at all times

**PRIVACY AND SECURITY RULES**
- NEVER delete or modify files that are NOT tracked by git (use `git ls-files` to check)
- Personal `.env` files contain private tokens and credentials - do NOT modify them unless explicitly asked
- Files in `notion-downloads/` and user-created files in `notion-urls/` contain private data
- Only modify tracked template files like `.env.example` and `.sample.md`
- When working with private data, always distinguish between:
  - Git-tracked files (safe to modify for open source)
  - User's private files (never touch without permission)
- Always check `git status` and `git ls-files` before making changes to understand what's tracked
- NEVER hardcode private information (URLs, names, tokens) in code or tracked files

## Project Overview

A Go-based tool that analyzes GitHub, Backlog, Calendar, and Notion productivity by fetching and summarizing activity data within specified date ranges. The tool provides statistics on pull requests, issues, activities, calendar events, and Notion pages across different repositories, organizations, and time periods.

## Architecture

The project has been refactored to use a unified architecture with:

**Unified Command Structure:**
- `cmd/dev-stats/main.go` - Main unified command that can run any analyzer
- `pkg/common/` - Shared libraries (HTTP client, config, error handling, analyzer interface)
- `pkg/github/analyzer.go` - GitHub analysis implementation
- `pkg/backlog/analyzer.go` - Backlog analysis implementation  
- `pkg/calendar/analyzer.go` - Calendar analysis implementation
- `pkg/notion/analyzer.go` - Notion analysis implementation


All analyzers implement the common `Analyzer` interface with methods:
- `GetName()` - Returns analyzer name
- `Analyze(config)` - Performs analysis and returns results
- `ValidateConfig()` - Validates required configuration

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

**Notion analysis:**
- `NOTION_TOKEN` - Notion integration token with content read access
- `NOTION_USER_ID` - (Optional) Specific user ID to filter pages by. If not provided, auto-detected user ID will be used
- `START_DATE` / `END_DATE` - Date range in YYYY-MM-DD format

Use `.env.example` as a template.

## Common Commands

**Install dependencies:**
```bash
make install
# or: go mod tidy
```

**Build unified command:**
```bash
make build
```

**Run analysis (unified command):**
```bash
make run-github     # GitHub analysis
make run-backlog    # Backlog analysis  
make run-calendar   # Calendar analysis
make run-notion     # Notion analysis
make run-all        # All analyzers

# Direct execution with flags:
./dev-stats -analyzer github
./dev-stats -analyzer backlog,calendar
./dev-stats -analyzer all
./dev-stats -list    # List available analyzers
./dev-stats -help    # Show help
```

**Notion page download:**
```bash
make download       # Downloads Notion pages specified in notion-urls/${START_DATE}_to_${END_DATE}.md
# or: ./dev-stats -download notion-urls/YYYY-MM-DD_to_YYYY-MM-DD.md
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

**Notion API Integration:**
- Uses Notion API v1 with Integration Token authentication
- Auto-detects user ID from workspace pages to handle token vs workspace user ID mismatch
- Supports custom user ID specification via `NOTION_USER_ID` environment variable
- Client-side filtering by date range and user involvement (created or edited pages)
- Smart pagination with early termination for performance optimization
- Caches database titles and user names to minimize API calls
- Extracts page titles from properties with `type: "title"`

**Notion Page Downloader:**
- Downloads specific Notion pages to markdown files based on URLs listed in markdown files
- Automatically updates page titles with actual Notion page titles after fetching
- Uses category names from markdown file as directory names (no hardcoded mappings)
- Preserves spaces in filenames, only sanitizes filesystem-incompatible characters
- Automatically updates the original markdown file with actual page titles
- File structure: `notion-downloads/YYYY-MM-DD_to_YYYY-MM-DD/Category Name/Page Title.md`
- Creates comprehensive markdown output with metadata, properties, and full content

**Data Processing:**
- Deduplicates PRs/issues using URL/ID as unique keys
- Sorts output alphabetically by organization/repository names or by ranking criteria
- Provides both detailed listings and comprehensive statistics
- Filters activities/events by date range during processing
- Calendar events display duration indicators with special handling for all-day events (`(-)` marker)
- Notion pages categorized as "created" vs "updated" based on user involvement