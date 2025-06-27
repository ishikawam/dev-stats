package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// Create output directory
	outputDir := createOutputDirectory(config.StartDate, config.EndDate)
	fmt.Printf("Output directory: %s\n", outputDir)

	// Run analyzers
	var results []*common.AnalysisResult
	for _, analyzer := range analyzersToRun {
		fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
		fmt.Printf("Running %s analyzer...\n", analyzer.GetName())
		fmt.Printf(strings.Repeat("=", 60) + "\n")

		// Capture output
		originalStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Buffer to capture output
		var outputBuffer bytes.Buffer
		done := make(chan bool)
		go func() {
			io.Copy(&outputBuffer, r)
			done <- true
		}()

		result, err := analyzer.Analyze(config)

		// Restore stdout and close pipe
		w.Close()
		os.Stdout = originalStdout
		<-done
		r.Close()

		if err != nil {
			log.Printf("Error running %s analyzer: %v", analyzer.GetName(), err)
			continue
		}

		// Print to console and save to file
		output := outputBuffer.String()
		fmt.Print(output)

		// Save to file
		analyzerName := strings.ToLower(strings.ReplaceAll(analyzer.GetName(), " ", "-"))
		filename := fmt.Sprintf("%s-stats.txt", analyzerName)
		filepath := filepath.Join(outputDir, filename)

		if err := saveOutputToFile(filepath, output); err != nil {
			log.Printf("Warning: Failed to save %s output to file: %v", analyzer.GetName(), err)
		} else {
			fmt.Printf("\n📁 Output saved to: %s\n", filepath)
		}

		results = append(results, result)
	}

	// Print overall summary
	if len(results) > 1 {
		printOverallSummary(results)
	}

	fmt.Println("\nAnalysis completed successfully!")
}

// createOutputDirectory creates a directory for storing output files
func createOutputDirectory(startDate, endDate time.Time) string {
	outputDir := fmt.Sprintf("stats/%s_to_%s",
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"))

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: Failed to create output directory %s: %v", outputDir, err)
		return "."
	}

	return outputDir
}

// saveOutputToFile saves the output string to a file
func saveOutputToFile(filepath, output string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(output)
	return err
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
