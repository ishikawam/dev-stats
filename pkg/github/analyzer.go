package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"dev-stats/pkg/common"
)

// GitHubAnalyzer implements the Analyzer interface for GitHub
type GitHubAnalyzer struct {
	token    string
	username string
	client   *common.HTTPClient
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Title     string    `json:"title"`
	URL       string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	RepositoryURL string `json:"repository_url"`
}

// SearchResponse represents GitHub search API response
type SearchResponse struct {
	TotalCount int           `json:"total_count"`
	Items      []PullRequest `json:"items"`
}

// NewGitHubAnalyzer creates a new GitHub analyzer
func NewGitHubAnalyzer() *GitHubAnalyzer {
	return &GitHubAnalyzer{
		token:    os.Getenv("GITHUB_TOKEN"),
		username: os.Getenv("GITHUB_USERNAME"),
		client:   common.NewHTTPClient(),
	}
}

// GetName returns the analyzer name
func (g *GitHubAnalyzer) GetName() string {
	return "GitHub"
}

// ValidateConfig validates the required configuration
func (g *GitHubAnalyzer) ValidateConfig() error {
	if g.token == "" {
		return common.NewError("GITHUB_TOKEN environment variable is required")
	}
	if g.username == "" {
		return common.NewError("GITHUB_USERNAME environment variable is required")
	}
	return nil
}

// Analyze performs GitHub analysis
func (g *GitHubAnalyzer) Analyze(config *common.Config) (*common.AnalysisResult, error) {
	if err := g.ValidateConfig(); err != nil {
		return nil, err
	}

	g.client.SetHeader("Authorization", "token "+g.token)
	g.client.SetHeader("Accept", "application/vnd.github.v3+json")

	fmt.Printf("Analyzing GitHub activity for user: %s\n", g.username)
	fmt.Printf("Date range: %s to %s\n", config.StartDate.Format("2006-01-02"), config.EndDate.Format("2006-01-02"))

	// Get PRs where user is involved
	involvedPRs, err := g.searchPRs("involves:"+g.username, config.StartDate, config.EndDate)
	if err != nil {
		return nil, common.WrapError(err, "failed to search involved PRs")
	}

	// Get PRs authored by user
	authoredPRs, err := g.searchPRs("author:"+g.username, config.StartDate, config.EndDate)
	if err != nil {
		return nil, common.WrapError(err, "failed to search authored PRs")
	}

	// Analyze results
	orgStats := make(map[string]struct{ authored, involved int })
	repoStats := make(map[string]struct{ authored, involved int })

	for _, pr := range authoredPRs {
		fullName := g.extractRepoFromURL(pr.RepositoryURL)
		repoName := g.extractRepoName(fullName)
		orgName := g.extractOrgName(fullName)

		if stat, exists := orgStats[orgName]; exists {
			stat.authored++
			orgStats[orgName] = stat
		} else {
			orgStats[orgName] = struct{ authored, involved int }{authored: 1, involved: 0}
		}

		if stat, exists := repoStats[repoName]; exists {
			stat.authored++
			repoStats[repoName] = stat
		} else {
			repoStats[repoName] = struct{ authored, involved int }{authored: 1, involved: 0}
		}
	}

	for _, pr := range involvedPRs {
		fullName := g.extractRepoFromURL(pr.RepositoryURL)
		repoName := g.extractRepoName(fullName)
		orgName := g.extractOrgName(fullName)

		if stat, exists := orgStats[orgName]; exists {
			stat.involved++
			orgStats[orgName] = stat
		} else {
			orgStats[orgName] = struct{ authored, involved int }{authored: 0, involved: 1}
		}

		if stat, exists := repoStats[repoName]; exists {
			stat.involved++
			repoStats[repoName] = stat
		} else {
			repoStats[repoName] = struct{ authored, involved int }{authored: 0, involved: 1}
		}
	}

	// Create result
	result := &common.AnalysisResult{
		AnalyzerName: g.GetName(),
		StartDate:    config.StartDate,
		EndDate:      config.EndDate,
		Summary: map[string]interface{}{
			"Total PRs":            len(involvedPRs),
			"Total PRs (author)":   len(authoredPRs),
			"Total PRs (involves)": len(involvedPRs),
			"Active organizations": len(orgStats),
			"Active repositories":  len(repoStats),
		},
		Details: map[string]interface{}{
			"authored_prs": authoredPRs,
			"involved_prs": involvedPRs,
			"org_stats":    orgStats,
			"repo_stats":   repoStats,
		},
	}

	g.printResults(result, authoredPRs, involvedPRs, orgStats, repoStats)
	return result, nil
}

