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
- Files in `output/` and user-created files in `notion-urls/` contain private data
- Only modify tracked template files like `.env.example` and `.sample.md`
- When working with private data, always distinguish between:
  - Git-tracked files (safe to modify for open source)
  - User's private files (never touch without permission)
- Always check `git status` and `git ls-files` before making changes to understand what's tracked
- NEVER hardcode private information (URLs, names, tokens) in code or tracked files

## Project Overview

A Go-based tool that analyzes GitHub, Backlog, Calendar, Notion, and Google Workspace productivity by fetching and summarizing activity data within specified date ranges. The tool provides statistics on pull requests, issues, activities, calendar events, Notion pages, and Google Workspace files across different repositories, organizations, and time periods.

## Architecture

The project uses a unified architecture with:

**Unified Command Structure:**
- `cmd/dev-stats/main.go` - Main unified command that can run any analyzer
- `pkg/common/` - Shared libraries (HTTP client, config, error handling, analyzer interface)
- `pkg/github/analyzer.go` - GitHub analysis implementation
- `pkg/backlog/analyzer.go` - Backlog analysis implementation
- `pkg/calendar/analyzer.go` - Calendar analysis implementation
- `pkg/notion/analyzer.go` - Notion analysis implementation
- `pkg/google/analyzer.go` - Google Workspace analysis implementation (Docs/Slides/Sheets)

All analyzers implement the common `Analyzer` interface with methods:
- `GetName()` - Returns analyzer name
- `Analyze(config)` - Performs analysis and returns results
- `ValidateConfig()` - Validates required configuration

## Output Directory Structure

All output is written under `output/YYYY-MM-DD_to_YYYY-MM-DD/`:
- `stats/` - Analysis result text files (run-*)
- `notion/` - Downloaded Notion pages
- `google/` - Downloaded Google Workspace files
  - `docs/` - Google Docs as Markdown
  - `slides/` - Google Slides as Markdown (plain text)
  - `sheets/` - Google Sheets as CSV
  - `.cache/` - Revision check cache

## Environment Setup

The application requires environment variables to be set in a `.env` file. Use `.env.example` as a template.

**GitHub analysis:**
- `GITHUB_TOKEN` - Personal access token with `repo` and `read:org` scopes
- `GITHUB_USERNAME` - GitHub username to analyze

**Backlog analysis:**
- `BACKLOG_<PROFILE>_API_KEY` - API key from Backlog space settings
- `BACKLOG_<PROFILE>_HOST` - Backlog host (e.g., `mycompany.backlog.com`)
- `BACKLOG_<PROFILE>_USER_ID` - User ID (integer, optional)
- `BACKLOG_<PROFILE>_PROJECT_ID` - Project ID (integer, optional)

**Calendar analysis:**
- ICS files should be placed in `storage/calendar/` directory

**Notion analysis:**
- `NOTION_TOKEN` - Notion integration token with content read access
- `NOTION_USER_ID` - (Optional) Specific user ID to filter pages by

**Google Workspace analysis:**
- `GOOGLE_CLIENT_ID` - OAuth2 client ID (from GCP Console)
- `GOOGLE_CLIENT_SECRET` - OAuth2 client secret
- `GOOGLE_TOKEN_FILE` - (Optional) Token cache path (default: `storage/google_token.json`)
- `GOOGLE_DOCS_RELATED_NAMES` - (Optional) Comma-separated keywords to match related files by title
- `GOOGLE_DOCS_CHECK_REVISIONS` - (Optional) Set to `true` to check revision history of excluded files

**All analyzers:**
- `START_DATE` / `END_DATE` - Date range in YYYY-MM-DD format

## Common Commands

**Install dependencies:**
```bash
make install
```

**Build:**
```bash
make build
```

**Run analysis:**
```bash
make run-github
make run-backlog
make run-calendar
make run-notion
make run-google
make run-all

# Direct execution:
./bin/dev-stats -analyzer github
./bin/dev-stats -analyzer google
./bin/dev-stats -analyzer all
./bin/dev-stats -list
./bin/dev-stats -help
```

**Download:**
```bash
make download-notion   # Downloads Notion pages specified in notion-urls/${START_DATE}_to_${END_DATE}.md
make download-google   # Downloads Google Workspace files modified in date range
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
- Provides three ranking systems: event count, duration (excluding all-day), and all-day event days

**Notion API Integration:**
- Uses Notion API v1 with Integration Token authentication
- Auto-detects user ID from workspace pages to handle token vs workspace user ID mismatch
- Client-side filtering by date range and user involvement (created or edited pages)
- Smart pagination with early termination for performance optimization
- Caches database titles and user names to minimize API calls

**Notion Page Downloader:**
- Downloads specific Notion pages to markdown files based on URLs listed in markdown files
- Uses category names from markdown file as directory names (no hardcoded mappings)
- Automatically updates the original markdown file with actual page titles
- File structure: `output/YYYY-MM-DD_to_YYYY-MM-DD/notion/<Category Name>/<Page Title>.md`

**Google Workspace Integration:**
- Uses Google Drive API v3 to list Docs/Slides/Sheets (`'me' in owners or 'me' in writers`)
- OAuth2 authentication with localhost callback; token cached in `storage/google_token.json`
- Categorizes files as created / updated / related / excluded
- Optional revision history check (`GOOGLE_DOCS_CHECK_REVISIONS=true`) with cache in `.cache/revision-cache.json`
- Export formats: Docs→Markdown, Slides→plain text (.md), Sheets→CSV
- Skips already-downloaded files based on local vs Drive modification time

**Data Processing:**
- Deduplicates PRs/issues using URL/ID as unique keys
- Sorts output alphabetically by organization/repository names or by ranking criteria
- Filters activities/events by date range during processing
- Calendar events display duration indicators with special handling for all-day events (`(-)` marker)
- Notion pages categorized as "created" vs "updated" based on user involvement
