package notion

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dev-stats/pkg/common"
)

// NotionDownloader handles downloading specific Notion pages
type NotionDownloader struct {
	token  string
	client *common.HTTPClient
}

// PageDownloadInfo represents a page to be downloaded
type PageDownloadInfo struct {
	Title    string
	URL      string
	PageID   string
	Category string
}

// CategoryInfo represents a category of pages
type CategoryInfo struct {
	Name  string
	Pages []PageDownloadInfo
}

// DownloadConfig represents the download configuration
type DownloadConfig struct {
	Categories   []CategoryInfo
	OutputDir    string
	StartDate    string
	EndDate      string
	MarkdownPath string // Path to the original markdown file
}

// NewNotionDownloader creates a new Notion downloader
func NewNotionDownloader() *NotionDownloader {
	client := common.NewHTTPClient()
	return &NotionDownloader{
		token:  os.Getenv("NOTION_TOKEN"),
		client: client,
	}
}

// GetName returns the downloader name
func (d *NotionDownloader) GetName() string {
	return "NotionDownloader"
}

// ValidateConfig validates the required configuration
func (d *NotionDownloader) ValidateConfig() error {
	if d.token == "" {
		return common.NewError("NOTION_TOKEN environment variable is required")
	}
	return nil
}

// LoadFromMarkdown loads page URLs from a markdown file
func (d *NotionDownloader) LoadFromMarkdown(markdownPath string) (*DownloadConfig, error) {
	file, err := os.Open(markdownPath)
	if err != nil {
		return nil, common.WrapError(err, "failed to open markdown file")
	}
	defer file.Close()

	config := &DownloadConfig{
		Categories:   make([]CategoryInfo, 0),
		MarkdownPath: markdownPath, // Store the original file path
	}

	scanner := bufio.NewScanner(file)
	var currentCategory *CategoryInfo
	var currentTitle string

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		// Extract date range from title
		if strings.Contains(trimmedLine, "to") && strings.Contains(trimmedLine, "2025") {
			dateRegex := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}) to (\d{4}-\d{2}-\d{2})`)
			if matches := dateRegex.FindStringSubmatch(trimmedLine); len(matches) == 3 {
				config.StartDate = matches[1]
				config.EndDate = matches[2]
				config.OutputDir = fmt.Sprintf("notion-downloads/%s_to_%s", matches[1], matches[2])
			}
		}

		// Detect category headers (## Category Name)
		if strings.HasPrefix(trimmedLine, "## ") {
			categoryName := strings.TrimPrefix(trimmedLine, "## ")
			categoryInfo := CategoryInfo{
				Name:  categoryName,
				Pages: make([]PageDownloadInfo, 0),
			}
			config.Categories = append(config.Categories, categoryInfo)
			currentCategory = &config.Categories[len(config.Categories)-1]
		}

		// Extract page title (- Title)
		if strings.HasPrefix(trimmedLine, "- ") && currentCategory != nil && !strings.HasPrefix(line, "    -") {
			currentTitle = strings.TrimPrefix(trimmedLine, "- ")
		}

		// Extract URL (    - https://...)
		if strings.HasPrefix(line, "    - https://www.notion.so/") && currentCategory != nil && currentTitle != "" {
			url := strings.TrimPrefix(line, "    - ")
			pageID := d.extractPageIDFromURL(url)

			pageInfo := PageDownloadInfo{
				Title:    currentTitle,
				URL:      url,
				PageID:   pageID,
				Category: currentCategory.Name,
			}

			// Add page to current category
			currentCategory.Pages = append(currentCategory.Pages, pageInfo)
			currentTitle = "" // Reset for next page
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, common.WrapError(err, "failed to read markdown file")
	}

	return config, nil
}

// extractPageIDFromURL extracts the page ID from a Notion URL
func (d *NotionDownloader) extractPageIDFromURL(url string) string {
	// Extract ID from URLs like:
	// https://www.notion.so/page-title-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa#fragment
	// https://www.notion.so/another-page-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb

	// Remove fragment identifier (#)
	if idx := strings.Index(url, "#"); idx != -1 {
		url = url[:idx]
	}

	// Extract the last part after the last dash
	parts := strings.Split(url, "-")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		// Remove any trailing characters that aren't part of the ID
		if len(lastPart) >= 32 {
			return lastPart[:32]
		}
	}

	return ""
}

// DownloadPages downloads all pages specified in the config
func (d *NotionDownloader) DownloadPages(config *DownloadConfig, writer io.Writer) error {
	if err := d.ValidateConfig(); err != nil {
		return err
	}

	d.client.SetHeader("Authorization", "Bearer "+d.token)
	d.client.SetHeader("Notion-Version", apiVersion)
	d.client.SetHeader("Content-Type", "application/json")

	fmt.Fprintf(writer, "Starting download of %d categories to: %s\n", len(config.Categories), config.OutputDir)

	totalPages := 0
	for _, category := range config.Categories {
		totalPages += len(category.Pages)
	}

	fmt.Fprintf(writer, "Total pages to download: %d\n", totalPages)

	// Create output directory structure
	if err := d.createDirectoryStructure(config); err != nil {
		return common.WrapError(err, "failed to create directory structure")
	}

	// Download pages by category
	downloadedCount := 0
	titlesUpdated := false
	for categoryIdx, category := range config.Categories {
		fmt.Fprintf(writer, "\nDownloading category: %s (%d pages)\n", category.Name, len(category.Pages))

		for pageIdx, page := range category.Pages {
			fmt.Fprintf(writer, "  Downloading: %s\n", page.Title)

			// Get page details to extract actual title
			pageDetails, err := d.getPageDetails(page.PageID)
			if err != nil {
				fmt.Fprintf(writer, "    Warning: Failed to get page details for %s: %v\n", page.Title, err)
				continue
			}

			// Extract actual page title
			actualTitle := d.extractPageTitle(*pageDetails)
			if actualTitle == "" {
				actualTitle = page.Title // fallback to original title
			}

			// Update the title in config if it's different
			if actualTitle != page.Title {
				config.Categories[categoryIdx].Pages[pageIdx].Title = actualTitle
				titlesUpdated = true
			}

			if err := d.downloadSinglePageWithTitle(page, actualTitle, config, writer); err != nil {
				fmt.Fprintf(writer, "    Warning: Failed to download %s: %v\n", actualTitle, err)
				continue
			}

			downloadedCount++
			fmt.Fprintf(writer, "    ✓ Downloaded (%d/%d): %s\n", downloadedCount, totalPages, actualTitle)

			// Rate limiting
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Fprintf(writer, "\nDownload completed: %d/%d pages successful\n", downloadedCount, totalPages)

	// Update the original markdown file with actual titles if any were updated
	if titlesUpdated {
		fmt.Fprintf(writer, "\nUpdating original markdown file with actual page titles...\n")
		if err := d.updateMarkdownFile(config, writer); err != nil {
			fmt.Fprintf(writer, "Warning: Failed to update markdown file: %v\n", err)
		} else {
			fmt.Fprintf(writer, "✓ Markdown file updated with actual page titles\n")
		}
	}

	return nil
}

// createDirectoryStructure creates the necessary directory structure
func (d *NotionDownloader) createDirectoryStructure(config *DownloadConfig) error {
	for _, category := range config.Categories {
		// Use the category name directly as directory name
		fullPath := filepath.Join(config.OutputDir, category.Name)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return err
		}
	}

	return nil
}

// downloadSinglePageWithTitle downloads a single Notion page with specified title
func (d *NotionDownloader) downloadSinglePageWithTitle(page PageDownloadInfo, actualTitle string, config *DownloadConfig, writer io.Writer) error {
	// Get page details
	pageDetails, err := d.getPageDetails(page.PageID)
	if err != nil {
		return common.WrapError(err, "failed to get page details")
	}

	// Get page content (blocks)
	blocks, err := d.getPageBlocks(page.PageID)
	if err != nil {
		return common.WrapError(err, "failed to get page blocks")
	}

	// Convert to markdown
	markdown := d.convertToMarkdown(pageDetails, blocks)

	// Save to file using actual title (sanitize for filesystem)
	fileName := d.sanitizeFileNameMinimal(actualTitle) + ".md"
	categoryDir := d.getCategoryDirectory(page.Category)
	filePath := filepath.Join(config.OutputDir, categoryDir, fileName)

	if err := os.WriteFile(filePath, []byte(markdown), 0644); err != nil {
		return common.WrapError(err, "failed to write file")
	}

	return nil
}

// updateMarkdownFile updates the original markdown file with actual page titles
func (d *NotionDownloader) updateMarkdownFile(config *DownloadConfig, writer io.Writer) error {
	markdownPath := config.MarkdownPath

	// Read the original file
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		return common.WrapError(err, "failed to read original markdown file")
	}

	lines := strings.Split(string(content), "\n")
	updatedLines := make([]string, 0, len(lines))

	var currentCategory string
	pageIndex := 0

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect category headers
		if strings.HasPrefix(trimmedLine, "## ") {
			currentCategory = strings.TrimPrefix(trimmedLine, "## ")
			pageIndex = 0
			updatedLines = append(updatedLines, line)
			continue
		}

		// Check if this is a page title line that needs updating
		if strings.HasPrefix(trimmedLine, "- ") && currentCategory != "" && !strings.HasPrefix(line, "    -") {
			// Find the corresponding page in our config
			for _, category := range config.Categories {
				if category.Name == currentCategory && pageIndex < len(category.Pages) {
					// Replace with actual title
					actualTitle := category.Pages[pageIndex].Title
					updatedLines = append(updatedLines, fmt.Sprintf("- %s", actualTitle))
					pageIndex++
					goto nextLine
				}
			}
			// If not found, keep original line
			updatedLines = append(updatedLines, line)
		} else {
			updatedLines = append(updatedLines, line)
		}

	nextLine:
	}

	// Write the updated content back to the file
	updatedContent := strings.Join(updatedLines, "\n")
	if err := os.WriteFile(markdownPath, []byte(updatedContent), 0644); err != nil {
		return common.WrapError(err, "failed to write updated markdown file")
	}

	return nil
}

// downloadSinglePage downloads a single Notion page
func (d *NotionDownloader) downloadSinglePage(page PageDownloadInfo, config *DownloadConfig, writer io.Writer) error {
	// Get page details
	pageDetails, err := d.getPageDetails(page.PageID)
	if err != nil {
		return common.WrapError(err, "failed to get page details")
	}

	// Get page content (blocks)
	blocks, err := d.getPageBlocks(page.PageID)
	if err != nil {
		return common.WrapError(err, "failed to get page blocks")
	}

	// Extract actual page title from Notion
	actualTitle := d.extractPageTitle(*pageDetails)
	if actualTitle == "" {
		actualTitle = page.Title // fallback to original title
	}

	// Update the page info with actual title
	page.Title = actualTitle

	// Convert to markdown
	markdown := d.convertToMarkdown(pageDetails, blocks)

	// Save to file using actual title
	fileName := actualTitle + ".md"
	categoryDir := d.getCategoryDirectory(page.Category)
	filePath := filepath.Join(config.OutputDir, categoryDir, fileName)

	if err := os.WriteFile(filePath, []byte(markdown), 0644); err != nil {
		return common.WrapError(err, "failed to write file")
	}

	return nil
}

// getPageDetails fetches page details from Notion API
func (d *NotionDownloader) getPageDetails(pageID string) (*Page, error) {
	url := fmt.Sprintf("%s/pages/%s", notionAPIURL, pageID)
	body, err := d.client.Get(url, nil)
	if err != nil {
		return nil, err
	}

	var page Page
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}

	return &page, nil
}

// getPageBlocks fetches page blocks (content) from Notion API
func (d *NotionDownloader) getPageBlocks(pageID string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/blocks/%s/children", notionAPIURL, pageID)
	body, err := d.client.Get(url, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Results []map[string]interface{} `json:"results"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return response.Results, nil
}

