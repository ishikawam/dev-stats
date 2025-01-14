
# dev-stats

[![Go Report Card](https://goreportcard.com/badge/github.com/ishikawam/dev-stats)](https://goreportcard.com/report/github.com/ishikawam/dev-stats)

A tool to analyze your GitHub and Backlog productivity by fetching and summarizing activity data within a specified date range.

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
   go run cmd/github/main.go
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
   go run cmd/backlog/main.go
   ```

3. **View the output**:
    - The results, including activity details, issue counts, and summaries, will be displayed in your terminal.

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
