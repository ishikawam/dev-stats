package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"dev-stats/pkg/backlog"
	"dev-stats/pkg/calendar"
	"dev-stats/pkg/common"
	"dev-stats/pkg/github"
	"dev-stats/pkg/notion"
)

func main() {
	var (
		analyzerFlag = flag.String("analyzer", "", "Analyzer to run (github,backlog,calendar,notion,all)")
		helpFlag     = flag.Bool("help", false, "Show help")
		listFlag     = flag.Bool("list", false, "List available analyzers")
	)
	flag.Parse()

	if *helpFlag {
		printHelp()
		return
	}

	if *listFlag {
		printAvailableAnalyzers()
		return
	}

	if *analyzerFlag == "" {
		fmt.Println("Error: -analyzer flag is required")
		printHelp()
		os.Exit(1)
	}

	// Load common configuration
	config, err := common.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create analyzers
	analyzers := map[string]common.Analyzer{
		"github":   github.NewGitHubAnalyzer(),
		"backlog":  backlog.NewBacklogAnalyzer(),
		"calendar": calendar.NewCalendarAnalyzer(),
		"notion":   notion.NewNotionAnalyzer(),
	}

	// Determine which analyzers to run
	var analyzersToRun []common.Analyzer
	if *analyzerFlag == "all" {
		for _, analyzer := range analyzers {
			analyzersToRun = append(analyzersToRun, analyzer)
		}
	} else {
		requestedAnalyzers := strings.Split(*analyzerFlag, ",")
		for _, name := range requestedAnalyzers {
			name = strings.TrimSpace(name)
			if analyzer, exists := analyzers[name]; exists {
				analyzersToRun = append(analyzersToRun, analyzer)
			} else {
				log.Fatalf("Unknown analyzer: %s", name)
			}
		}
	}

	if len(analyzersToRun) == 0 {
		log.Fatal("No valid analyzers specified")
	}

	fmt.Printf("Running analysis from %s to %s\n",
		config.StartDate.Format("2006-01-02"),
		config.EndDate.Format("2006-01-02"))

	// Run analyzers
	var results []*common.AnalysisResult
	for _, analyzer := range analyzersToRun {
		fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
		fmt.Printf("Running %s analyzer...\n", analyzer.GetName())
		fmt.Printf(strings.Repeat("=", 60) + "\n")

		result, err := analyzer.Analyze(config)
		if err != nil {
			log.Printf("Error running %s analyzer: %v", analyzer.GetName(), err)
			continue
		}

		results = append(results, result)
	}

	// Print overall summary
	if len(results) > 1 {
		printOverallSummary(results)
	}

	fmt.Println("\nAnalysis completed successfully!")
}

func printHelp() {
	fmt.Println("dev-stats - Development Statistics Analyzer")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  dev-stats -analyzer <analyzer_name>")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -analyzer string    Analyzer to run (github,backlog,calendar,notion,all)")
	fmt.Println("  -list              List available analyzers")
	fmt.Println("  -help              Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  dev-stats -analyzer github")
	fmt.Println("  dev-stats -analyzer github,backlog")
	fmt.Println("  dev-stats -analyzer all")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  START_DATE         Start date in YYYY-MM-DD format")
	fmt.Println("  END_DATE           End date in YYYY-MM-DD format")
	fmt.Println()
	fmt.Println("  For GitHub:")
	fmt.Println("    GITHUB_TOKEN     GitHub personal access token")
	fmt.Println("    GITHUB_USERNAME  GitHub username")
	fmt.Println()
	fmt.Println("  For Backlog:")
	fmt.Println("    BACKLOG_API_KEY     Backlog API key")
	fmt.Println("    BACKLOG_SPACE_NAME  Backlog space name (e.g., 'yourspace' for yourspace.backlog.jp)")
	fmt.Println("    BACKLOG_USER_ID     Backlog user ID")
	fmt.Println("    BACKLOG_PROJECT_ID  Backlog project ID")
	fmt.Println()
	fmt.Println("  For Calendar:")
	fmt.Println("    No additional environment variables required")
	fmt.Println("    (Reads ICS files from storage/calendar directory)")
	fmt.Println()
	fmt.Println("  For Notion:")
	fmt.Println("    NOTION_TOKEN        Notion integration token")
}

func printAvailableAnalyzers() {
	fmt.Println("Available analyzers:")
	fmt.Println("  github   - GitHub pull request analysis")
	fmt.Println("  backlog  - Backlog issue and activity analysis")
	fmt.Println("  calendar - Calendar event analysis")
	fmt.Println("  notion   - Notion page analysis")
	fmt.Println("  all      - Run all available analyzers")
}

func printOverallSummary(results []*common.AnalysisResult) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Println("OVERALL SUMMARY")
	fmt.Printf(strings.Repeat("=", 60) + "\n")

	if len(results) == 0 {
		fmt.Println("No results to summarize.")
		return
	}

	startDate := results[0].StartDate
	endDate := results[0].EndDate

	fmt.Printf("\nPeriod: %s to %s\n", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	fmt.Printf("Analyzers run: %d\n", len(results))

	for _, result := range results {
		fmt.Printf("\n%s:\n", result.AnalyzerName)
		for key, value := range result.Summary {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}
}
