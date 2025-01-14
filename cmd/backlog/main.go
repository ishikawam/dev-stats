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
	"strconv"
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

type Comment struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	Created struct {
		User struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"createdUser"`
	} `json:"created"`
}

func fetchIssues(baseURL string, query url.Values, token string) []Issue {
	fullURL := fmt.Sprintf("%s/issues?%s&apiKey=%s", baseURL, query.Encode(), token)
	resp, err := http.Get(fullURL)
	if err != nil {
		log.Fatalf("Error fetching issues: %v", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Backlog API returned status: %d\nResponse: %s", resp.StatusCode, body)
	}

	var issues []Issue
	err = json.Unmarshal(body, &issues)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	return issues
}

func fetchComments(baseURL string, issueID int, token string) []Comment {
	commentsURL := fmt.Sprintf("%s/issues/%d/comments?apiKey=%s", baseURL, issueID, token)
	resp, err := http.Get(commentsURL)
	if err != nil {
		log.Fatalf("Error fetching comments for issue %d: %v", issueID, err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Backlog API returned status for comments: %d\nResponse: %s", resp.StatusCode, body)
	}

	var comments []Comment
	_ = json.Unmarshal(body, &comments)
	return comments
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found. Environment variables must be set.")
	}

	// Get environment variables
	apiKey := os.Getenv("BACKLOG_API_KEY")
	userIDStr := os.Getenv("BACKLOG_USER_ID")
	projectID := os.Getenv("BACKLOG_PROJECT_ID")
	startDate := os.Getenv("START_DATE")
	endDate := os.Getenv("END_DATE")

	// Validate required environment variables
	if apiKey == "" || userIDStr == "" || projectID == "" || startDate == "" || endDate == "" {
		log.Fatal("Environment variables BACKLOG_API_KEY, BACKLOG_USER_ID, BACKLOG_PROJECT_ID, START_DATE, and END_DATE must be set.")
	}

	// Convert userID to integer
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		log.Fatalf("Invalid BACKLOG_USER_ID: %v", err)
	}

	// Fetch backlog base URL dynamically
	baseURL := fetchBacklogBaseURL()

	// Fetch issues created by the user
	query := url.Values{}
	query.Set("projectId[]", projectID)
	query.Set("createdUserId[]", userIDStr)
	query.Set("createdSince", startDate)
	query.Set("createdUntil", endDate)
	createdIssues := fetchIssues(baseURL, query, apiKey)
	fmt.Printf("Issues you created: %d\n", len(createdIssues))

	// Fetch issues assigned to the user (including past assignments)
	query.Del("createdUserId[]")
	query.Set("assigneeId[]", userIDStr)
	assignedIssues := fetchIssues(baseURL, query, apiKey)
	fmt.Printf("Issues assigned to you: %d\n", len(assignedIssues))

	// Fetch all comments and count issues where the user commented
	commentedIssueIDs := make(map[int]bool)
	for _, issue := range createdIssues {
		comments := fetchComments(baseURL, issue.ID, apiKey)
		for _, comment := range comments {
			if comment.Created.User.ID == userID {
				commentedIssueIDs[issue.ID] = true
			}
		}
	}
	fmt.Printf("Issues you commented on: %d\n", len(commentedIssueIDs))
}
