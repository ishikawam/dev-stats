package backlog

import (
	"dev-stats/pkg/common"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"
)

// BacklogAnalyzer implements the Analyzer interface for Backlog
type BacklogAnalyzer struct {
	profile *BacklogProfile
	client  *common.HTTPClient
}

// Issue represents a Backlog issue
type Issue struct {
	ID          int       `json:"id"`
	Summary     string    `json:"summary"`
	Created     time.Time `json:"created"`
	Assignee    *User     `json:"assignee"`
	CreatedUser User      `json:"createdUser"`
	IssueType   IssueType `json:"issueType"`
	Status      Status    `json:"status"`
}

// User represents a Backlog user
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// IssueType represents a Backlog issue type
type IssueType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Status represents a Backlog status
type Status struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Activity represents a Backlog activity
type Activity struct {
	ID      int                    `json:"id"`
	Type    int                    `json:"type"`
	Content map[string]interface{} `json:"content"`
	Created time.Time              `json:"created"`
}

// ActivityItem represents a simplified activity item for listing
type ActivityItem struct {
	ID      int
	Title   string
	Created time.Time
	Type    string
}

// NewBacklogAnalyzer creates a new Backlog analyzer (legacy method for backward compatibility)
func NewBacklogAnalyzer() *BacklogAnalyzer {
	// For backward compatibility, check old environment variables first
	if os.Getenv("BACKLOG_API_KEY") != "" {
		profile := &BacklogProfile{
			Name:      "default",
			APIKey:    os.Getenv("BACKLOG_API_KEY"),
			Host:      os.Getenv("BACKLOG_HOST"),
			UserID:    os.Getenv("BACKLOG_USER_ID"),
			ProjectID: os.Getenv("BACKLOG_PROJECT_ID"),
		}
		return &BacklogAnalyzer{
			profile: profile,
			client:  common.NewHTTPClient(),
		}
	}
	return nil
}

// NewBacklogAnalyzerWithProfile creates a new Backlog analyzer with a specific profile
func NewBacklogAnalyzerWithProfile(profile *BacklogProfile) *BacklogAnalyzer {
	return &BacklogAnalyzer{
		profile: profile,
		client:  common.NewHTTPClient(),
	}
}

// GetName returns the analyzer name
func (b *BacklogAnalyzer) GetName() string {
	return "Backlog"
}

// ValidateConfig validates the required configuration
func (b *BacklogAnalyzer) ValidateConfig(writer io.Writer) error {
	if b.profile.APIKey == "" {
		return common.NewError("BACKLOG_API_KEY environment variable is required")
	}
	if b.profile.Host == "" {
		return common.NewError("BACKLOG_HOST environment variable is required")
	}
	if b.profile.UserID == "" {
		return common.NewError("BACKLOG_USER_ID environment variable is required")
	}
	if b.profile.ProjectID == "" {
		return common.NewError("BACKLOG_PROJECT_ID environment variable is required")
	}

	// Test API connectivity with helpful error messages
	baseURL := b.profile.GetBaseURL()
	fmt.Fprintf(writer, "Testing Backlog API connection to: %s\n", baseURL)
	testURL := fmt.Sprintf("%s/api/v2/space", baseURL)
	params := url.Values{}
	params.Set("apiKey", b.profile.APIKey)
	fullURL := fmt.Sprintf("%s?%s", testURL, params.Encode())

	_, err := b.client.Get(fullURL, nil)
	if err != nil {
		return common.WrapError(err, "Failed to connect to Backlog API.\n"+
			"Please verify:\n"+
			"1. BACKLOG_HOST is correct (current: %s)\n"+
			"2. BACKLOG_API_KEY is valid\n"+
			"3. Your Backlog URL should be: %s\n"+
			"4. API key has proper permissions", b.profile.Host, baseURL)
	}
	fmt.Fprintf(writer, "✓ Backlog API connection successful\n")

	return nil
}

