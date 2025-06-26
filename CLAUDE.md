# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go-based tool that analyzes GitHub and Backlog productivity by fetching and summarizing activity data within specified date ranges. The tool provides statistics on pull requests, issues, and activities across different repositories and organizations.

## Architecture

The project uses a command-based structure with two main entry points:
- `cmd/github/main.go` - Analyzes GitHub pull request activity using the GitHub Search API
- `cmd/backlog/main.go` - Analyzes Backlog issue and activity data using the Backlog REST API

Both commands follow a similar pattern:
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

Use `.env.example` as a template.

## Common Commands

**Install dependencies:**
```bash
go mod tidy
```

**Run GitHub analysis:**
```bash
go run cmd/github/main.go
```

**Run Backlog analysis:**
```bash
go run cmd/backlog/main.go
```

**Build executables:**
```bash
go build -o github-stats cmd/github/main.go
go build -o backlog-stats cmd/backlog/main.go
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

**Data Processing:**
- Deduplicates PRs/issues using URL/ID as unique keys
- Sorts output alphabetically by organization/repository names
- Provides both detailed listings and summary statistics
- Filters activities by date range during processing