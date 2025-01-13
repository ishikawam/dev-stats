# github-productivity

[![Go Report Card](https://goreportcard.com/badge/github.com/ishikawam/github-productivity)](https://goreportcard.com/report/github.com/ishikawam/github-productivity)

A tool to analyze your GitHub productivity by fetching PR data within a specified date range.

## Usage

1. **Clone this repository**:
   ```bash
   git clone https://github.com/ishikawam/github-productivity.git
   cd github-productivity
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
   go run main.go
   ```

5. **View the output**:
    - The results, including PR details and summaries, will be displayed in your terminal.

## Example `.env` File

```plaintext
# .env

GITHUB_TOKEN=your-github-token
GITHUB_USERNAME=your-github-username

# Specify the date range in YYYY-MM-DD format
START_DATE=2024-01-01
END_DATE=2024-06-30
```

## Requirements

- **Go**: Version 1.23.4 or later.
- **GitHub Personal Access Token**:
    - Generate a token with the following scopes:
        - `repo`
        - `read:org`
    - See GitHub's [documentation](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token) for more details.

## Notes

- **Custom Date Range**: Specify the `START_DATE` and `END_DATE` in the `.env` file or environment variables to fetch PRs for a specific period.
- **Output Details**:
    - PRs you were involved in as an author or reviewer.
    - Summary of PR counts per organization and repository.
