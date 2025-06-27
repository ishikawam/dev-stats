package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// CategorizationConfig represents the shared configuration for event categorization
type CategorizationConfig struct {
	Categories       map[string]CategoryDefinition `yaml:"categories"`
	EventCategories  map[string]EventRule          `yaml:"event_categories"`
	NotionCategories map[string]NotionRule         `yaml:"notion_categories"`
}

// CategoryDefinition defines a category with its name and keywords
type CategoryDefinition struct {
	Name     string   `yaml:"name"`
	Keywords []string `yaml:"keywords"`
}

// EventRule defines specific event categorization rules
type EventRule struct {
	Keywords []string `yaml:"keywords"`
	Category string   `yaml:"category"`
}

// NotionRule defines Notion-specific categorization rules
type NotionRule struct {
	Keywords []string `yaml:"keywords"`
}

// LoadCategorizationConfig loads categorization configuration from YAML file
func LoadCategorizationConfig(configPath string) (*CategorizationConfig, error) {
	if configPath == "" {
		// Default config path
		configPath = "config/categorization.yaml"
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file %s not found. Please create this file with categorization rules", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config CategorizationConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	return &config, nil
}

// CategorizeByKeywords categorizes a title using the loaded configuration
func (config *CategorizationConfig) CategorizeByKeywords(title string) string {
	title = strings.ToLower(title)

	// First, try specific event rules - sort for deterministic order
	var eventTypes []string
	for eventType := range config.EventCategories {
		eventTypes = append(eventTypes, eventType)
	}
	sort.Strings(eventTypes)

	for _, eventType := range eventTypes {
		rule := config.EventCategories[eventType]
		for _, keyword := range rule.Keywords {
			if strings.Contains(title, strings.ToLower(keyword)) {
				return eventType
			}
		}
	}

	// Fallback to general categories - sort for deterministic order
	var categoryNames []string
	for categoryName := range config.Categories {
		categoryNames = append(categoryNames, categoryName)
	}
	sort.Strings(categoryNames)

	for _, categoryName := range categoryNames {
		for _, keyword := range config.Categories[categoryName].Keywords {
			if strings.Contains(title, strings.ToLower(keyword)) {
				return categoryName
			}
		}
	}

	return "other"
}

// GetCategoryTime calculates time spent in each main category
func (config *CategorizationConfig) GetCategoryTime(title string) string {
	title = strings.ToLower(title)

	// Check each main category - sort for deterministic order
	var categoryNames []string
	for categoryName := range config.Categories {
		categoryNames = append(categoryNames, categoryName)
	}
	sort.Strings(categoryNames)

	for _, categoryName := range categoryNames {
		definition := config.Categories[categoryName]
		for _, keyword := range definition.Keywords {
			if strings.Contains(title, strings.ToLower(keyword)) {
				return categoryName
			}
		}
	}

	return "other"
}

// GetCategoryDisplayName returns the display name for a category
func (config *CategorizationConfig) GetCategoryDisplayName(category string) string {
	if def, exists := config.Categories[category]; exists {
		return def.Name
	}
	return strings.Title(category)
}

// CategorizeNotionPage categorizes a Notion page based on its title
func (config *CategorizationConfig) CategorizeNotionPage(title string) string {
	title = strings.ToLower(title)

	// Sort notion categories for deterministic order
	var notionCategories []string
	for category := range config.NotionCategories {
		notionCategories = append(notionCategories, category)
	}
	sort.Strings(notionCategories)

	for _, category := range notionCategories {
		rule := config.NotionCategories[category]
		for _, keyword := range rule.Keywords {
			if strings.Contains(title, strings.ToLower(keyword)) {
				return category
			}
		}
	}

	return "other"
}
