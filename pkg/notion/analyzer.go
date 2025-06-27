package notion

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"dev-stats/pkg/common"
)

const (
	notionAPIURL = "https://api.notion.com/v1"
	apiVersion   = "2022-06-28"
)

// NotionAnalyzer implements the Analyzer interface for Notion
type NotionAnalyzer struct {
	token  string
	client *common.HTTPClient
}

// User represents a Notion user
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Page represents a Notion page
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

// SearchResponse represents Notion search API response
type SearchResponse struct {
	Results    []json.RawMessage `json:"results"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor"`
}

// Database represents a Notion database
type Database struct {
	ID    string `json:"id"`
	Title []struct {
		PlainText string `json:"plain_text"`
	} `json:"title"`
}

// NewNotionAnalyzer creates a new Notion analyzer
func NewNotionAnalyzer() *NotionAnalyzer {
	client := common.NewHTTPClient()
	return &NotionAnalyzer{
		token:  os.Getenv("NOTION_TOKEN"),
		client: client,
	}
}

// GetName returns the analyzer name
func (n *NotionAnalyzer) GetName() string {
	return "Notion"
}

// ValidateConfig validates the required configuration
func (n *NotionAnalyzer) ValidateConfig() error {
	if n.token == "" {
		return common.NewError("NOTION_TOKEN environment variable is required")
	}
	return nil
}

// Analyze performs Notion analysis
func (n *NotionAnalyzer) Analyze(config *common.Config) (*common.AnalysisResult, error) {
	if err := n.ValidateConfig(); err != nil {
		return nil, err
	}

	n.client.SetHeader("Authorization", "Bearer "+n.token)
	n.client.SetHeader("Notion-Version", apiVersion)
	n.client.SetHeader("Content-Type", "application/json")

	// Get current user
	currentUser, err := n.getCurrentUser()
	if err != nil {
		return nil, common.WrapError(err, "failed to get current user")
	}

	fmt.Printf("Analyzing Notion activity for user: %s (ID: %s)\n", currentUser.Name, currentUser.ID)

	// Auto-detect the actual user ID
	fmt.Println("Auto-detecting user ID from workspace pages...")
	detectedUserID := n.detectActualUserID()
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
	pages, err := n.searchPages(targetUserID, config.StartDate, config.EndDate)
	if err != nil {
		return nil, common.WrapError(err, "failed to search pages")
	}

	// Categorize pages
	createdPages, updatedPages := n.categorizePages(pages, targetUserID)

	// Create result
	result := &common.AnalysisResult{
		AnalyzerName: n.GetName(),
		StartDate:    config.StartDate,
		EndDate:      config.EndDate,
		Summary: map[string]interface{}{
			"Pages created":     len(createdPages),
			"Pages updated":     len(updatedPages),
			"Total activity":    len(createdPages) + len(updatedPages),
			"Total pages found": len(pages),
		},
		Details: map[string]interface{}{
			"created_pages": createdPages,
			"updated_pages": updatedPages,
			"all_pages":     pages,
		},
	}

	n.printResults(result, createdPages, updatedPages, targetUserID)
	return result, nil
}

func (n *NotionAnalyzer) getCurrentUser() (*User, error) {
	url := fmt.Sprintf("%s/users/me", notionAPIURL)
	body, err := n.client.Get(url, nil)
	if err != nil {
		return nil, err
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, common.WrapError(err, "failed to parse user response")
	}

	return &user, nil
}

func (n *NotionAnalyzer) detectActualUserID() string {
	requestBody := `{
		"sort": {
			"direction": "descending",
			"timestamp": "last_edited_time"
		},
		"page_size": 10
	}`

	url := fmt.Sprintf("%s/search", notionAPIURL)
	body, err := n.client.Post(url, requestBody, nil)
	if err != nil {
		fmt.Printf("Warning: Failed to auto-detect user ID: %v\n", err)
		return ""
	}

	var response SearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Warning: Failed to parse search response for auto-detection: %v\n", err)
		return ""
	}

	// Count user IDs from the sample
	userIDCounts := make(map[string]int)

	for _, result := range response.Results {
		var objType struct {
			Object string `json:"object"`
		}
		if err := json.Unmarshal(result, &objType); err != nil {
			continue
		}

		if objType.Object != "page" {
			continue
		}

		var page Page
		if err := json.Unmarshal(result, &page); err != nil {
			continue
		}

		if page.CreatedBy.ID != "" {
			userIDCounts[page.CreatedBy.ID]++
		}
		if page.LastEditedBy.ID != "" {
			userIDCounts[page.LastEditedBy.ID]++
		}
	}

	// Find the most common user ID
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