// convertToMarkdown converts Notion page and blocks to markdown
func (d *NotionDownloader) convertToMarkdown(page *Page, blocks []map[string]interface{}) string {
	var md strings.Builder

	// Add title and metadata
	title := d.extractPageTitle(*page)
	md.WriteString(fmt.Sprintf("# %s\n\n", title))
	md.WriteString(fmt.Sprintf("**Created:** %s  \n", page.CreatedTime.Format("2006-01-02 15:04:05")))
	md.WriteString(fmt.Sprintf("**Last Edited:** %s  \n", page.LastEditedTime.Format("2006-01-02 15:04:05")))
	md.WriteString(fmt.Sprintf("**URL:** %s  \n\n", page.URL))
	md.WriteString("---\n\n")

	// Add properties if they exist
	if len(page.Properties) > 0 {
		project, workTime := d.getPageProperties(*page)
		if project != "" || workTime != "" {
			md.WriteString("## Properties\n\n")
			if project != "" {
				md.WriteString(fmt.Sprintf("- **Project:** %s\n", project))
			}
			if workTime != "" {
				md.WriteString(fmt.Sprintf("- **Work Time:** %s\n", workTime))
			}
			md.WriteString("\n")
		}
	}

	// Add content blocks
	md.WriteString("## Content\n\n")
	for _, block := range blocks {
		blockMd := d.convertBlockToMarkdown(block)
		if blockMd != "" {
			md.WriteString(blockMd)
			md.WriteString("\n")
		}
	}

	return md.String()
}

