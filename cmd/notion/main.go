package main

import (
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	notionAPIURL = "https://api.notion.com/v1"
	apiVersion   = "2022-06-28"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Page struct {
	ID             string                 `json:"id"`
	CreatedTime    time.Time              `json:"created_time"`
	LastEditedTime time.Time              `json:"last_edited_time"`
	CreatedBy      User                   `json:"created_by"`
	LastEditedBy   User                   `json:"last_edited_by"`
	Properties     map[string]interface{} `json:"properties"`
	URL            string                 `json:"url"`
	Object         string                 `json:"object"`
	Title          string                 // Extracted from properties
	DatabaseTitle  string                 // Database name if page is in database
}

type SearchResponse struct {
	Results    []json.RawMessage `json:"results"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor"`
}

type Database struct {
	ID    string `json:"id"`
	Title []struct {
		PlainText string `json:"plain_text"`
	} `json:"title"`
}

func makeNotionRequest(url string, token string, body string) ([]byte, error) {
	var req *http.Request
	var err error
	
	if body != "" {
		req, err = http.NewRequest("POST", url, strings.NewReader(body))
	} else {
		req, err = http.NewRequest("GET", url, nil)
	}
	
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Notion-Version", apiVersion)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Notion API returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}

func getCurrentUser(token string) (*User, error) {
	url := fmt.Sprintf("%s/users/me", notionAPIURL)
	body, err := makeNotionRequest(url, token, "")
	if err != nil {
		return nil, err
	}

	var user User
	err = json.Unmarshal(body, &user)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling user: %v", err)
	}

	return &user, nil
}

func detectActualUserID(token string) string {
	// Search for a small sample of pages to detect the actual user ID
	requestBody := `{
		"sort": {
			"direction": "descending",
			"timestamp": "last_edited_time"
		},
		"page_size": 10
	}`

	url := fmt.Sprintf("%s/search", notionAPIURL)
	body, err := makeNotionRequest(url, token, requestBody)
	if err != nil {
		log.Printf("Warning: Failed to auto-detect user ID: %v", err)
		return ""
	}

	var response SearchResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Printf("Warning: Failed to parse search response for auto-detection: %v", err)
		return ""
	}

	// Count user IDs from the sample
	userIDCounts := make(map[string]int)
	
	for _, result := range response.Results {
		// Check if this is a page object
		var objType struct {
			Object string `json:"object"`
		}
		if err := json.Unmarshal(result, &objType); err != nil {
			continue
		}
		
		if objType.Object != "page" {
			continue
		}
		
		// Parse as page
		var page Page
		if err := json.Unmarshal(result, &page); err != nil {
			continue
		}
		
		// Count created_by user IDs
		if page.CreatedBy.ID != "" {
			userIDCounts[page.CreatedBy.ID]++
		}
		
		// Count last_edited_by user IDs
		if page.LastEditedBy.ID != "" {
			userIDCounts[page.LastEditedBy.ID]++
		}
	}
	
	// Find the most common user ID (likely to be the workspace owner/main user)
	var mostCommonUserID string
	maxCount := 0
	
	for userID, count := range userIDCounts {
		if count > maxCount {
			maxCount = count
			mostCommonUserID = userID
		}
	}
	
	return mostCommonUserID
}

func getDatabase(databaseID string, token string) (*Database, error) {
	url := fmt.Sprintf("%s/databases/%s", notionAPIURL, databaseID)
	body, err := makeNotionRequest(url, token, "")
	if err != nil {
		return nil, err
	}

	var database Database
	err = json.Unmarshal(body, &database)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling database: %v", err)
	}

	return &database, nil
}

func extractPageTitle(page Page) string {
	// Try to extract title from properties
	for key, value := range page.Properties {
		if strings.ToLower(key) == "title" || strings.ToLower(key) == "name" {
			if prop, ok := value.(map[string]interface{}); ok {
				if titleArray, ok := prop["title"].([]interface{}); ok && len(titleArray) > 0 {
					if titleObj, ok := titleArray[0].(map[string]interface{}); ok {
						if plainText, ok := titleObj["plain_text"].(string); ok {
							return plainText
						}
					}
				}
			}
		}
	}
	
	// Fallback to page ID if no title found
	return fmt.Sprintf("Page %s", page.ID[:8])
}

func searchPages(token string, userID string, startDate, endDate time.Time) ([]Page, error) {
	var allPages []Page
	var cursor string

	for {
		// Use simple search without user filtering - we'll filter client-side
		requestBody := `{
			"sort": {
				"direction": "descending",
				"timestamp": "last_edited_time"
			}`

		if cursor != "" {
			requestBody += fmt.Sprintf(`,
			"start_cursor": "%s"`, cursor)
		}
		
		requestBody += ",\n\"page_size\": 100\n}"

		url := fmt.Sprintf("%s/search", notionAPIURL)
		body, err := makeNotionRequest(url, token, requestBody)
		if err != nil {
			return nil, err
		}

		var response SearchResponse
		err = json.Unmarshal(body, &response)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling search response: %v", err)
		}
		
		// Filter pages by user and date range
		for _, result := range response.Results {
			// First check if this is a page object
			var objType struct {
				Object string `json:"object"`
			}
			if err := json.Unmarshal(result, &objType); err != nil {
				continue
			}
			
			if objType.Object != "page" {
				continue // Skip non-page objects
			}
			
			// Parse as page
			var page Page
			if err := json.Unmarshal(result, &page); err != nil {
				log.Printf("Warning: failed to parse page: %v", err)
				continue
			}
			
			// Check if user created or edited this page
			isUserInvolved := (page.CreatedBy.ID == userID) || (page.LastEditedBy.ID == userID)
			
			// Check if activity happened in date range
			inDateRange := (page.CreatedTime.After(startDate) && page.CreatedTime.Before(endDate.AddDate(0, 0, 1))) ||
						 (page.LastEditedTime.After(startDate) && page.LastEditedTime.Before(endDate.AddDate(0, 0, 1)))
			
			if isUserInvolved && inDateRange {
				page.Title = extractPageTitle(page)
				allPages = append(allPages, page)
			}
		}

		if !response.HasMore {
			break
		}
		cursor = response.NextCursor
	}

	return allPages, nil
}