func (n *NotionAnalyzer) searchPages(userID string, startDate, endDate time.Time) ([]Page, error) {
	var allPages []Page
	var cursor string
	requestCount := 0
	consecutiveOldPages := 0
	maxConsecutiveOldPages := 500

	// Cache for database titles and user names
	databaseCache := make(map[string]string)
	userCache := make(map[string]string)

	fmt.Printf("Searching pages (stopping when %d consecutive pages are outside date range)...\n", maxConsecutiveOldPages)

	for {
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
		requestCount++
		fmt.Printf("API Request #%d (fetching up to 100 pages)...", requestCount)

		body, err := n.client.Post(url, requestBody, nil)
		if err != nil {
			return nil, err
		}

		var response SearchResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, common.WrapError(err, "failed to parse search response")
		}

		// Filter pages by user and date range
		pagesInRange := 0
		userPagesFound := 0
		for _, result := range response.Results {
			var objType struct {
				Object string `json:"object"`
			}
			if err := json.Unmarshal(result, &objType); err != nil {
				continue
			}

			if objType.Object != "page" {
				continue
			}

			var page Page
			if err := json.Unmarshal(result, &page); err != nil {
				continue
			}

			// Check if user created or edited this page
			isUserInvolved := (page.CreatedBy.ID == userID) || (page.LastEditedBy.ID == userID)

			// Check if activity happened in date range
			inDateRange := (page.CreatedTime.After(startDate) && page.CreatedTime.Before(endDate.AddDate(0, 0, 1))) ||
				(page.LastEditedTime.After(startDate) && page.LastEditedTime.Before(endDate.AddDate(0, 0, 1)))

			if inDateRange {
				pagesInRange++
				if isUserInvolved {
					userPagesFound++

					// Try to get database title if this page is in a database
					if parent, ok := n.parseDatabaseParent(result); ok && parent != "" {
						if cachedTitle, exists := databaseCache[parent]; exists {
							page.DatabaseTitle = cachedTitle
						} else {
							if database, err := n.getDatabase(parent); err == nil {
								if len(database.Title) > 0 {
									page.DatabaseTitle = database.Title[0].PlainText
									databaseCache[parent] = page.DatabaseTitle
								}
							}
						}
					}

					// Try to get user name if not already available
					if page.CreatedBy.Name == "" && page.CreatedBy.ID != "" {
						if cachedName, exists := userCache[page.CreatedBy.ID]; exists {
							page.CreatedBy.Name = cachedName
						} else {
							if userName := n.getUserName(page.CreatedBy.ID); userName != "" {
								page.CreatedBy.Name = userName
								userCache[page.CreatedBy.ID] = userName
							}
						}
					}

					page.Title = n.extractPageTitle(page)
					allPages = append(allPages, page)
				}
			}
		}

		fmt.Printf(" found %d/%d pages in date range (%d user pages)\n", pagesInRange, len(response.Results), userPagesFound)

		// Early termination condition check
		if pagesInRange == 0 {
			consecutiveOldPages += len(response.Results)
		} else {
			consecutiveOldPages = 0
		}

		if consecutiveOldPages >= maxConsecutiveOldPages {
			fmt.Printf("Stopped search: %d consecutive pages outside date range (search appears complete)\n", consecutiveOldPages)
			break
		}

		if !response.HasMore {
			break
		}
		cursor = response.NextCursor
	}

	fmt.Printf("Total API requests made: %d\n", requestCount)
	return allPages, nil
}