// convertBlockToMarkdown converts a single Notion block to markdown
func (d *NotionDownloader) convertBlockToMarkdown(block map[string]interface{}) string {
	blockType, ok := block["type"].(string)
	if !ok {
		return ""
	}

	switch blockType {
	case "paragraph":
		return d.extractRichText(block, "paragraph")
	case "heading_1":
		text := d.extractRichText(block, "heading_1")
		if text != "" {
			return fmt.Sprintf("# %s", text)
		}
	case "heading_2":
		text := d.extractRichText(block, "heading_2")
		if text != "" {
			return fmt.Sprintf("## %s", text)
		}
	case "heading_3":
		text := d.extractRichText(block, "heading_3")
		if text != "" {
			return fmt.Sprintf("### %s", text)
		}
	case "bulleted_list_item":
		text := d.extractRichText(block, "bulleted_list_item")
		if text != "" {
			return fmt.Sprintf("- %s", text)
		}
	case "numbered_list_item":
		text := d.extractRichText(block, "numbered_list_item")
		if text != "" {
			return fmt.Sprintf("1. %s", text)
		}
	case "to_do":
		text := d.extractRichText(block, "to_do")
		checked := false
		if todoBlock, ok := block["to_do"].(map[string]interface{}); ok {
			if checkedVal, ok := todoBlock["checked"].(bool); ok {
				checked = checkedVal
			}
		}
		if text != "" {
			checkbox := "[ ]"
			if checked {
				checkbox = "[x]"
			}
			return fmt.Sprintf("- %s %s", checkbox, text)
		}
	case "code":
		text := d.extractRichText(block, "code")
		language := ""
		if codeBlock, ok := block["code"].(map[string]interface{}); ok {
			if lang, ok := codeBlock["language"].(string); ok {
				language = lang
			}
		}
		if text != "" {
			return fmt.Sprintf("```%s\n%s\n```", language, text)
		}
	case "quote":
		text := d.extractRichText(block, "quote")
		if text != "" {
			return fmt.Sprintf("> %s", text)
		}
	case "divider":
		return "---"
	}

	return ""
}

