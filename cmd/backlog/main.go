package main

import (
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"
)

func fetchBacklogBaseURL() string {
	spaceName := os.Getenv("BACKLOG_SPACE_NAME")
	if spaceName == "" {
		log.Fatal("Environment variable BACKLOG_SPACE_NAME is not set")
	}
	return fmt.Sprintf("https://%s.backlog.com/api/v2", spaceName)
}

type Issue struct {
	ID       int    `json:"id"`
	Title    string `json:"summary"`
	Assignee struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"assignee"`
	CreatedUser struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"createdUser"`
	Status struct {
		Name string `json:"name"`
	} `json:"status"`
}

type Activity struct {
	ID      int       `json:"id"`
	Created time.Time `json:"created"`
	Project struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
	Type    int `json:"type"`
	Content struct {
		ID int `json:"id"`
	} `json:"content"`
}

func fetchIssues(baseURL string, query url.Values, token string) []Issue {
	fullURL := fmt.Sprintf("%s/issues?%s&apiKey=%s", baseURL, query.Encode(), token)
	resp, err := http.Get(fullURL)
	if err != nil {
		log.Fatalf("Error fetching issues: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Fatalf("Backlog API returned status: %d\nResponse: %s", resp.StatusCode, body)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	var issues []Issue
	_ = json.Unmarshal(body, &issues)
	return issues
}

func fetchActivities(baseURL string, userID int, token string, minID int) []Activity {
	query := url.Values{}
	if minID > 0 {
		query.Set("maxId", strconv.Itoa(minID))
	}
	query.Set("count", "100")

	fullURL := fmt.Sprintf("%s/users/%d/activities?%s&apiKey=%s", baseURL, userID, query.Encode(), token)
	resp, err := http.Get(fullURL)
	if err != nil {
		log.Fatalf("Error fetching activities: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Fatalf("Backlog API returned status %d: %s", resp.StatusCode, body)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	var activities []Activity
	_ = json.Unmarshal(body, &activities)
	return activities
}

func displayActivitySummary(activityCount map[int]int, uniqueIssuesByType map[int]map[int]struct{}, uniqueIssues map[int]struct{}) {
	typeDescriptions := map[int]string{
		1: "Issue created",
		2: "Comment added",
		3: "Status changed",
		// 必要に応じて他のタイプを追加
	}

	// ソートのためのキーを抽出
	var activityTypes []int
	for activityType := range activityCount {
		activityTypes = append(activityTypes, activityType)
	}
	sort.Ints(activityTypes)

	// 表示
	fmt.Println("\nActivity count by type (count issues):")
	for _, activityType := range activityTypes {
		description := typeDescriptions[activityType]
		if description == "" {
			description = fmt.Sprintf("Unknown activity (type %d)", activityType)
		}
		uniqueIssuesCount := len(uniqueIssuesByType[activityType])
		fmt.Printf("- %d. %s: %d (%d)\n", activityType, description, activityCount[activityType], uniqueIssuesCount)
	}

	// 総チケット数
	fmt.Printf("\nYour issues count: %d\n", len(uniqueIssues))
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found. Environment variables must be set.")
	}

	apiKey := os.Getenv("BACKLOG_API_KEY")
	spaceName := os.Getenv("BACKLOG_SPACE_NAME")
	userIDStr := os.Getenv("BACKLOG_USER_ID")
	projectID := os.Getenv("BACKLOG_PROJECT_ID")
	startDateStr := os.Getenv("START_DATE")
	endDateStr := os.Getenv("END_DATE")

	if apiKey == "" || spaceName == "" || userIDStr == "" || projectID == "" || startDateStr == "" || endDateStr == "" {
		log.Fatal("Required environment variables are missing.")
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		log.Fatalf("Invalid BACKLOG_USER_ID: %v", err)
	}

	baseURL := fetchBacklogBaseURL()

	startDate, _ := time.Parse("2006-01-02", startDateStr)
	endDate, _ := time.Parse("2006-01-02", endDateStr)

	// Fetch created issues
	query := url.Values{}
	query.Set("projectId[]", projectID)
	query.Set("createdUserId[]", userIDStr)
	query.Set("createdSince", startDateStr)
	query.Set("createdUntil", endDateStr)

	createdIssues := fetchIssues(baseURL, query, apiKey)
	fmt.Printf("Issues you created: %d\n", len(createdIssues))

	// Fetch assigned issues
	query.Del("createdUserId[]")
	query.Set("assigneeId[]", userIDStr)
	assignedIssues := fetchIssues(baseURL, query, apiKey)
	fmt.Printf("Issues assigned to you: %d\n", len(assignedIssues))

	// Fetch activities
	var minID int
	activityCount := make(map[int]int)
	uniqueIssuesByType := make(map[int]map[int]struct{})
	uniqueIssues := make(map[int]struct{})
	for {
		activities := fetchActivities(baseURL, userID, apiKey, minID)
		if len(activities) == 0 {
			break
		}
		for _, activity := range activities {
			if activity.Created.After(startDate) && activity.Created.Before(endDate) {
				activityCount[activity.Type]++
				// 一意の issues をタイプごとにカウント
				if uniqueIssuesByType[activity.Type] == nil {
					uniqueIssuesByType[activity.Type] = make(map[int]struct{})
				}
				uniqueIssuesByType[activity.Type][activity.Content.ID] = struct{}{}
				// 全体の一意の issues
				uniqueIssues[activity.Content.ID] = struct{}{}
			}
			if minID == 0 || activity.ID < minID {
				minID = activity.ID
			}
		}
	}
	displayActivitySummary(activityCount, uniqueIssuesByType, uniqueIssues)
}
