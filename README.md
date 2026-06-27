
# dev-stats

[![Go Report Card](https://goreportcard.com/badge/github.com/ishikawam/dev-stats)](https://goreportcard.com/report/github.com/ishikawam/dev-stats)

A tool to analyze your GitHub, Backlog, Calendar, Notion, and Google Workspace productivity by fetching and summarizing activity data within a specified date range.

## Usage

### GitHub

1. **Clone this repository**:
   ```bash
   git clone https://github.com/ishikawam/dev-stats.git
   cd dev-stats
   ```

2. **Set up your environment variables**:
    - Create a `.env` file in the project root directory:
      ```plaintext
      # .env

      GITHUB_TOKEN=your-github-token
      GITHUB_USERNAME=your-github-username

      # YYYY-MM-DD format
      START_DATE=2024-01-01
      END_DATE=2024-06-30
      ```
    - Alternatively, export the variables in your terminal:
      ```bash
      export GITHUB_TOKEN=your-github-token
      export GITHUB_USERNAME=your-github-username
      export START_DATE=2024-01-01
      export END_DATE=2024-06-30
      ```

3. **Install dependencies**:
    - Ensure you have Go installed (version 1.23.4 or later).
    - Install dependencies listed in `go.mod`:
      ```bash
      go mod tidy
      ```

4. **Run the tool**:
   ```bash
   make run-github
   ```

5. **View the output**:
    - The results, including PR details and summaries, will be displayed in your terminal.

### Backlog

**Multi-Profile Support**: This tool supports multiple Backlog accounts with both `.backlog.com` and `.backlog.jp` domains.

1. **Set up your environment variables**:
    - Use the pattern `BACKLOG_<PROFILE>_<SETTING>` to define multiple profiles
    - Update the `.env` file in the project root directory:
      ```plaintext
      # .env

      # Profile 1: HOGE (backlog.com)
      BACKLOG_HOGE_API_KEY=your-api-key-1
      BACKLOG_HOGE_HOST=mycompany.backlog.com
      BACKLOG_HOGE_USER_ID=123456
      BACKLOG_HOGE_PROJECT_ID=789012

      # Profile 2: FUGA (backlog.jp)
      BACKLOG_FUGA_API_KEY=your-api-key-2
      BACKLOG_FUGA_HOST=projectspace.backlog.jp
      BACKLOG_FUGA_USER_ID=234567
      BACKLOG_FUGA_PROJECT_ID=890123

      # Date range (YYYY-MM-DD format)
      START_DATE=2024-01-01
      END_DATE=2024-06-30
      ```
    - **Finding USER_ID and PROJECT_ID**:
      ```bash
      # List all configured profiles
      make list-backlog-profiles

      # List all projects and members for all profiles
      make list-backlog
      ```

2. **Run the tool**:
   ```bash
   make run-backlog
   ```
   - This command runs analysis for **all configured profiles**

3. **View the output**:
    - Results for each profile will be displayed separately in your terminal
    - Output files are saved to `output/YYYY-MM-DD_to_YYYY-MM-DD/stats/backlog-<profile>-stats.txt`

### Calendar

Two sources are supported and can be used together (merged with UID-based deduplication).

**Option A: ICS file (offline export)**

1. Export your calendar from Google Calendar:
   1. Open Google Calendar settings (⚙️ → Settings → Import & Export)
   2. Click "Export" to download all calendars as a ZIP file
   3. Extract the ZIP and copy your `.ics` file to `storage/calendar/` directory
2. Run the tool:
   ```bash
   make run-calendar
   ```

**Option B: Google Calendar API (live fetch)**

Fetches events from your primary calendar using the same OAuth2 credentials as Google Workspace.

1. Set up OAuth2 credentials (see [Google Workspace](#google-workspace) section).
   - Additionally enable the **Google Calendar API** in GCP Console.
2. Set up your environment variables:
   ```plaintext
   GOOGLE_CLIENT_ID=your-oauth2-client-id
   GOOGLE_CLIENT_SECRET=your-oauth2-client-secret
   ```
3. Run the tool:
   ```bash
   make run-calendar
   ```
   On the first run, a browser window opens for OAuth2 authentication.

**View the output**:
- The results include event count rankings, duration rankings, and all-day event rankings.

### Notion

1. **Set up your environment variables**:
    - Update the `.env` file in the project root directory:
      ```plaintext
      # .env

      NOTION_TOKEN=your-notion-integration-token
      # Optional: Specific user ID to filter pages by
      NOTION_USER_ID=your-notion-user-id

      # YYYY-MM-DD format
      START_DATE=2024-01-01
      END_DATE=2024-06-30
      ```
    - Alternatively, export the variables in your terminal:
      ```bash
      export NOTION_TOKEN=your-notion-integration-token
      export NOTION_USER_ID=your-notion-user-id  # Optional
      export START_DATE=2024-01-01
      export END_DATE=2024-06-30
      ```

2. **Run the tool**:
   ```bash
   make run-notion
   ```

3. **View the output**:
    - The tool automatically detects your user ID from workspace pages (or uses the specified `NOTION_USER_ID`)
    - The results include pages you created and updated, with URLs and timestamps
    - Includes timekeeper entries and categorized work analysis

### Google Workspace

Analyzes Google Docs, Slides, and Sheets you created, updated, or are related to.

1. **Create OAuth2 credentials**:
   - Go to [GCP Console](https://console.cloud.google.com/apis/credentials)
   - Create an OAuth2 client ID (Application type: Desktop app)
   - Enable the Google Drive API

2. **Set up your environment variables**:
   ```plaintext
   GOOGLE_CLIENT_ID=your-oauth2-client-id
   GOOGLE_CLIENT_SECRET=your-oauth2-client-secret

   # Optional: comma-separated keywords to identify related files by title
   # GOOGLE_DOCS_RELATED_NAMES=YourName,TeamName,ProjectName

   # Optional: check revision history of excluded files (slower, adds ~200ms per file)
   # GOOGLE_DOCS_CHECK_REVISIONS=true
   ```

3. **Run the analysis**:
   ```bash
   make run-google
   ```
   On the first run, a browser window opens for OAuth2 authentication. The token is cached in `storage/google_token.json`.

4. **Download files**:
   ```bash
   make download-google
   ```
   Files are saved to `output/YYYY-MM-DD_to_YYYY-MM-DD/google/`:
   - `docs/` — Google Docs exported as Markdown
   - `slides/` — Google Slides exported as Markdown (plain text)
   - `sheets/` — Google Sheets exported as CSV

   Files already downloaded are skipped based on modification time.

**File categorization**:
- **created**: Files you own, created within the date range
- **updated**: Files where you are the last modifier
- **related**: Files matching `GOOGLE_DOCS_RELATED_NAMES` keywords in the title
- **revision**: Files in the excluded list where you appear in revision history (requires `GOOGLE_DOCS_CHECK_REVISIONS=true`)

### Notion Page Download

You can download specific Notion pages (e.g., 1on1 notes, MBO memos) as markdown files.

**Setup for each period:**

1. **Edit `notion-urls/YYYY-MM-DD_to_YYYY-MM-DD.md`**:
   - The file matching `START_DATE_to_END_DATE` is used automatically
   - Add newly created pages that are not in the previous period file
   - Remove pages that are no longer relevant

   Format:
   ```markdown
   # Notion Pages to Download (YYYY-MM-DD to YYYY-MM-DD)

   ## Category Name
   - Page Title
       - https://www.notion.so/page-url
   ```

2. **Download pages**:
   ```bash
   make download-notion
   ```
   Pages are saved to `output/YYYY-MM-DD_to_YYYY-MM-DD/notion/<Category Name>/<Page Title>.md`

## Unified Command Usage

The project has been refactored to provide a unified command interface. You can now run any analyzer using the `dev-stats` command:

```bash
# Build the unified command
make build

# Run individual analyzers
./bin/dev-stats -analyzer github
./bin/dev-stats -analyzer backlog
./bin/dev-stats -analyzer calendar
./bin/dev-stats -analyzer notion
./bin/dev-stats -analyzer google

# Run multiple analyzers
./bin/dev-stats -analyzer github,backlog

# Run all analyzers
./bin/dev-stats -analyzer all

# Show help and available options
./bin/dev-stats -help
./bin/dev-stats -list
```

## Example `.env` File

```plaintext
# .env

# GitHub
GITHUB_TOKEN=your-github-token
GITHUB_USERNAME=your-github-username

# Backlog (Multi-Profile Support)
# Pattern: BACKLOG_<PROFILE>_<SETTING>
BACKLOG_HOGE_API_KEY=your-api-key-1
BACKLOG_HOGE_HOST=mycompany.backlog.com
BACKLOG_HOGE_USER_ID=123456
BACKLOG_HOGE_PROJECT_ID=789012

BACKLOG_FUGA_API_KEY=your-api-key-2
BACKLOG_FUGA_HOST=projectspace.backlog.jp
BACKLOG_FUGA_USER_ID=234567
BACKLOG_FUGA_PROJECT_ID=890123

# Notion
NOTION_TOKEN=your-notion-integration-token
# Optional: Specific user ID to filter pages by
NOTION_USER_ID=your-notion-user-id

# Google Workspace (Docs/Slides/Sheets)
GOOGLE_CLIENT_ID=your-oauth2-client-id
GOOGLE_CLIENT_SECRET=your-oauth2-client-secret
# GOOGLE_DOCS_RELATED_NAMES=YourName,TeamName,ProjectName
# GOOGLE_DOCS_CHECK_REVISIONS=true

# Calendar analysis requires START_DATE and END_DATE for filtering events

# Specify the date range in YYYY-MM-DD format
START_DATE=2024-01-01
END_DATE=2024-06-30
```

## Sample Output

### GitHub

```plaintext
Pull Requests you were involved in (created or merged) from 2024-01-01 to 2024-06-30:
- Title: Go version update
  URL: https://github.com/example-org/project-api/pull/123
  Created At: 2024-02-13T07:53:44Z
  Repository: https://github.com/example-org/project-api

- Title: Actions plugin update
  URL: https://github.com/example-org/project-api/pull/124
  Created At: 2024-02-20T06:44:46Z
  Repository: https://github.com/example-org/project-api

...

Pull Requests summary from 2024-01-01 to 2024-06-30:

Total PRs: 210

Total PRs (author): 54
Total PRs (involves): 210

PR count per organization (author/involves):
- example-org: 21 (133)
- demo-inc: 33 (77)

PR count per repository (author/involves):
- example-org/project-api: 8 (20)
- example-org/project-cms: 2 (2)
- example-org/project-k8s: 4 (20)
- demo-inc/project-server: 22 (62)
...
```

### Backlog

```plaintext
Issues you created: 3
Issues assigned to you: 6

Activity count by type (count issues):
- 1. Issue created: 3 (3)
- 2. Comment added: 20 (8)
- 3. Status changed: 10 (5)

Your issues count: 21
```

### Calendar

```plaintext
Reading calendar file: storage/calendar/calendar.ics
Successfully parsed 1234 events from storage/calendar/calendar.ics

Total events parsed from all files: 1234

Calendar events from 2024-01-01 to 2024-01-31:
- 2024-01-01 15:00: Holiday (-)
- 2024-01-02 09:00: Team Meeting (1h0m)
- 2024-01-03 10:30: Project Review (2h0m)
...

Calendar summary from 2024-01-01 to 2024-01-31:
Total events: 156
Total duration: 245h30m

Events by title ranking:

Top events by count (all):
 1. Daily Standup: 22 events (11h0m)
 2. Project Review: 8 events (16h0m)
 3. Team Meeting: 6 events (6h0m)
...

Top events by total duration (all):
 1. Project Review: 16h0m (8 events)
 2. Daily Standup: 11h0m (22 events)
 3. Deep Work: 8h30m (3 events)
...

All-day events ranking by total days (all):
 1. Holiday: 3 days (3 events)
 2. Vacation: 2 days (1 events)
...
```

## Makefile Commands

This project includes a Makefile for convenient development and execution:

```bash
# Show available commands
make help

# Install dependencies
make install

# Build the unified command
make build

# Run specific analysis (unified command)
make run-github
make run-backlog
make run-calendar
make run-notion
make run-google     # Google Workspace (Docs/Slides/Sheets)
make run-all        # Run all analyzers

# Download files
make download-notion       # Download Notion pages listed in notion-urls/
make download-google       # Download Google Workspace files

# Code quality checks
make fmt        # Format code
make vet        # Run go vet
make check      # Run all checks (fmt, vet)
```

## Requirements

- **Go**: Version 1.23.4 or later.
- **GitHub Personal Access Token**:
    - Generate a token with the following scopes:
        - `repo`
        - `read:org`
    - See GitHub's [documentation](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token) for more details.
- **Backlog API Key**:
    - Generate a key from your Backlog space settings.
    - See Backlog's [API documentation](https://developer.nulab.com/docs/backlog/#api-key) for more details.
- **Notion Integration Token**:
    - Create an integration at https://www.notion.so/my-integrations
    - Copy the Integration Token
    - Share your workspace pages with the integration
    - Optionally specify `NOTION_USER_ID` to filter pages for a specific user
- **Google OAuth2 Credentials** (for Google Workspace and/or Google Calendar analysis):
    - Create OAuth2 credentials at [GCP Console](https://console.cloud.google.com/apis/credentials) (Application type: Desktop app)
    - Enable the Google Drive API (for Workspace) and/or Google Calendar API (for Calendar)
    - The token is cached in `storage/google_token.json` after the first authentication

## Notes

- **Custom Date Range**: Specify the `START_DATE` and `END_DATE` in the `.env` file or environment variables to fetch data for a specific period.
- **Output Details**:
    - GitHub: PRs you were involved in as an author or reviewer, summary of PR counts per organization and repository.
    - Backlog: Activity count by type, unique issues involved, and summaries.
    - Calendar: Event listings with duration indicators, rankings by count/duration/days, all-day event detection.
    - Notion: Pages you created or updated, with URLs and activity timestamps, including timekeeper entries and work category analysis.
    - Google Workspace: Docs/Slides/Sheets categorized by your involvement (created/updated/related/revision history), downloaded to `output/YYYY-MM-DD_to_YYYY-MM-DD/google/`.
- **Architecture**: The project uses a unified architecture with common libraries and interfaces, making it easy to extend with new analyzers.