// extractRichText extracts rich text from a block
func (d *NotionDownloader) extractRichText(block map[string]interface{}, blockType string) string {
	if blockData, ok := block[blockType].(map[string]interface{}); ok {
		if richText, ok := blockData["rich_text"].([]interface{}); ok {
			return d.extractTextFromRichTextArray(richText)
		}
	}
	return ""
}

// extractTextFromRichTextArray extracts plain text from rich text array (reuse from analyzer.go)
func (d *NotionDownloader) extractTextFromRichTextArray(richTextArray []interface{}) string {
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

// extractPageTitle extracts page title (reuse from analyzer.go)
func (d *NotionDownloader) extractPageTitle(page Page) string {
	for _, value := range page.Properties {
		if prop, ok := value.(map[string]interface{}); ok {
			if propType, exists := prop["type"].(string); exists && propType == "title" {
				if titleArray, ok := prop["title"].([]interface{}); ok {
					title := d.extractTextFromRichTextArray(titleArray)
					if title != "" {
						return title
					}
				}
			}
		}
	}
	return fmt.Sprintf("Page %s", page.ID[:8])
}

// getPageProperties extracts page properties (reuse from analyzer.go)
func (d *NotionDownloader) getPageProperties(page Page) (project string, workTime string) {
	if page.Properties == nil {
		return "", ""
	}

	for propName, propValue := range page.Properties {
		propNameLower := strings.ToLower(propName)

		// Check for project-related properties (supports multiple languages)
		if strings.Contains(propNameLower, "project") || strings.Contains(propName, "プロジェクト") {
			project = d.extractPropertyValue(propValue)
		}

		// Check for work time properties (supports multiple languages)
		if strings.Contains(propName, "作業時間") ||
			strings.Contains(propNameLower, "work time") ||
			strings.Contains(propNameLower, "working time") ||
			strings.Contains(propNameLower, "work hours") ||
			strings.Contains(propNameLower, "working hours") {
			workTime = d.extractPropertyValue(propValue)
		}
	}

	return project, workTime
}

// extractPropertyValue extracts property value (reuse from analyzer.go)
func (d *NotionDownloader) extractPropertyValue(property interface{}) string {
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
		case "number":
			if number, ok := prop["number"].(float64); ok {
				if number == float64(int(number)) {
					return fmt.Sprintf("%.0f", number)
				} else {
					return fmt.Sprintf("%.1f", number)
				}
			}
		case "rich_text":
			if richTextArray, ok := prop["rich_text"].([]interface{}); ok {
				return d.extractTextFromRichTextArray(richTextArray)
			}
		case "title":
			if titleArray, ok := prop["title"].([]interface{}); ok {
				return d.extractTextFromRichTextArray(titleArray)
			}
		}
	}
	return ""
}

// getCategoryDirectory returns the directory name for a category
func (d *NotionDownloader) getCategoryDirectory(category string) string {
	// Use the category name directly as directory name
	return category
}

// sanitizeFileNameMinimal sanitizes filename while preserving spaces, only removing filesystem-incompatible characters
func (d *NotionDownloader) sanitizeFileNameMinimal(name string) string {
	// Replace invalid characters for filesystem
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalid {
		name = strings.ReplaceAll(name, char, "_")
	}

	// Remove control characters but keep spaces
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1 // Remove control characters
		}
		return r
	}, name)

	// Trim whitespace from beginning and end
	name = strings.TrimSpace(name)

	// Limit length to avoid filesystem issues
	if len(name) > 200 {
		name = name[:200]
	}

	return name
}
