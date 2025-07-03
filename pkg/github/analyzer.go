package github

import (
	"encoding/json"
	"fmt"
	"io"
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

// Label represents a GitHub label
type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Title     string    `json:"title"`
	URL       string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	RepositoryURL string  `json:"repository_url"`
	Number        int     `json:"number"`
	Labels        []Label `json:"labels"`
}

// ReviewComment represents a PR review comment
type ReviewComment struct {
	ID        int       `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

// Review represents a PR review
type Review struct {
	ID          int       `json:"id"`
	State       string    `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED
	Body        string    `json:"body"`
	SubmittedAt time.Time `json:"submitted_at"`
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
}

// ReviewStats tracks review activity
type ReviewStats struct {
	ReviewsGiven     int `json:"reviews_given"`
	ApprovalsGiven   int `json:"approvals_given"`
	CommentsGiven    int `json:"comments_given"`
	ChangesRequested int `json:"changes_requested"`
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
func (g *GitHubAnalyzer) Analyze(config *common.Config, writer io.Writer) (*common.AnalysisResult, error) {
	if err := g.ValidateConfig(); err != nil {
		return nil, err
	}

	g.client.SetHeader("Authorization", "token "+g.token)
	g.client.SetHeader("Accept", "application/vnd.github.v3+json")

	fmt.Fprintf(writer, "Analyzing GitHub activity for user: %s\n", g.username)
	fmt.Fprintf(writer, "Date range: %s to %s\n", config.StartDate.Format("2006-01-02"), config.EndDate.Format("2006-01-02"))

	// Get PRs where user is involved
	involvedPRs, err := g.searchPRs(writer, "involves:"+g.username, config.StartDate, config.EndDate)
	if err != nil {
		return nil, common.WrapError(err, "failed to search involved PRs")
	}

	// Get PRs authored by user
	authoredPRs, err := g.searchPRs(writer, "author:"+g.username, config.StartDate, config.EndDate)
	if err != nil {
		return nil, common.WrapError(err, "failed to search authored PRs")
	}

	// Analyze review activity
	fmt.Fprintln(writer, "Analyzing review activity...")
	reviewStats, err := g.analyzeReviewActivity(writer, involvedPRs, config.StartDate, config.EndDate)
	if err != nil {
		fmt.Fprintf(writer, "Warning: Failed to analyze review activity: %v\n", err)
		reviewStats = &ReviewStats{} // Use empty stats if analysis fails
	}

	// Analyze results
	orgStats := make(map[string]struct{ authored, involved int })
	repoStats := make(map[string]struct{ authored, involved int })
	labelStats := make(map[string]int)

	// Categorize PRs by value
	var valuablePRs, lowValuePRs []PullRequest
	for _, pr := range authoredPRs {
		if g.isLowValuePR(pr) {
			lowValuePRs = append(lowValuePRs, pr)
		} else {
			valuablePRs = append(valuablePRs, pr)
		}
	}

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

		// Count labels
		if len(pr.Labels) == 0 {
			labelStats["No labels"]++
		} else {
			for _, label := range pr.Labels {
				labelStats[label.Name]++
			}
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
			"PRs (valuable)":       len(valuablePRs),
			"PRs (low-value)":      len(lowValuePRs),
			"Active organizations": len(orgStats),
			"Active repositories":  len(repoStats),
			"Unique labels":        len(labelStats),
			"Reviews given":        reviewStats.ReviewsGiven,
			"Approvals given":      reviewStats.ApprovalsGiven,
			"Review comments":      reviewStats.CommentsGiven,
			"Changes requested":    reviewStats.ChangesRequested,
		},
		Details: map[string]interface{}{
			"authored_prs":  authoredPRs,
			"involved_prs":  involvedPRs,
			"valuable_prs":  valuablePRs,
			"low_value_prs": lowValuePRs,
			"org_stats":     orgStats,
			"repo_stats":    repoStats,
			"label_stats":   labelStats,
			"review_stats":  reviewStats,
		},
	}

	g.printResults(writer, result, authoredPRs, involvedPRs, valuablePRs, lowValuePRs, orgStats, repoStats, labelStats, reviewStats)
	return result, nil
}