func (n *NotionAnalyzer) parseDatabaseParent(result json.RawMessage) (string, bool) {
	var parentInfo struct {
		Parent struct {
			Type       string `json:"type"`
			DatabaseID string `json:"database_id"`
		} `json:"parent"`
	}

	if err := json.Unmarshal(result, &parentInfo); err != nil {
		return "", false
	}

	if parentInfo.Parent.Type == "database_id" && parentInfo.Parent.DatabaseID != "" {
		return parentInfo.Parent.DatabaseID, true
	}

	return "", false
}

func (n *NotionAnalyzer) getDatabase(databaseID string) (*Database, error) {
	url := fmt.Sprintf("%s/databases/%s", notionAPIURL, databaseID)
	body, err := n.client.Get(url, nil)
	if err != nil {
		return nil, err
	}

	var database Database
	if err := json.Unmarshal(body, &database); err != nil {
		return nil, common.WrapError(err, "failed to parse database response")
	}

	return &database, nil
}

func (n *NotionAnalyzer) getUserName(userID string) string {
	url := fmt.Sprintf("%s/users/%s", notionAPIURL, userID)
	body, err := n.client.Get(url, nil)
	if err != nil {
		return ""
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return ""
	}

	return user.Name
}

func (n *NotionAnalyzer) extractPageTitle(page Page) string {
	// Look for the actual title property (type: "title")
	for _, value := range page.Properties {
		if prop, ok := value.(map[string]interface{}); ok {
			if propType, exists := prop["type"].(string); exists && propType == "title" {
				if titleArray, ok := prop["title"].([]interface{}); ok {
					title := n.extractTextFromRichTextArray(titleArray)
					if title != "" {
						return title
					}
				}
			}
		}
	}

	// If no title found, fallback to page ID
	return fmt.Sprintf("Page %s", page.ID[:8])
}

func (n *NotionAnalyzer) extractTextFromRichTextArray(richTextArray []interface{}) string {
	var textParts []string

	for _, item := range richTextArray {
		if textObj, ok := item.(map[string]interface{}); ok {
			if plainText, ok := textObj["plain_text"].(string); ok && plainText != "" {
				textParts = append(textParts, plainText)
			}
		}
	}

	return strings.Join(textParts, "")
}

func (n *NotionAnalyzer) categorizePages(pages []Page, userID string) (created []Page, updated []Page) {
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

func (n *NotionAnalyzer) printResults(result *common.AnalysisResult, createdPages, updatedPages []Page, targetUserID string) {
	userIDDisplay := targetUserID
	if len(targetUserID) > 8 {
		userIDDisplay = targetUserID[:8]
	}
	fmt.Printf("Found %d pages where user %s was involved\n", len(createdPages)+len(updatedPages), userIDDisplay)

	// Sort pages by last edited time
	sort.Slice(createdPages, func(i, j int) bool {
		return createdPages[i].LastEditedTime.Before(createdPages[j].LastEditedTime)
	})
	sort.Slice(updatedPages, func(i, j int) bool {
		return updatedPages[i].LastEditedTime.Before(updatedPages[j].LastEditedTime)
	})

	fmt.Printf("\nNotion activity from %s to %s:\n",
		result.StartDate.Format("2006-01-02"),
		result.EndDate.Format("2006-01-02"))

	fmt.Printf("\nPages you created (%d):\n", len(createdPages))
	for _, page := range createdPages {
		fmt.Printf("- %s: %s\n", page.LastEditedTime.Format("2006-01-02 15:04"), page.Title)
		fmt.Printf("  URL: %s\n", page.URL)
		fmt.Println()
	}

	fmt.Printf("Pages you updated (%d):\n", len(updatedPages))
	for _, page := range updatedPages {
		fmt.Printf("- %s: %s\n", page.LastEditedTime.Format("2006-01-02 15:04"), page.Title)
		fmt.Printf("  URL: %s\n", page.URL)

		creatorName := page.CreatedBy.Name
		if creatorName == "" {
			creatorName = "-"
		}
		fmt.Printf("  Originally created by: %s\n", creatorName)
		fmt.Println()
	}

	result.PrintSummary()
}