// Analyze performs Backlog analysis
func (b *BacklogAnalyzer) Analyze(config *common.Config, writer io.Writer) (*common.AnalysisResult, error) {
	if err := b.ValidateConfig(writer); err != nil {
		return nil, err
	}

	fmt.Fprintf(writer, "Analyzing Backlog activity for user ID: %s\n", b.profile.UserID)
	fmt.Fprintf(writer, "Host: %s, Project ID: %s\n", b.profile.Host, b.profile.ProjectID)
	fmt.Fprintf(writer, "Date range: %s to %s\n", config.StartDate.Format("2006-01-02"), config.EndDate.Format("2006-01-02"))

	// Get issues created by user
	createdIssues, err := b.getIssuesCreatedByUser(config.StartDate, config.EndDate)
	if err != nil {
		return nil, common.WrapError(err, "failed to get created issues")
	}

	// Get issues assigned to user
	assignedIssues, err := b.getIssuesAssignedToUser(config.StartDate, config.EndDate)
	if err != nil {
		return nil, common.WrapError(err, "failed to get assigned issues")
	}

	// Get user activities
	activities, err := b.getUserActivities(config.StartDate, config.EndDate)
	if err != nil {
		return nil, common.WrapError(err, "failed to get user activities")
	}

	// Analyze activities
	activityStats := b.analyzeActivities(writer, activities)

	// Extract detailed activity lists
	commentedIssues := b.extractCommentedIssues(activities)
	updatedIssues := b.extractUpdatedIssues(activities)
	updatedWikis := b.extractUpdatedWikis(activities)
	createdWikis := b.extractCreatedWikis(activities)

	// Create result
	result := &common.AnalysisResult{
		AnalyzerName: b.GetName(),
		StartDate:    config.StartDate,
		EndDate:      config.EndDate,
		Summary: map[string]interface{}{
			"Issues created":   len(createdIssues),
			"Issues assigned":  len(assignedIssues),
			"Issues commented": len(commentedIssues),
			"Issues updated":   len(updatedIssues),
			"Wikis created":    len(createdWikis),
			"Wikis updated":    len(updatedWikis),
			"Total activities": len(activities),
			"Activity types":   len(activityStats),
		},
		Details: map[string]interface{}{
			"created_issues":   createdIssues,
			"assigned_issues":  assignedIssues,
			"commented_issues": commentedIssues,
			"updated_issues":   updatedIssues,
			"created_wikis":    createdWikis,
			"updated_wikis":    updatedWikis,
			"activities":       activities,
			"activity_stats":   activityStats,
		},
	}

	b.printResults(writer, result, createdIssues, assignedIssues, commentedIssues, updatedIssues, createdWikis, updatedWikis, activityStats)
	return result, nil
}

func (b *BacklogAnalyzer) getIssuesCreatedByUser(startDate, endDate time.Time) ([]Issue, error) {
	params := url.Values{}
	params.Set("apiKey", b.profile.APIKey)
	params.Set("projectId[]", b.profile.ProjectID)
	params.Set("createdUserId[]", b.profile.UserID)
	params.Set("createdSince", startDate.Format("2006-01-02"))
	params.Set("createdUntil", endDate.Format("2006-01-02"))
	params.Set("count", "100")

	apiURL := fmt.Sprintf("%s/api/v2/issues?%s", b.profile.GetBaseURL(), params.Encode())

	body, err := b.client.Get(apiURL, nil)
	if err != nil {
		return nil, err
	}

	var issues []Issue
	if err := json.Unmarshal(body, &issues); err != nil {
		return nil, common.WrapError(err, "failed to parse Backlog issues response")
	}

	return issues, nil
}

func (b *BacklogAnalyzer) getIssuesAssignedToUser(startDate, endDate time.Time) ([]Issue, error) {
	params := url.Values{}
	params.Set("apiKey", b.profile.APIKey)
	params.Set("projectId[]", b.profile.ProjectID)
	params.Set("assigneeId[]", b.profile.UserID)
	params.Set("createdSince", startDate.Format("2006-01-02"))
	params.Set("createdUntil", endDate.Format("2006-01-02"))
	params.Set("count", "100")

	apiURL := fmt.Sprintf("%s/api/v2/issues?%s", b.profile.GetBaseURL(), params.Encode())

	body, err := b.client.Get(apiURL, nil)
	if err != nil {
		return nil, err
	}

	var issues []Issue
	if err := json.Unmarshal(body, &issues); err != nil {
		return nil, common.WrapError(err, "failed to parse Backlog issues response")
	}

	return issues, nil
}

func (b *BacklogAnalyzer) getUserActivities(startDate, endDate time.Time) ([]Activity, error) {
	var allActivities []Activity
	maxId := ""

	userIDInt, _ := strconv.Atoi(b.profile.UserID)
	requestCount := 0

	for {
		requestCount++
		params := url.Values{}
		params.Set("apiKey", b.profile.APIKey)
		params.Set("count", "100")
		if maxId != "" {
			params.Set("maxId", maxId)
		}

		apiURL := fmt.Sprintf("%s/api/v2/users/%d/activities?%s", b.profile.GetBaseURL(), userIDInt, params.Encode())

		body, err := b.client.Get(apiURL, nil)
		if err != nil {
			return nil, err
		}

		var activities []Activity
		if err := json.Unmarshal(body, &activities); err != nil {
			return nil, common.WrapError(err, "failed to parse Backlog activities response")
		}

		if len(activities) == 0 {
			break
		}

		// Filter activities by date range
		var filteredActivities []Activity
		for _, activity := range activities {
			if activity.Created.After(startDate) && activity.Created.Before(endDate.AddDate(0, 0, 1)) {
				filteredActivities = append(filteredActivities, activity)
			}
		}

		allActivities = append(allActivities, filteredActivities...)

		// If the oldest activity is before our start date, we can stop
		oldestActivity := activities[len(activities)-1]
		if oldestActivity.Created.Before(startDate) {
			break
		}

		maxId = strconv.Itoa(oldestActivity.ID)

		if len(activities) < 100 {
			break
		}
	}

	return allActivities, nil
}

