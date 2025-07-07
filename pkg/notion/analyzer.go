package notion

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"dev-stats/pkg/common"
	"dev-stats/pkg/config"
)

const (
	notionAPIURL = "https://api.notion.com/v1"
	apiVersion   = "2022-06-28"
)

// NotionAnalyzer implements the Analyzer interface for Notion
type NotionAnalyzer struct {
	token          string
	client         *common.HTTPClient
	categoryConfig *config.CategorizationConfig
	relationCache  map[string]string // Cache for relation page titles
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

// CategoryStats represents analysis of page categories
type CategoryStats struct {
	Categories      map[string]int `json:"categories"`
	DailyWorkLogs   int            `json:"daily_work_logs"`
	MeetingNotes    int            `json:"meeting_notes"`
	TechnicalDocs   int            `json:"technical_docs"`
	ProjectPlanning int            `json:"project_planning"`
}

// WorkPatterns represents work activity patterns
type WorkPatterns struct {
	HourlyActivity map[int]int    `json:"hourly_activity"`
	DailyActivity  map[string]int `json:"daily_activity"`
	PeakHour       int            `json:"peak_hour"`
	PeakDay        string         `json:"peak_day"`
}

// NewNotionAnalyzer creates a new Notion analyzer
func NewNotionAnalyzer() *NotionAnalyzer {
	client := common.NewHTTPClient()

	// Load category configuration
	categoryConfig, err := config.LoadCategorizationConfig("")
	if err != nil {
		// Return nil to indicate initialization failure
		// The caller should handle this error
		fmt.Printf("Error: Failed to load category config: %v\n", err)
		return nil
	}

	return &NotionAnalyzer{
		token:          os.Getenv("NOTION_TOKEN"),
		client:         client,
		categoryConfig: categoryConfig,
		relationCache:  make(map[string]string),
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
func (n *NotionAnalyzer) Analyze(config *common.Config, writer io.Writer) (*common.AnalysisResult, error) {
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

	fmt.Fprintf(writer, "Analyzing Notion activity for user: %s (ID: %s)\n", currentUser.Name, currentUser.ID)

	// Auto-detect the actual user ID
	fmt.Fprintln(writer, "Auto-detecting user ID from workspace pages...")
	detectedUserID := n.detectActualUserID(writer)
	var targetUserID string

	if detectedUserID != "" && detectedUserID != currentUser.ID {
		fmt.Fprintf(writer, "Detected workspace user ID: %s (different from Integration Token user: %s)\n", detectedUserID, currentUser.ID)
		targetUserID = detectedUserID
	} else {
		fmt.Fprintf(writer, "Using Integration Token user ID: %s\n", currentUser.ID)
		targetUserID = currentUser.ID
	}

	// Search for pages
	fmt.Fprintln(writer, "Searching for pages...")
	pages, err := n.searchPages(writer, targetUserID, config.StartDate, config.EndDate)
	if err != nil {
		return nil, common.WrapError(err, "failed to search pages")
	}

	// Categorize pages
	createdPages, updatedPages := n.categorizePages(pages, targetUserID)

	// Analyze categories and patterns
	categoryStats := n.analyzeCategoryStats(createdPages, updatedPages)
	workPatterns := n.analyzeWorkPatterns(createdPages, updatedPages)

	// Create result
	result := &common.AnalysisResult{
		AnalyzerName: n.GetName(),
		StartDate:    config.StartDate,
		EndDate:      config.EndDate,
		Summary: map[string]interface{}{
			"Pages created":      len(createdPages),
			"Pages updated":      len(updatedPages),
			"Total activity":     len(createdPages) + len(updatedPages),
			"Total pages found":  len(pages),
			"Work categories":    len(categoryStats.Categories),
			"Daily work logs":    categoryStats.DailyWorkLogs,
			"Meeting notes":      categoryStats.MeetingNotes,
			"Technical docs":     categoryStats.TechnicalDocs,
			"Project planning":   categoryStats.ProjectPlanning,
			"Peak activity day":  workPatterns.PeakDay,
			"Peak activity hour": workPatterns.PeakHour,
		},
		Details: map[string]interface{}{
			"created_pages":  createdPages,
			"updated_pages":  updatedPages,
			"all_pages":      pages,
			"category_stats": categoryStats,
			"work_patterns":  workPatterns,
		},
	}

	n.printResults(writer, result, createdPages, updatedPages, targetUserID, categoryStats, workPatterns)
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

func (n *NotionAnalyzer) detectActualUserID(writer io.Writer) string {
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
		fmt.Fprintf(writer, "Warning: Failed to auto-detect user ID: %v\n", err)
		return ""
	}

	var response SearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintf(writer, "Warning: Failed to parse search response for auto-detection: %v\n", err)
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

func (n *NotionAnalyzer) searchPages(writer io.Writer, userID string, startDate, endDate time.Time) ([]Page, error) {
	var allPages []Page
	var cursor string
	requestCount := 0
	consecutiveOldPages := 0
	maxConsecutiveOldPages := 500

	// Cache for database titles and user names
	databaseCache := make(map[string]string)
	userCache := make(map[string]string)

	fmt.Fprintf(writer, "Searching pages (stopping when %d consecutive pages are outside date range)...\n", maxConsecutiveOldPages)

	for {
		var requestBodyBuilder strings.Builder
		requestBodyBuilder.WriteString(`{
            "sort": {
                "direction": "descending",
                "timestamp": "last_edited_time"
            }`)

		if cursor != "" {
			requestBodyBuilder.WriteString(fmt.Sprintf(`,
            "start_cursor": "%s"`, cursor))
		}

		requestBodyBuilder.WriteString(`,
            "page_size": 100
}`)
		requestBody := requestBodyBuilder.String()

		url := fmt.Sprintf("%s/search", notionAPIURL)
		requestCount++
		fmt.Fprintf(writer, "API Request #%d (fetching up to 100 pages)...", requestCount)

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

			// Get specified user ID from environment, fallback to detected user ID
			specifiedUserID := os.Getenv("NOTION_USER_ID")
			if specifiedUserID == "" {
				specifiedUserID = userID
			}

			// Check if specified user created or edited this page
			isUserInvolved := (page.CreatedBy.ID == specifiedUserID) || (page.LastEditedBy.ID == specifiedUserID)

			// Check if activity happened in date range
			// Extend END_DATE by 10 days for search purposes while keeping original for file/directory names
			endDateExtended := endDate.AddDate(0, 0, 10) // Add 10 days to END_DATE for search
			inDateRange := (page.CreatedTime.After(startDate) && page.CreatedTime.Before(endDate.AddDate(0, 0, 1))) ||
				(page.LastEditedTime.After(startDate) && page.LastEditedTime.Before(endDateExtended.AddDate(0, 0, 1)))

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

		fmt.Fprintf(writer, " found %d/%d pages in date range (%d user pages)\n", pagesInRange, len(response.Results), userPagesFound)

		// Early termination condition check
		if pagesInRange == 0 {
			consecutiveOldPages += len(response.Results)
		} else {
			consecutiveOldPages = 0
		}

		if consecutiveOldPages >= maxConsecutiveOldPages {
			fmt.Fprintf(writer, "Stopped search: %d consecutive pages outside date range (search appears complete)\n", consecutiveOldPages)
			break
		}

		if !response.HasMore {
			break
		}
		cursor = response.NextCursor
	}

	fmt.Fprintf(writer, "Total API requests made: %d\n", requestCount)

	fmt.Fprintf(writer, "Total unique pages found: %d\n", len(allPages))
	return allPages, nil
}

// getPageDetails fetches detailed information for a specific page
func (n *NotionAnalyzer) getPageDetails(pageID string) (*Page, error) {
	url := fmt.Sprintf("%s/pages/%s", notionAPIURL, pageID)
	body, err := n.client.Get(url, nil)
	if err != nil {
		return nil, err
	}

	var page Page
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}

	return &page, nil
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

// getRelatedPageTitle retrieves the title of a related page by its ID with caching
func (n *NotionAnalyzer) getRelatedPageTitle(pageID string) string {
	// Check cache first
	if title, exists := n.relationCache[pageID]; exists {
		return title
	}

	url := fmt.Sprintf("%s/pages/%s", notionAPIURL, pageID)
	body, err := n.client.Get(url, nil)
	if err != nil {
		// Cache empty result to avoid repeated failed requests
		n.relationCache[pageID] = ""
		return ""
	}

	var page Page
	if err := json.Unmarshal(body, &page); err != nil {
		// Cache empty result to avoid repeated failed requests
		n.relationCache[pageID] = ""
		return ""
	}

	title := n.extractPageTitle(page)
	// Cache the result for future use
	n.relationCache[pageID] = title
	return title
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

// extractPropertyValue extracts the value of a property based on its type
func (n *NotionAnalyzer) extractPropertyValue(property interface{}) string {
	if prop, ok := property.(map[string]interface{}); ok {
		propType, exists := prop["type"].(string)
		if !exists {
			return ""
		}

		switch propType {
		case "select":
			if selectProp, ok := prop["select"].(map[string]interface{}); ok {
				if name, ok := selectProp["name"].(string); ok {
					return name
				}
			}
		case "relation":
			// Handle database relations
			if relationArray, ok := prop["relation"].([]interface{}); ok {
				if len(relationArray) > 0 {
					// Get the actual page titles from relation IDs
					var relationTitles []string
					for _, rel := range relationArray {
						if relObj, ok := rel.(map[string]interface{}); ok {
							if pageID, ok := relObj["id"].(string); ok {
								if title := n.getRelatedPageTitle(pageID); title != "" {
									relationTitles = append(relationTitles, title)
								}
							}
						}
					}
					if len(relationTitles) > 0 {
						return strings.Join(relationTitles, ", ")
					}
					return fmt.Sprintf("関連あり (%d項目)", len(relationArray))
				}
			}
		case "number":
			if number, ok := prop["number"].(float64); ok {
				// Check if it's a whole number to avoid unnecessary decimal places
				if number == float64(int(number)) {
					return fmt.Sprintf("%.0f", number)
				} else {
					return fmt.Sprintf("%.1f", number)
				}
			}
		case "rich_text":
			if richTextArray, ok := prop["rich_text"].([]interface{}); ok {
				return n.extractTextFromRichTextArray(richTextArray)
			}
		case "title":
			if titleArray, ok := prop["title"].([]interface{}); ok {
				return n.extractTextFromRichTextArray(titleArray)
			}
		}
	}
	return ""
}

// getPageProperties extracts specific properties from a page
func (n *NotionAnalyzer) getPageProperties(page Page) (project string, workTime string) {
	if page.Properties == nil {
		return "", ""
	}

	// Debug: log all property names to understand the structure
	for propName, propValue := range page.Properties {
		if strings.Contains(propName, "プロジェクト") {
			project = n.extractPropertyValue(propValue)
		}
		if strings.Contains(propName, "作業時間") {
			workTime = n.extractPropertyValue(propValue)
		}
	}

	if workTime == "" {
		if workTimeProp, exists := page.Properties["作業時間"]; exists {
			workTime = n.extractPropertyValue(workTimeProp)
		}
	}

	return project, workTime
}

func (n *NotionAnalyzer) categorizePages(pages []Page, userID string) (created []Page, updated []Page) {
	// Get specified user ID from environment, fallback to detected user ID
	specifiedUserID := os.Getenv("NOTION_USER_ID")
	if specifiedUserID == "" {
		specifiedUserID = userID
	}

	for _, page := range pages {
		if page.CreatedBy.ID == specifiedUserID {
			created = append(created, page)
		}
		if page.LastEditedBy.ID == specifiedUserID && page.CreatedBy.ID != specifiedUserID {
			updated = append(updated, page)
		}
	}
	return created, updated
}

func (n *NotionAnalyzer) printResults(writer io.Writer, result *common.AnalysisResult, createdPages, updatedPages []Page, targetUserID string, categoryStats *CategoryStats, workPatterns *WorkPatterns) {
	userIDDisplay := targetUserID
	if len(targetUserID) > 8 {
		userIDDisplay = targetUserID[:8]
	}
	fmt.Fprintf(writer, "Found %d pages where user %s was involved\n", len(createdPages)+len(updatedPages), userIDDisplay)

	// Sort pages by last edited time
	sort.Slice(createdPages, func(i, j int) bool {
		return createdPages[i].LastEditedTime.Before(createdPages[j].LastEditedTime)
	})
	sort.Slice(updatedPages, func(i, j int) bool {
		return updatedPages[i].LastEditedTime.Before(updatedPages[j].LastEditedTime)
	})

	fmt.Fprintf(writer, "\nNotion activity from %s to %s:\n",
		result.StartDate.Format("2006-01-02"),
		result.EndDate.Format("2006-01-02"))

	fmt.Fprintf(writer, "\nPages you created (%d):\n", len(createdPages))
	for _, page := range createdPages {
		fmt.Fprintf(writer, "- %s: %s\n", page.LastEditedTime.Format("2006-01-02 15:04"), page.Title)
		fmt.Fprintf(writer, "  URL: %s\n", page.URL)

		// Display properties if they exist
		project, workTime := n.getPageProperties(page)
		if project != "" {
			fmt.Fprintf(writer, "  プロジェクト: %s\n", project)
		}
		if workTime != "" {
			fmt.Fprintf(writer, "  作業時間: %s\n", workTime)
		}

		fmt.Fprintln(writer)
	}

	fmt.Fprintf(writer, "Pages you updated (%d):\n", len(updatedPages))
	for _, page := range updatedPages {
		fmt.Fprintf(writer, "- %s: %s\n", page.LastEditedTime.Format("2006-01-02 15:04"), page.Title)
		fmt.Fprintf(writer, "  URL: %s\n", page.URL)

		// Display properties if they exist
		project, workTime := n.getPageProperties(page)
		if project != "" {
			fmt.Fprintf(writer, "  プロジェクト: %s\n", project)
		}
		if workTime != "" {
			fmt.Fprintf(writer, "  作業時間: %s\n", workTime)
		}

		creatorName := page.CreatedBy.Name
		if creatorName == "" {
			creatorName = "-"
		}
		fmt.Fprintf(writer, "  Originally created by: %s\n", creatorName)
		fmt.Fprintln(writer)
	}

	// Print category analysis
	fmt.Fprintln(writer, "\nWork Category Analysis:")
	// Sort categories for deterministic output
	var categories []string
	for category := range categoryStats.Categories {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	for _, category := range categories {
		fmt.Fprintf(writer, "- %s: %d pages\n", category, categoryStats.Categories[category])
	}

	// Print work patterns
	fmt.Fprintf(writer, "\nWork Patterns:\n")
	fmt.Fprintf(writer, "- Peak activity hour: %02d:00\n", workPatterns.PeakHour)
	fmt.Fprintf(writer, "- Peak activity day: %s\n", workPatterns.PeakDay)

	result.PrintSummary(writer)
}

// analyzeCategoryStats analyzes page categories based on titles and content
func (n *NotionAnalyzer) analyzeCategoryStats(createdPages, updatedPages []Page) *CategoryStats {
	stats := &CategoryStats{
		Categories: make(map[string]int),
	}

	allPages := append(createdPages, updatedPages...)

	for _, page := range allPages {
		title := strings.ToLower(page.Title)

		// Categorize by title patterns using configuration
		category := n.categoryConfig.CategorizeNotionPage(title)

		switch category {
		case "daily work log":
			stats.DailyWorkLogs++
			stats.Categories["Daily Work Log"]++
		case "meeting notes":
			stats.MeetingNotes++
			stats.Categories["Meeting Notes"]++
		case "technical documentation":
			stats.TechnicalDocs++
			stats.Categories["Technical Documentation"]++
		case "project planning":
			stats.ProjectPlanning++
			stats.Categories["Project Planning"]++
		default:
			stats.Categories["Other"]++
		}
	}

	return stats
}

// analyzeWorkPatterns analyzes when work activities occur
func (n *NotionAnalyzer) analyzeWorkPatterns(createdPages, updatedPages []Page) *WorkPatterns {
	patterns := &WorkPatterns{
		HourlyActivity: make(map[int]int),
		DailyActivity:  make(map[string]int),
	}

	allPages := append(createdPages, updatedPages...)

	for _, page := range allPages {
		// Use last edited time for activity analysis
		hour := page.LastEditedTime.Hour()
		dayOfWeek := page.LastEditedTime.Weekday().String()

		patterns.HourlyActivity[hour]++
		patterns.DailyActivity[dayOfWeek]++
	}

	// Find peak activity times
	maxHourActivity := 0
	maxDayActivity := 0

	for hour, count := range patterns.HourlyActivity {
		if count > maxHourActivity {
			maxHourActivity = count
			patterns.PeakHour = hour
		}
	}

	for day, count := range patterns.DailyActivity {
		if count > maxDayActivity {
			maxDayActivity = count
			patterns.PeakDay = day
		}
	}

	return patterns
}