func (g *GitHubAnalyzer) searchPRs(writer io.Writer, query string, startDate, endDate time.Time) ([]PullRequest, error) {
	var allPRs []PullRequest
	page := 1
	perPage := 100

	dateRange := fmt.Sprintf("created:%s..%s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	fullQuery := fmt.Sprintf("%s type:pr %s", query, dateRange)

	fmt.Fprintf(writer, "Searching GitHub with query: %s\n", fullQuery)

	for {
		apiURL := fmt.Sprintf("https://api.github.com/search/issues?q=%s&page=%d&per_page=%d",
			url.QueryEscape(fullQuery), page, perPage)

		fmt.Fprintf(writer, "Making request to GitHub API (page %d)...\n", page)

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

// isLowValuePR determines if a PR is low value based on title patterns
func (g *GitHubAnalyzer) isLowValuePR(pr PullRequest) bool {
	title := strings.ToLower(strings.TrimSpace(pr.Title))

	// Check for back merge patterns
	if strings.Contains(title, "back merge") || strings.Contains(title, "backmerge") {
		return true
	}

	// Define branch names
	branches := []string{"develop", "main", "stg", "staging", "production", "master"}

	// Check for branch to branch patterns like "develop -> main", "main to staging"
	for _, fromBranch := range branches {
		for _, toBranch := range branches {
			if fromBranch != toBranch {
				// Check patterns: "branch -> branch", "branch to branch"
				if strings.Contains(title, fromBranch+" -> "+toBranch) ||
					strings.Contains(title, fromBranch+" to "+toBranch) ||
					strings.Contains(title, fromBranch+"->"+toBranch) {
					return true
				}
			}
		}
	}

	return false
}

func (g *GitHubAnalyzer) printResults(writer io.Writer, result *common.AnalysisResult, authoredPRs, involvedPRs, valuablePRs, lowValuePRs []PullRequest, orgStats, repoStats map[string]struct{ authored, involved int }, labelStats map[string]int, reviewStats *ReviewStats) {
	fmt.Fprintf(writer, "\nPull Requests from %s to %s:\n",
		result.StartDate.Format("2006-01-02"),
		result.EndDate.Format("2006-01-02"))

	// Print valuable PRs
	fmt.Fprintf(writer, "\nValuable Pull Requests you authored (%d):\n", len(valuablePRs))
	for _, pr := range valuablePRs {
		fmt.Fprintf(writer, "- %s: %s\n", pr.CreatedAt.Format("2006-01-02 15:04"), pr.Title)
		fmt.Fprintf(writer, "  URL: %s\n", pr.URL)
		fmt.Fprintf(writer, "  Repository: %s\n", g.extractRepoFromURL(pr.RepositoryURL))

		// Display labels if any
		if len(pr.Labels) > 0 {
			labelNames := make([]string, len(pr.Labels))
			for i, label := range pr.Labels {
				labelNames[i] = label.Name
			}
			fmt.Fprintf(writer, "  Labels: %s\n", strings.Join(labelNames, ", "))
		}

		fmt.Fprintln(writer)
	}

	// Print low-value PRs
	fmt.Fprintf(writer, "Low-value Pull Requests you authored (%d):\n", len(lowValuePRs))
	for _, pr := range lowValuePRs {
		fmt.Fprintf(writer, "- %s: %s\n", pr.CreatedAt.Format("2006-01-02 15:04"), pr.Title)
		fmt.Fprintf(writer, "  URL: %s\n", pr.URL)
		fmt.Fprintf(writer, "  Repository: %s\n", g.extractRepoFromURL(pr.RepositoryURL))

		// Display labels if any
		if len(pr.Labels) > 0 {
			labelNames := make([]string, len(pr.Labels))
			for i, label := range pr.Labels {
				labelNames[i] = label.Name
			}
			fmt.Fprintf(writer, "  Labels: %s\n", strings.Join(labelNames, ", "))
		}

		fmt.Fprintln(writer)
	}

	result.PrintSummary(writer)

	// Print review activity stats
	fmt.Fprintln(writer, "\nReview Activity:")
	fmt.Fprintf(writer, "- Total reviews given: %d\n", reviewStats.ReviewsGiven)
	fmt.Fprintf(writer, "- Approvals given: %d\n", reviewStats.ApprovalsGiven)
	fmt.Fprintf(writer, "- Review comments: %d\n", reviewStats.CommentsGiven)
	fmt.Fprintf(writer, "- Changes requested: %d\n", reviewStats.ChangesRequested)

	// Print organization stats
	fmt.Fprintln(writer, "\nPR count per organization (author/involves):")
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
		fmt.Fprintf(writer, "- %s: %d (%d)\n", stat.name, stat.authored, stat.involved)
	}

	// Print repository stats
	fmt.Fprintln(writer, "\nPR count per repository (author/involves):")
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
		fmt.Fprintf(writer, "- %s: %d (%d)\n", stat.name, stat.authored, stat.involved)
	}

	// Print label stats
	fmt.Fprintln(writer, "\nLabel usage statistics:")
	type labelStat struct {
		name  string
		count int
	}
	var sortedLabels []labelStat
	for name, count := range labelStats {
		sortedLabels = append(sortedLabels, labelStat{name, count})
	}
	sort.Slice(sortedLabels, func(i, j int) bool {
		if sortedLabels[i].count == sortedLabels[j].count {
			return sortedLabels[i].name < sortedLabels[j].name
		}
		return sortedLabels[i].count > sortedLabels[j].count
	})

	if len(sortedLabels) == 0 {
		fmt.Fprintln(writer, "- No labels found in authored PRs")
	} else {
		for _, stat := range sortedLabels {
			fmt.Fprintf(writer, "- %s: %d\n", stat.name, stat.count)
		}
	}
}

// analyzeReviewActivity analyzes the user's review activity on PRs
func (g *GitHubAnalyzer) analyzeReviewActivity(writer io.Writer, involvedPRs []PullRequest, startDate, endDate time.Time) (*ReviewStats, error) {
	stats := &ReviewStats{}

	// Track unique repositories to avoid rate limiting
	repoMap := make(map[string]bool)
	for _, pr := range involvedPRs {
		repoFullName := g.extractRepoFromURL(pr.RepositoryURL)
		repoMap[repoFullName] = true
	}

	fmt.Fprintf(writer, "Analyzing reviews across %d repositories...\n", len(repoMap))

	// Analyze each repository
	for repoFullName := range repoMap {
		repoStats, err := g.getReviewStatsForRepo(writer, repoFullName, startDate, endDate)
		if err != nil {
			fmt.Fprintf(writer, "Warning: Failed to get review stats for %s: %v\n", repoFullName, err)
			continue
		}

		stats.ReviewsGiven += repoStats.ReviewsGiven
		stats.ApprovalsGiven += repoStats.ApprovalsGiven
		stats.CommentsGiven += repoStats.CommentsGiven
		stats.ChangesRequested += repoStats.ChangesRequested
	}

	return stats, nil
}

// getReviewStatsForRepo gets review statistics for a specific repository
func (g *GitHubAnalyzer) getReviewStatsForRepo(writer io.Writer, repoFullName string, startDate, endDate time.Time) (*ReviewStats, error) {
	stats := &ReviewStats{}

	// Search for PRs in this repo within date range that the user reviewed
	query := fmt.Sprintf("repo:%s type:pr reviewed-by:%s created:%s..%s",
		repoFullName, g.username, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	apiURL := fmt.Sprintf("https://api.github.com/search/issues?q=%s&per_page=100",
		url.QueryEscape(query))

	body, err := g.client.Get(apiURL, nil)
	if err != nil {
		return stats, err
	}

	var response SearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return stats, common.WrapError(err, "failed to parse review search response")
	}

	// For each PR, get detailed review information
	for _, pr := range response.Items {
		reviewsURL := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d/reviews",
			repoFullName, pr.Number)

		reviewBody, err := g.client.Get(reviewsURL, nil)
		if err != nil {
			fmt.Fprintf(writer, "Warning: Failed to get reviews for PR #%d: %v\n", pr.Number, err)
			continue
		}

		var reviews []Review
		if err := json.Unmarshal(reviewBody, &reviews); err != nil {
			fmt.Fprintf(writer, "Warning: Failed to parse reviews for PR #%d: %v\n", pr.Number, err)
			continue
		}

		// Count reviews by this user within date range
		for _, review := range reviews {
			if review.User.Login == g.username &&
				review.SubmittedAt.After(startDate.Add(-24*time.Hour)) &&
				review.SubmittedAt.Before(endDate.Add(24*time.Hour)) {
				stats.ReviewsGiven++

				switch review.State {
				case "APPROVED":
					stats.ApprovalsGiven++
				case "CHANGES_REQUESTED":
					stats.ChangesRequested++
				case "COMMENTED":
					stats.CommentsGiven++
				}
			}
		}
	}

	return stats, nil
}