func (b *BacklogAnalyzer) analyzeActivities(writer io.Writer, activities []Activity) map[string]int {
	// Activity types based on official Backlog API documentation
	// https://developer.nulab.com/docs/backlog/api/2/get-activity/
	activityTypes := map[int]string{
		1:  "Issue Created",
		2:  "Issue Updated",
		3:  "Issue Commented",
		4:  "Issue Deleted",
		5:  "Wiki Created",
		6:  "Wiki Updated",
		7:  "Wiki Deleted",
		8:  "File Added",
		9:  "File Updated",
		10: "File Deleted",
		11: "SVN Committed",
		12: "Git Pushed",
		13: "Git Repository Created",
		14: "Issue Multi Updated",
		15: "Project User Added",
		16: "Project User Deleted",
		17: "Comment Notification Added",
		18: "Pull Request Added",
		19: "Pull Request Updated",
		20: "Comment Added on Pull Request",
		21: "Pull Request Deleted",
		22: "Milestone Created",
		23: "Milestone Updated",
		24: "Milestone Deleted",
		25: "Project Group Added",
		26: "Project Group Deleted",
	}

	stats := make(map[string]int)
	unknownTypes := make(map[int][]string) // Track unknown types with examples

	for _, activity := range activities {
		if typeName, exists := activityTypes[activity.Type]; exists {
			stats[typeName]++
		} else {
			typeName := fmt.Sprintf("Activity type %d", activity.Type)
			stats[typeName]++

			// Collect example content for unknown types
			if len(unknownTypes[activity.Type]) < 3 { // Keep max 3 examples
				var example string
				if summary, ok := activity.Content["summary"].(string); ok && summary != "" {
					example = summary
				} else {
					example = fmt.Sprintf("ID: %v", activity.Content["id"])
				}
				unknownTypes[activity.Type] = append(unknownTypes[activity.Type], example)
			}
		}
	}

	// Print unknown activity types with examples for debugging
	if len(unknownTypes) > 0 {
		fmt.Fprintln(writer, "\nUnknown activity types found:")
		for actType, examples := range unknownTypes {
			fmt.Fprintf(writer, "  Type %d: %v\n", actType, examples)
		}
	}

	return stats
}

