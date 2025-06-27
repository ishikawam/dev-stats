package backlog

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"

	"dev-stats/pkg/common"
)

// BacklogAnalyzer implements the Analyzer interface for Backlog
type BacklogAnalyzer struct {
	apiKey    string
	spaceName string
	userID    string
	projectID string
	client    *common.HTTPClient
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

// NewBacklogAnalyzer creates a new Backlog analyzer
func NewBacklogAnalyzer() *BacklogAnalyzer {
	return &BacklogAnalyzer{
		apiKey:    os.Getenv("BACKLOG_API_KEY"),
		spaceName: os.Getenv("BACKLOG_SPACE_NAME"),
		userID:    os.Getenv("BACKLOG_USER_ID"),
		projectID: os.Getenv("BACKLOG_PROJECT_ID"),
		client:    common.NewHTTPClient(),
	}
}

// GetName returns the analyzer name
func (b *BacklogAnalyzer) GetName() string {
	return "Backlog"
}

// ValidateConfig validates the required configuration
func (b *BacklogAnalyzer) ValidateConfig() error {
	if b.apiKey == "" {
		return common.NewError("BACKLOG_API_KEY environment variable is required")
	}
	if b.spaceName == "" {
		return common.NewError("BACKLOG_SPACE_NAME environment variable is required")
	}
	if b.userID == "" {
		return common.NewError("BACKLOG_USER_ID environment variable is required")
	}
	if b.projectID == "" {
		return common.NewError("BACKLOG_PROJECT_ID environment variable is required")
	}

	// Test API connectivity with helpful error messages
	fmt.Printf("Testing Backlog API connection to: https://%s.backlog.com\n", b.spaceName)
	testURL := fmt.Sprintf("https://%s.backlog.com/api/v2/space", b.spaceName)
	params := url.Values{}
	params.Set("apiKey", b.apiKey)
	fullURL := fmt.Sprintf("%s?%s", testURL, params.Encode())

	_, err := b.client.Get(fullURL, nil)
	if err != nil {
		return common.WrapError(err, "Failed to connect to Backlog API.\n"+
			"Please verify:\n"+
			"1. BACKLOG_SPACE_NAME is correct (current: %s)\n"+
			"2. BACKLOG_API_KEY is valid\n"+
			"3. Your Backlog URL should be: https://%s.backlog.com\n"+
			"4. API key has proper permissions", b.spaceName, b.spaceName)
	}
	fmt.Printf("✓ Backlog API connection successful\n")

	return nil
}

// Analyze performs Backlog analysis
func (b *BacklogAnalyzer) Analyze(config *common.Config) (*common.AnalysisResult, error) {
	if err := b.ValidateConfig(); err != nil {
		return nil, err
	}

	fmt.Printf("Analyzing Backlog activity for user ID: %s\n", b.userID)
	fmt.Printf("Space: %s, Project ID: %s\n", b.spaceName, b.projectID)
	fmt.Printf("Date range: %s to %s\n", config.StartDate.Format("2006-01-02"), config.EndDate.Format("2006-01-02"))

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
	activityStats := b.analyzeActivities(activities)

	// Create result
	result := &common.AnalysisResult{
		AnalyzerName: b.GetName(),
		StartDate:    config.StartDate,
		EndDate:      config.EndDate,
		Summary: map[string]interface{}{
			"Issues created":   len(createdIssues),
			"Issues assigned":  len(assignedIssues),
			"Total activities": len(activities),
			"Activity types":   len(activityStats),
		},
		Details: map[string]interface{}{
			"created_issues":  createdIssues,
			"assigned_issues": assignedIssues,
			"activities":      activities,
			"activity_stats":  activityStats,
		},
	}

	b.printResults(result, createdIssues, assignedIssues, activityStats)
	return result, nil
}

func (b *BacklogAnalyzer) getIssuesCreatedByUser(startDate, endDate time.Time) ([]Issue, error) {
	params := url.Values{}
	params.Set("apiKey", b.apiKey)
	params.Set("projectId[]", b.projectID)
	params.Set("createdUserId[]", b.userID)
	params.Set("createdSince", startDate.Format("2006-01-02"))
	params.Set("createdUntil", endDate.Format("2006-01-02"))
	params.Set("count", "100")

	apiURL := fmt.Sprintf("https://%s.backlog.com/api/v2/issues?%s", b.spaceName, params.Encode())

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
	params.Set("apiKey", b.apiKey)
	params.Set("projectId[]", b.projectID)
	params.Set("assigneeId[]", b.userID)
	params.Set("createdSince", startDate.Format("2006-01-02"))
	params.Set("createdUntil", endDate.Format("2006-01-02"))
	params.Set("count", "100")

	apiURL := fmt.Sprintf("https://%s.backlog.com/api/v2/issues?%s", b.spaceName, params.Encode())

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

	userIDInt, _ := strconv.Atoi(b.userID)
	requestCount := 0

	for {
		requestCount++
		params := url.Values{}
		params.Set("apiKey", b.apiKey)
		params.Set("count", "100")
		if maxId != "" {
			params.Set("maxId", maxId)
		}

		apiURL := fmt.Sprintf("https://%s.backlog.com/api/v2/users/%d/activities?%s", b.spaceName, userIDInt, params.Encode())

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

func (b *BacklogAnalyzer) analyzeActivities(activities []Activity) map[string]int {
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
		fmt.Println("\nUnknown activity types found:")
		for actType, examples := range unknownTypes {
			fmt.Printf("  Type %d: %v\n", actType, examples)
		}
	}

	return stats
}

func (b *BacklogAnalyzer) printResults(result *common.AnalysisResult, createdIssues, assignedIssues []Issue, activityStats map[string]int) {
	fmt.Printf("\nBacklog activity from %s to %s:\n",
		result.StartDate.Format("2006-01-02"),
		result.EndDate.Format("2006-01-02"))

	fmt.Printf("\nIssues you created (%d):\n", len(createdIssues))
	for _, issue := range createdIssues {
		fmt.Printf("- %s: %s\n", issue.Created.Format("2006-01-02 15:04"), issue.Summary)
		fmt.Printf("  Type: %s\n", issue.IssueType.Name)
		fmt.Printf("  Status: %s\n", issue.Status.Name)
		fmt.Println()
	}

	fmt.Printf("Issues assigned to you (%d):\n", len(assignedIssues))
	for _, issue := range assignedIssues {
		fmt.Printf("- %s: %s\n", issue.Created.Format("2006-01-02 15:04"), issue.Summary)
		fmt.Printf("  Type: %s\n", issue.IssueType.Name)
		fmt.Printf("  Status: %s\n", issue.Status.Name)
		if issue.CreatedUser.ID != 0 {
			fmt.Printf("  Created by: %s\n", issue.CreatedUser.Name)
		}
		fmt.Println()
	}

	result.PrintSummary()

	// Print activity stats
	fmt.Println("\nActivity count by type:")
	type activityStat struct {
		name  string
		count int
	}
	var sortedStats []activityStat
	for name, count := range activityStats {
		sortedStats = append(sortedStats, activityStat{name, count})
	}
	sort.Slice(sortedStats, func(i, j int) bool {
		return sortedStats[i].count > sortedStats[j].count
	})
	for i, stat := range sortedStats {
		fmt.Printf("- %d. %s: %d\n", i+1, stat.name, stat.count)
	}
}
