package main

import (
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
	"dev-stats/pkg/google"
	"dev-stats/pkg/notion"
)

func main() {
	var (
		analyzerFlag        = flag.String("analyzer", "", "Analyzer to run (github,backlog,calendar,notion,google,all)")
		downloadFlag        = flag.String("download", "", "Download Notion pages from markdown file")
		downloadGoogleFlag  = flag.Bool("download-google", false, "Download all Google Workspace files modified in START_DATE to END_DATE")
		listBacklogFlag     = flag.Bool("list-backlog", false, "List Backlog projects and members for all profiles")
		listBacklogProject  = flag.String("list-backlog-project", "", "List members of a specific Backlog project (specify project ID)")
		listBacklogProfiles = flag.Bool("list-backlog-profiles", false, "List all Backlog profiles")
		listBacklogClear    = flag.Bool("list-backlog-clear", false, "Clear cache and refresh Backlog data")
		helpFlag            = flag.Bool("help", false, "Show help")
		listFlag            = flag.Bool("list", false, "List available analyzers")
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

	// Handle Backlog profiles listing
	if *listBacklogProfiles {
		handleListBacklogProfiles()
		return
	}

	// Handle Backlog listing mode
	if *listBacklogFlag || *listBacklogProject != "" || *listBacklogClear {
		handleBacklogList(*listBacklogProject, *listBacklogClear)
		return
	}

	// Handle download mode
	if *downloadFlag != "" {
		handleDownload(*downloadFlag)
		return
	}

	// Handle Google Workspace download mode
	if *downloadGoogleFlag {
		handleDownloadGoogle()
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

	// Refuse to run if today is past END_DATE: results would be incomplete
	// because APIs filter by last_edited_time, so pages updated after END_DATE
	// would be excluded even if they were active during the target period.
	if time.Now().After(config.EndDate.AddDate(0, 0, 1)) {
		log.Fatalf("Error: today (%s) is past END_DATE (%s). Running now would produce incomplete stats because active files updated after END_DATE would be excluded. Update END_DATE in .env before running.",
			time.Now().Format("2006-01-02"),
			config.EndDate.Format("2006-01-02"))
	}

	// Create analyzers
	analyzers := make(map[string]common.Analyzer)

	if githubAnalyzer := github.NewGitHubAnalyzer(); githubAnalyzer != nil {
		analyzers["github"] = githubAnalyzer
	}

	// Note: Backlog analyzers are handled separately due to multi-profile support
	// They will be created dynamically when running backlog analysis

	if calendarAnalyzer := calendar.NewCalendarAnalyzer(); calendarAnalyzer != nil {
		analyzers["calendar"] = calendarAnalyzer
	}
	if notionAnalyzer := notion.NewNotionAnalyzer(); notionAnalyzer != nil {
		analyzers["notion"] = notionAnalyzer
	}
	analyzers["google"] = google.NewGDocsAnalyzer()

	// Determine which analyzers to run
	var analyzersToRun []common.Analyzer
	requestedAnalyzers := []string{}

	if *analyzerFlag == "all" {
		requestedAnalyzers = []string{"github", "backlog", "calendar", "notion", "google"}
	} else {
		for _, name := range strings.Split(*analyzerFlag, ",") {
			requestedAnalyzers = append(requestedAnalyzers, strings.TrimSpace(name))
		}
	}

	for _, name := range requestedAnalyzers {
		if name == "backlog" {
			// Handle Backlog separately due to multi-profile support
			continue
		}
		if analyzer, exists := analyzers[name]; exists {
			analyzersToRun = append(analyzersToRun, analyzer)
		} else {
			log.Fatalf("Unknown analyzer: %s", name)
		}
	}

	// Check if backlog was requested
	backlogRequested := false
	for _, name := range requestedAnalyzers {
		if name == "backlog" {
			backlogRequested = true
			break
		}
	}

	if len(analyzersToRun) == 0 && !backlogRequested {
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

	// Run Backlog analyzers for all profiles
	if backlogRequested {
		backlogProfiles := backlog.LoadBacklogProfiles()
		if len(backlogProfiles) == 0 {
			log.Println("Warning: No Backlog profiles found. Please set BACKLOG_<PROFILE>_* environment variables.")
		} else {
			for _, profile := range backlogProfiles {
				if !profile.IsAnalysisReady() {
					fmt.Printf("⚠️  Backlog profile '%s' is missing USER_ID or PROJECT_ID. Skipping analysis.\n", profile.Name)
					fmt.Printf("    Run 'make list-backlog' to find the IDs.\n\n")
					continue
				}

				analyzer := backlog.NewBacklogAnalyzerWithProfile(&profile)
				analyzerName := fmt.Sprintf("backlog-%s", strings.ToLower(profile.Name))
				filename := fmt.Sprintf("%s-stats.txt", analyzerName)
				filePath := filepath.Join(outputDir, filename)

				// Create file writer
				file, err := os.Create(filePath)
				if err != nil {
					log.Printf("Warning: Failed to create output file %s: %v", filePath, err)
					continue
				}
				defer file.Close()

				// Create multi-writer to write to both stdout and file
				writer := io.MultiWriter(os.Stdout, file)

				// Print header
				fmt.Fprintf(writer, "\n"+strings.Repeat("=", 60)+"\n")
				fmt.Fprintf(writer, "Running Backlog analyzer (%s)...\n", profile.Name)
				fmt.Fprintf(writer, strings.Repeat("=", 60)+"\n")

				result, err := analyzer.Analyze(config, writer)
				if err != nil {
					log.Printf("Error running Backlog analyzer (%s): %v", profile.Name, err)
					continue
				}

				fmt.Fprintf(writer, "\n📁 Output saved to: %s\n", filePath)

				results = append(results, result)
			}
		}
	}

	// Run other analyzers
	for _, analyzer := range analyzersToRun {
		analyzerName := strings.ToLower(strings.ReplaceAll(analyzer.GetName(), " ", "-"))
		filename := fmt.Sprintf("%s-stats.txt", analyzerName)
		filePath := filepath.Join(outputDir, filename)

		// Create file writer
		file, err := os.Create(filePath)
		if err != nil {
			log.Printf("Warning: Failed to create output file %s: %v", filePath, err)
			continue
		}
		defer file.Close()

		// Create multi-writer to write to both stdout and file
		writer := io.MultiWriter(os.Stdout, file)

		// Print header
		fmt.Fprintf(writer, "\n"+strings.Repeat("=", 60)+"\n")
		fmt.Fprintf(writer, "Running %s analyzer...\n", analyzer.GetName())
		fmt.Fprintf(writer, strings.Repeat("=", 60)+"\n")

		result, err := analyzer.Analyze(config, writer)
		if err != nil {
			log.Printf("Error running %s analyzer: %v", analyzer.GetName(), err)
			continue
		}

		fmt.Fprintf(writer, "\n📁 Output saved to: %s\n", filePath)

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
	outputDir := fmt.Sprintf("output/%s_to_%s/stats",
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"))

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: Failed to create output directory %s: %v", outputDir, err)
		return "."
	}

	return outputDir
}

// handleListBacklogProfiles lists all Backlog profiles
func handleListBacklogProfiles() {
	profiles := backlog.LoadBacklogProfiles()

	if len(profiles) == 0 {
		fmt.Println("No Backlog profiles found.")
		fmt.Println("\nTo configure Backlog profiles, set environment variables with pattern:")
		fmt.Println("  BACKLOG_<PROFILE_NAME>_API_KEY")
		fmt.Println("  BACKLOG_<PROFILE_NAME>_HOST            (e.g., mycompany.backlog.com)")
		fmt.Println("  BACKLOG_<PROFILE_NAME>_USER_ID         (optional, for analysis)")
		fmt.Println("  BACKLOG_<PROFILE_NAME>_PROJECT_ID      (optional, for analysis)")
		fmt.Println("\nExample:")
		fmt.Println("  BACKLOG_HOGE_API_KEY=your-api-key")
		fmt.Println("  BACKLOG_HOGE_HOST=mycompany.backlog.com")
		return
	}

	fmt.Print("\n=== Backlog Profiles ===\n\n")
	fmt.Printf("%-15s %-35s %-12s %-12s %s\n", "Profile", "Host", "User ID", "Project ID", "Status")
	fmt.Println(strings.Repeat("-", 90))

	for _, profile := range profiles {
		status := "Ready"
		if !profile.IsAnalysisReady() {
			if profile.UserID == "" || profile.ProjectID == "" {
				status = "Missing IDs"
			}
		}

		userID := profile.UserID
		if userID == "" {
			userID = "-"
		}
		projectID := profile.ProjectID
		if projectID == "" {
			projectID = "-"
		}

		fmt.Printf("%-15s %-35s %-12s %-12s %s\n",
			profile.Name,
			profile.Host,
			userID,
			projectID,
			status)
	}

	fmt.Printf("\nTotal profiles: %d\n", len(profiles))
}

// handleBacklogList handles Backlog listing functionality for all profiles
func handleBacklogList(projectID string, forceRefresh bool) {
	profiles := backlog.LoadBacklogProfiles()

	if len(profiles) == 0 {
		fmt.Println("No Backlog profiles found.")
		fmt.Println("Run './bin/dev-stats -list-backlog-profiles' for configuration help.")
		return
	}

	// Clear cache if reset is requested
	if forceRefresh {
		fmt.Println("🗑️  Clearing cache for all profiles...")
		for _, profile := range profiles {
			if err := backlog.ClearCache(profile.Name); err != nil {
				log.Printf("Warning: Failed to clear cache for profile '%s': %v", profile.Name, err)
			} else {
				fmt.Printf("✓ Cache cleared for profile: %s\n", profile.Name)
			}
		}
		fmt.Println()
	}

	for i, profile := range profiles {
		if i > 0 {
			fmt.Println("\n" + strings.Repeat("=", 80) + "\n")
		}

		fmt.Printf("Profile: %s (%s)\n", profile.Name, profile.GetBaseURL())

		analyzer := backlog.NewBacklogAnalyzerWithProfile(&profile)

		var err error
		if projectID != "" {
			// List members of a specific project (no caching for specific project query)
			err = analyzer.ListProjectMembers(projectID, os.Stdout)
		} else {
			// List all projects and their members (with caching)
			err = analyzer.ListAllProjectsAndMembersWithCache(os.Stdout, forceRefresh)
		}

		if err != nil {
			log.Printf("Error listing Backlog resources for profile '%s': %v", profile.Name, err)
		}
	}
}

// handleDownloadGoogle downloads all Google Workspace files modified in the config date range
func handleDownloadGoogle() {
	config, err := common.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if time.Now().After(config.EndDate.AddDate(0, 0, 1)) {
		log.Fatalf("Error: today (%s) is past END_DATE (%s). Running now would produce incomplete results. Update END_DATE in .env before running.",
			time.Now().Format("2006-01-02"),
			config.EndDate.Format("2006-01-02"))
	}

	d := google.NewGDocsDownloader()
	if err := d.DownloadAll(config.StartDate, config.EndDate, os.Stdout); err != nil {
		log.Fatalf("Failed to download Google Workspace files: %v", err)
	}

	fmt.Println("Google Workspace download completed successfully!")
}

// handleDownload handles the download functionality
func handleDownload(markdownFile string) {
	downloader := notion.NewNotionDownloader()

	// Load configuration from markdown file
	config, err := downloader.LoadFromMarkdown(markdownFile)
	if err != nil {
		log.Fatalf("Failed to load markdown file: %v", err)
	}

	fmt.Printf("Loaded configuration for period: %s to %s\n", config.StartDate, config.EndDate)
	fmt.Printf("Output directory: %s\n", config.OutputDir)

	// Download pages
	if err := downloader.DownloadPages(config, os.Stdout); err != nil {
		log.Fatalf("Failed to download pages: %v", err)
	}

	fmt.Println("Download completed successfully!")
}

func printHelp() {
	fmt.Println("dev-stats - Development Statistics Analyzer")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  dev-stats -analyzer <analyzer_name>")
	fmt.Println("  dev-stats -download <markdown_file>")
	fmt.Println("  dev-stats -download-google")
	fmt.Println("  dev-stats -list-backlog")
	fmt.Println("  dev-stats -list-backlog-profiles")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -analyzer string             Analyzer to run (github,backlog,calendar,notion,google,all)")
	fmt.Println("  -download string             Download Notion pages from markdown file")
	fmt.Println("  -download-google             Download Google Workspace files modified in date range")
	fmt.Println("  -list-backlog                List all Backlog projects and members (all profiles)")
	fmt.Println("  -list-backlog-project ID     List members of a specific Backlog project (all profiles)")
	fmt.Println("  -list-backlog-profiles       List all configured Backlog profiles")
	fmt.Println("  -list-backlog-clear          Clear cache and refresh Backlog data")
	fmt.Println("  -list                        List available analyzers")
	fmt.Println("  -help                        Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  dev-stats -analyzer github")
	fmt.Println("  dev-stats -analyzer github,backlog")
	fmt.Println("  dev-stats -analyzer all")
	fmt.Println("  dev-stats -download notion-urls/YYYY-MM-DD_to_YYYY-MM-DD.md")
	fmt.Println("  dev-stats -download-google")
	fmt.Println("  dev-stats -list-backlog-profiles")
	fmt.Println("  dev-stats -list-backlog")
	fmt.Println("  dev-stats -list-backlog-clear")
	fmt.Println("  dev-stats -list-backlog-project 1073924896")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  START_DATE         Start date in YYYY-MM-DD format")
	fmt.Println("  END_DATE           End date in YYYY-MM-DD format")
	fmt.Println()
	fmt.Println("  For GitHub:")
	fmt.Println("    GITHUB_TOKEN     GitHub personal access token")
	fmt.Println("    GITHUB_USERNAME  GitHub username")
	fmt.Println()
	fmt.Println("  For Backlog (Multi-Profile Support):")
	fmt.Println("    Pattern: BACKLOG_<PROFILE>_<SETTING>")
	fmt.Println()
	fmt.Println("    BACKLOG_<PROFILE>_API_KEY       Backlog API key")
	fmt.Println("    BACKLOG_<PROFILE>_HOST          Backlog host (e.g., mycompany.backlog.com)")
	fmt.Println("    BACKLOG_<PROFILE>_USER_ID       User ID (for analysis)")
	fmt.Println("    BACKLOG_<PROFILE>_PROJECT_ID    Project ID (for analysis)")
	fmt.Println()
	fmt.Println("    Example for multiple profiles:")
	fmt.Println("      BACKLOG_HOGE_API_KEY=xxx")
	fmt.Println("      BACKLOG_HOGE_HOST=mycompany.backlog.com")
	fmt.Println("      BACKLOG_HOGE_USER_ID=123456")
	fmt.Println("      BACKLOG_HOGE_PROJECT_ID=789012")
	fmt.Println()
	fmt.Println("      BACKLOG_FUGA_API_KEY=yyy")
	fmt.Println("      BACKLOG_FUGA_HOST=projectspace.backlog.jp")
	fmt.Println("      BACKLOG_FUGA_USER_ID=234567")
	fmt.Println("      BACKLOG_FUGA_PROJECT_ID=890123")
	fmt.Println()
	fmt.Println("  For Calendar:")
	fmt.Println("    No additional environment variables required")
	fmt.Println("    (Reads ICS files from storage/calendar directory)")
	fmt.Println()
	fmt.Println("  For Notion:")
	fmt.Println("    NOTION_TOKEN        Notion integration token")
	fmt.Println("    NOTION_USER_ID      (Optional) Specific user ID to filter pages by")
	fmt.Println()
	fmt.Println("  For Google Workspace:")
	fmt.Println("    GOOGLE_CLIENT_ID     OAuth2 client ID (from GCP Console)")
	fmt.Println("    GOOGLE_CLIENT_SECRET OAuth2 client secret")
	fmt.Println("    GOOGLE_TOKEN_FILE    (Optional) Token cache path (default: storage/google_token.json)")
}

func printAvailableAnalyzers() {
	fmt.Println("Available analyzers:")
	fmt.Println("  github   - GitHub pull request analysis")
	fmt.Println("  backlog  - Backlog issue and activity analysis")
	fmt.Println("  calendar - Calendar event analysis")
	fmt.Println("  notion   - Notion page analysis")
	fmt.Println("  google   - Google Workspace activity analysis (Docs/Slides/Sheets)")
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