func categorizePages(pages []Page, userID string) (created []Page, updated []Page) {
	for _, page := range pages {
		if page.CreatedBy.ID == userID {
			created = append(created, page)
		}
		if page.LastEditedBy.ID == userID && page.CreatedBy.ID != userID {
			updated = append(updated, page)
		}
	}
	return created, updated
}

func main() {
	godotenv.Load()

	token := os.Getenv("NOTION_TOKEN")
	startDateStr := os.Getenv("START_DATE")
	endDateStr := os.Getenv("END_DATE")

	if token == "" || startDateStr == "" || endDateStr == "" {
		log.Fatalf("Environment variables NOTION_TOKEN, START_DATE, and END_DATE must be set.")
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		log.Fatalf("Invalid START_DATE format: %v", err)
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		log.Fatalf("Invalid END_DATE format: %v", err)
	}

	// Get current user
	currentUser, err := getCurrentUser(token)
	if err != nil {
		log.Fatalf("Error getting current user: %v", err)
	}

	fmt.Printf("Analyzing Notion activity for user: %s (ID: %s)\n", currentUser.Name, currentUser.ID)
	fmt.Printf("Note: This shows activity where you are the creator or last editor.\n")

	// Auto-detect the actual user ID by sampling some pages
	fmt.Println("Auto-detecting user ID from workspace pages...")
	detectedUserID := detectActualUserID(token)
	var targetUserID string
	
	if detectedUserID != "" && detectedUserID != currentUser.ID {
		fmt.Printf("Detected workspace user ID: %s (different from Integration Token user: %s)\n", detectedUserID, currentUser.ID)
		targetUserID = detectedUserID
	} else {
		fmt.Printf("Using Integration Token user ID: %s\n", currentUser.ID)
		targetUserID = currentUser.ID
	}

	// Search for pages
	fmt.Println("Searching for pages...")
	pages, err := searchPages(token, targetUserID, startDate, endDate)
	if err != nil {
		log.Fatalf("Error searching pages: %v", err)
	}

	userIDDisplay := targetUserID
	if len(targetUserID) > 8 {
		userIDDisplay = targetUserID[:8]
	}
	fmt.Printf("Found %d pages where user %s was involved\n", len(pages), userIDDisplay)

	// Categorize pages
	createdPages, updatedPages := categorizePages(pages, targetUserID)

	// Sort pages by time
	sort.Slice(createdPages, func(i, j int) bool {
		return createdPages[i].CreatedTime.Before(createdPages[j].CreatedTime)
	})
	sort.Slice(updatedPages, func(i, j int) bool {
		return updatedPages[i].LastEditedTime.Before(updatedPages[j].LastEditedTime)
	})

	// Output detailed results
	fmt.Printf("\nNotion activity from %s to %s:\n", startDateStr, endDateStr)

	fmt.Printf("\nPages you created (%d):\n", len(createdPages))
	for _, page := range createdPages {
		fmt.Printf("- %s: %s\n", page.CreatedTime.Format("2006-01-02 15:04"), page.Title)
		fmt.Printf("  URL: %s\n", page.URL)
		fmt.Println()
	}

	fmt.Printf("Pages you updated (%d):\n", len(updatedPages))
	for _, page := range updatedPages {
		fmt.Printf("- %s: %s\n", page.LastEditedTime.Format("2006-01-02 15:04"), page.Title)
		fmt.Printf("  URL: %s\n", page.URL)
		fmt.Printf("  Originally created by: %s\n", page.CreatedBy.Name)
		fmt.Println()
	}

	// Summary statistics
	fmt.Printf("Notion summary from %s to %s:\n", startDateStr, endDateStr)
	fmt.Printf("Total pages created: %d\n", len(createdPages))
	fmt.Printf("Total pages updated: %d\n", len(updatedPages))
	fmt.Printf("Total activity: %d pages\n", len(createdPages)+len(updatedPages))

	// Daily activity analysis
	dailyActivity := make(map[string]int)
	for _, page := range createdPages {
		date := page.CreatedTime.Format("2006-01-02")
		dailyActivity[date]++
	}
	for _, page := range updatedPages {
		date := page.LastEditedTime.Format("2006-01-02")
		dailyActivity[date]++
	}

	if len(dailyActivity) > 0 {
		fmt.Println("\nActivity per day:")
		var sortedDates []string
		for date := range dailyActivity {
			sortedDates = append(sortedDates, date)
		}
		sort.Strings(sortedDates)

		for _, date := range sortedDates {
			fmt.Printf("- %s: %d activities\n", date, dailyActivity[date])
		}
	}
}