func (g *GitHubAnalyzer) searchPRs(query string, startDate, endDate time.Time) ([]PullRequest, error) {
	var allPRs []PullRequest
	page := 1
	perPage := 100

	dateRange := fmt.Sprintf("created:%s..%s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	fullQuery := fmt.Sprintf("%s type:pr %s", query, dateRange)

	fmt.Printf("Searching GitHub with query: %s\n", fullQuery)

	for {
		apiURL := fmt.Sprintf("https://api.github.com/search/issues?q=%s&page=%d&per_page=%d",
			url.QueryEscape(fullQuery), page, perPage)

		fmt.Printf("Making request to GitHub API (page %d)...\n", page)

		body, err := g.client.Get(apiURL, nil)
		if err != nil {
			return nil, err
		}

		var response SearchResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, common.WrapError(err, "failed to parse GitHub response")
		}

		allPRs = append(allPRs, response.Items...)

		if len(response.Items) < perPage {
			break
		}
		page++
	}

	return allPRs, nil
}

func (g *GitHubAnalyzer) extractRepoFromURL(repoURL string) string {
	// Extract repository name from URL like "https://api.github.com/repos/owner/repo"
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		// Return "owner/repo" format
		return fmt.Sprintf("%s/%s", parts[len(parts)-2], parts[len(parts)-1])
	}
	return repoURL
}

func (g *GitHubAnalyzer) extractOrgName(fullName string) string {
	parts := strings.Split(fullName, "/")
	if len(parts) >= 1 {
		return parts[0]
	}
	return fullName
}

func (g *GitHubAnalyzer) extractRepoName(fullName string) string {
	return fullName
}

func (g *GitHubAnalyzer) printResults(result *common.AnalysisResult, authoredPRs, involvedPRs []PullRequest, orgStats, repoStats map[string]struct{ authored, involved int }) {
	fmt.Printf("\nPull Requests from %s to %s:\n",
		result.StartDate.Format("2006-01-02"),
		result.EndDate.Format("2006-01-02"))

	// Print authored PRs
	fmt.Printf("\nPull Requests you authored (%d):\n", len(authoredPRs))
	for _, pr := range authoredPRs {
		fmt.Printf("- %s: %s\n", pr.CreatedAt.Format("2006-01-02 15:04"), pr.Title)
		fmt.Printf("  URL: %s\n", pr.URL)
		fmt.Printf("  Repository: %s\n", g.extractRepoFromURL(pr.RepositoryURL))
		fmt.Println()
	}

	result.PrintSummary()

	// Print organization stats
	fmt.Println("\nPR count per organization (author/involves):")
	type orgStat struct {
		name     string
		authored int
		involved int
	}
	var sortedOrgs []orgStat
	for name, stat := range orgStats {
		sortedOrgs = append(sortedOrgs, orgStat{name, stat.authored, stat.involved})
	}
	sort.Slice(sortedOrgs, func(i, j int) bool {
		return sortedOrgs[i].name < sortedOrgs[j].name
	})
	for _, stat := range sortedOrgs {
		fmt.Printf("- %s: %d (%d)\n", stat.name, stat.authored, stat.involved)
	}

	// Print repository stats
	fmt.Println("\nPR count per repository (author/involves):")
	type repoStat struct {
		name     string
		authored int
		involved int
	}
	var sortedRepos []repoStat
	for name, stat := range repoStats {
		sortedRepos = append(sortedRepos, repoStat{name, stat.authored, stat.involved})
	}
	sort.Slice(sortedRepos, func(i, j int) bool {
		return sortedRepos[i].name < sortedRepos[j].name
	})
	for _, stat := range sortedRepos {
		fmt.Printf("- %s: %d (%d)\n", stat.name, stat.authored, stat.involved)
	}
}
