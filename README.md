
# dev-stats

[![Go Report Card](https://goreportcard.com/badge/github.com/ishikawam/dev-stats)](https://goreportcard.com/report/github.com/ishikawam/dev-stats)

A tool to analyze your GitHub, Backlog, and Calendar productivity by fetching and summarizing activity data within a specified date range.

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

1. **Set up your environment variables**:
    - Update the `.env` file in the project root directory:
      ```plaintext
      # .env

      BACKLOG_API_KEY=your-backlog-api-key
      BACKLOG_SPACE_NAME=your-space-name
      BACKLOG_USER_ID=your-user-id
      BACKLOG_PROJECT_ID=your-project-id

      # YYYY-MM-DD format
      START_DATE=2024-01-01
      END_DATE=2024-06-30
      ```
    - Alternatively, export the variables in your terminal:
      ```bash
      export BACKLOG_API_KEY=your-backlog-api-key
      export BACKLOG_SPACE_NAME=your-space-name
      export BACKLOG_USER_ID=your-user-id
      export BACKLOG_PROJECT_ID=your-project-id
      export START_DATE=2024-01-01
      export END_DATE=2024-06-30
      ```

2. **Run the tool**:
   ```bash
   make run-backlog
   ```

3. **View the output**:
    - The results, including activity details, issue counts, and summaries, will be displayed in your terminal.

### Calendar

1. **Set up ICS files**:
    - Place your calendar ICS files in the `storage/calendar/` directory:
      ```bash
      mkdir -p storage/calendar
      # Copy your .ics files to storage/calendar/
      ```

2. **Set up your environment variables**:
    - Update the `.env` file in the project root directory:
      ```plaintext
      # YYYY-MM-DD format
      START_DATE=2024-01-01
      END_DATE=2024-06-30
      ```

3. **Run the tool**:
   ```bash
   make run-calendar
   ```

4. **View the output**:
    - The results include detailed event listings, event count rankings, duration rankings, and all-day event rankings.

## Example `.env` File

```plaintext
# .env

# GitHub
GITHUB_TOKEN=your-github-token
GITHUB_USERNAME=your-github-username

# Backlog
BACKLOG_API_KEY=your-backlog-api-key
BACKLOG_SPACE_NAME=your-space-name
BACKLOG_USER_ID=your-user-id
BACKLOG_PROJECT_ID=your-project-id

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
- 2024-01-01 15:00: 祝日 (-)
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
 1. 祝日: 3 days (3 events)
 2. 休暇: 2 days (1 events)
...
```

## Makefile Commands

This project includes a Makefile for convenient development and execution:

```bash
# Show available commands
make help

# Install dependencies
make install

# Run specific analysis
make run-github
make run-backlog
make run-calendar

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

## Notes

- **Custom Date Range**: Specify the `START_DATE` and `END_DATE` in the `.env` file or environment variables to fetch data for a specific period.
- **Output Details**:
    - GitHub: PRs you were involved in as an author or reviewer, summary of PR counts per organization and repository.
    - Backlog: Activity count by type, unique issues involved, and summaries.
    - Calendar: Event listings with duration indicators, rankings by count/duration/days, all-day event detection.
