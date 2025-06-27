package common

import (
	"fmt"
	"sort"
	"time"
)

// Analyzer defines the interface that all analysis tools must implement
type Analyzer interface {
	// GetName returns the name of the analyzer
	GetName() string

	// Analyze performs the analysis and returns the results
	Analyze(config *Config) (*AnalysisResult, error)

	// ValidateConfig validates that the analyzer has the required configuration
	ValidateConfig() error
}

// AnalysisResult contains the results of an analysis
type AnalysisResult struct {
	AnalyzerName string                 `json:"analyzer_name"`
	StartDate    time.Time              `json:"start_date"`
	EndDate      time.Time              `json:"end_date"`
	Summary      map[string]interface{} `json:"summary"`
	Details      interface{}            `json:"details,omitempty"`
}

// AnalysisStats contains common statistics
type AnalysisStats struct {
	TotalItems    int                    `json:"total_items"`
	ItemsByType   map[string]int         `json:"items_by_type"`
	ItemsByPeriod map[string]int         `json:"items_by_period"`
	CustomStats   map[string]interface{} `json:"custom_stats"`
}

// PrintSummary prints a formatted summary of the analysis result
func (r *AnalysisResult) PrintSummary() {
	fmt.Printf("\n%s summary from %s to %s:\n",
		r.AnalyzerName,
		r.StartDate.Format("2006-01-02"),
		r.EndDate.Format("2006-01-02"))

	// Sort summary keys for deterministic output
	var keys []string
	for key := range r.Summary {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		fmt.Printf("%s: %v\n", key, r.Summary[key])
	}
}