func (b *BacklogAnalyzer) extractCommentedIssues(activities []Activity) []ActivityItem {
	var items []ActivityItem
	seen := make(map[int]bool)

	for _, activity := range activities {
		if activity.Type == 3 {
			if content, ok := activity.Content["summary"].(string); ok {
				if id, ok := activity.Content["id"].(float64); ok {
					itemID := int(id)
					if !seen[itemID] {
						items = append(items, ActivityItem{
							ID:      itemID,
							Title:   content,
							Created: activity.Created,
							Type:    "Comment",
						})
						seen[itemID] = true
					}
				}
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Created.After(items[j].Created)
	})

	return items
}

func (b *BacklogAnalyzer) extractUpdatedIssues(activities []Activity) []ActivityItem {
	var items []ActivityItem
	seen := make(map[int]bool)

	for _, activity := range activities {
		if activity.Type == 2 || activity.Type == 14 {
			if content, ok := activity.Content["summary"].(string); ok {
				if id, ok := activity.Content["id"].(float64); ok {
					itemID := int(id)
					if !seen[itemID] {
						items = append(items, ActivityItem{
							ID:      itemID,
							Title:   content,
							Created: activity.Created,
							Type:    "Update",
						})
						seen[itemID] = true
					}
				}
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Created.After(items[j].Created)
	})

	return items
}

func (b *BacklogAnalyzer) extractUpdatedWikis(activities []Activity) []ActivityItem {
	var items []ActivityItem
	seen := make(map[int]bool)

	for _, activity := range activities {
		if activity.Type == 6 {
			if content, ok := activity.Content["name"].(string); ok {
				if id, ok := activity.Content["id"].(float64); ok {
					itemID := int(id)
					if !seen[itemID] {
						items = append(items, ActivityItem{
							ID:      itemID,
							Title:   content,
							Created: activity.Created,
							Type:    "Wiki Update",
						})
						seen[itemID] = true
					}
				}
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Created.After(items[j].Created)
	})

	return items
}

func (b *BacklogAnalyzer) extractCreatedWikis(activities []Activity) []ActivityItem {
	var items []ActivityItem
	seen := make(map[int]bool)

	for _, activity := range activities {
		if activity.Type == 5 {
			if content, ok := activity.Content["name"].(string); ok {
				if id, ok := activity.Content["id"].(float64); ok {
					itemID := int(id)
					if !seen[itemID] {
						items = append(items, ActivityItem{
							ID:      itemID,
							Title:   content,
							Created: activity.Created,
							Type:    "Wiki Creation",
						})
						seen[itemID] = true
					}
				}
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Created.After(items[j].Created)
	})

	return items
}

func (b *BacklogAnalyzer) printResults(writer io.Writer, result *common.AnalysisResult, createdIssues, assignedIssues []Issue, commentedIssues, updatedIssues, createdWikis, updatedWikis []ActivityItem, activityStats map[string]int) {
	fmt.Fprintf(writer, "\nBacklog activity from %s to %s:\n",
		result.StartDate.Format("2006-01-02"),
		result.EndDate.Format("2006-01-02"))

	fmt.Fprintf(writer, "\nIssues you created (%d):\n", len(createdIssues))
	for _, issue := range createdIssues {
		fmt.Fprintf(writer, "- %s: %s\n", issue.Created.Format("2006-01-02 15:04"), issue.Summary)
		fmt.Fprintf(writer, "  Type: %s\n", issue.IssueType.Name)
		fmt.Fprintf(writer, "  Status: %s\n", issue.Status.Name)
		fmt.Fprintln(writer)
	}

	fmt.Fprintf(writer, "Issues assigned to you (%d):\n", len(assignedIssues))
	for _, issue := range assignedIssues {
		fmt.Fprintf(writer, "- %s: %s\n", issue.Created.Format("2006-01-02 15:04"), issue.Summary)
		fmt.Fprintf(writer, "  Type: %s\n", issue.IssueType.Name)
		fmt.Fprintf(writer, "  Status: %s\n", issue.Status.Name)
		if issue.CreatedUser.ID != 0 {
			fmt.Fprintf(writer, "  Created by: %s\n", issue.CreatedUser.Name)
		}
		fmt.Fprintln(writer)
	}

	fmt.Fprintf(writer, "Issues you commented on (%d):\n", len(commentedIssues))
	for _, item := range commentedIssues {
		fmt.Fprintf(writer, "- %s: %s\n", item.Created.Format("2006-01-02 15:04"), item.Title)
		fmt.Fprintf(writer, "  Type: %s\n", item.Type)
		fmt.Fprintln(writer)
	}

	fmt.Fprintf(writer, "Issues you updated (%d):\n", len(updatedIssues))
	for _, item := range updatedIssues {
		fmt.Fprintf(writer, "- %s: %s\n", item.Created.Format("2006-01-02 15:04"), item.Title)
		fmt.Fprintf(writer, "  Type: %s\n", item.Type)
		fmt.Fprintln(writer)
	}

	fmt.Fprintf(writer, "Wikis you created (%d):\n", len(createdWikis))
	for _, item := range createdWikis {
		fmt.Fprintf(writer, "- %s: %s\n", item.Created.Format("2006-01-02 15:04"), item.Title)
		fmt.Fprintf(writer, "  Type: %s\n", item.Type)
		fmt.Fprintln(writer)
	}

	fmt.Fprintf(writer, "Wikis you updated (%d):\n", len(updatedWikis))
	for _, item := range updatedWikis {
		fmt.Fprintf(writer, "- %s: %s\n", item.Created.Format("2006-01-02 15:04"), item.Title)
		fmt.Fprintf(writer, "  Type: %s\n", item.Type)
		fmt.Fprintln(writer)
	}

	result.PrintSummary(writer)

	// Print activity stats
	fmt.Fprintln(writer, "\nActivity count by type:")
	type activityStat struct {
		name  string
		count int
	}
	var sortedStats []activityStat
	for name, count := range activityStats {
		sortedStats = append(sortedStats, activityStat{name, count})
	}
	sort.Slice(sortedStats, func(i, j int) bool {
		if sortedStats[i].count == sortedStats[j].count {
			return sortedStats[i].name < sortedStats[j].name
		}
		return sortedStats[i].count > sortedStats[j].count
	})
	for i, stat := range sortedStats {
		fmt.Fprintf(writer, "- %d. %s: %d\n", i+1, stat.name, stat.count)
	}
}
