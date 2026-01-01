package calendar

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dev-stats/pkg/common"
	"dev-stats/pkg/config"
)

// CalendarAnalyzer implements the Analyzer interface for Calendar
type CalendarAnalyzer struct {
	calendarDir    string
	categoryConfig *config.CategorizationConfig
}

// Event represents a calendar event
type Event struct {
	UID      string
	Summary  string
	Start    time.Time
	End      time.Time
	Created  time.Time
	IsAllDay bool
}

// TitleStats represents statistics for events by title
type TitleStats struct {
	Title    string
	Count    int
	Duration time.Duration
}

// EventCategoryStats represents statistics by event category
type EventCategoryStats struct {
	Categories   map[string]*CategoryInfo `json:"categories"`
	MeetingTime  time.Duration            `json:"meeting_time"`
	FocusTime    time.Duration            `json:"focus_time"`
	LearningTime time.Duration            `json:"learning_time"`
	AdminTime    time.Duration            `json:"admin_time"`
}

// CategoryInfo contains details about a specific category
type CategoryInfo struct {
	Count    int           `json:"count"`
	Duration time.Duration `json:"duration"`
	Events   []Event       `json:"events"`
}

// WorkingHoursStats represents analysis of working hours patterns
type WorkingHoursStats struct {
	HourlyDistribution map[int]time.Duration    `json:"hourly_distribution"`
	DailyDistribution  map[string]time.Duration `json:"daily_distribution"`
	PeakHours          []int                    `json:"peak_hours"`
	TotalWorkingHours  time.Duration            `json:"total_working_hours"`
}

// NewCalendarAnalyzer creates a new Calendar analyzer
func NewCalendarAnalyzer() *CalendarAnalyzer {
	// Load category configuration
	categoryConfig, err := config.LoadCategorizationConfig("")
	if err != nil {
		// Return nil to indicate initialization failure
		// The caller should handle this error
		fmt.Printf("Error: Failed to load category config: %v\n", err)
		return nil
	}

	return &CalendarAnalyzer{
		calendarDir:    "storage/calendar",
		categoryConfig: categoryConfig,
	}
}

// GetName returns the analyzer name
func (c *CalendarAnalyzer) GetName() string {
	return "Calendar"
}

// ValidateConfig validates the required configuration
func (c *CalendarAnalyzer) ValidateConfig() error {
	if _, err := os.Stat(c.calendarDir); os.IsNotExist(err) {
		return common.NewError("Calendar directory '%s' does not exist", c.calendarDir)
	}
	return nil
}

// Analyze performs Calendar analysis
func (c *CalendarAnalyzer) Analyze(config *common.Config, writer io.Writer) (*common.AnalysisResult, error) {
	if err := c.ValidateConfig(); err != nil {
		return nil, err
	}

	fmt.Fprintf(writer, "Analyzing calendar events from directory: %s\n", c.calendarDir)

	// Read all ICS files
	allEvents, err := c.readAllICSFiles(writer)
	if err != nil {
		return nil, common.WrapError(err, "failed to read ICS files")
	}

	// Filter events by date range
	filteredEvents := c.filterEventsByDateRange(allEvents, config.StartDate, config.EndDate)

	// Sort events by start time
	sort.Slice(filteredEvents, func(i, j int) bool {
		return filteredEvents[i].Start.Before(filteredEvents[j].Start)
	})

	// Calculate statistics
	totalDuration := c.calculateDuration(filteredEvents)
	groupedByTitle := c.groupEventsByTitle(filteredEvents)
	titleStats := c.calculateTitleStats(groupedByTitle)
	allDayStats := c.calculateAllDayStats(groupedByTitle)

	// Enhanced analysis
	categoryStats := c.analyzeCategoryStats(filteredEvents)
	workingHoursStats := c.analyzeWorkingHours(filteredEvents)

	// Create result
	result := &common.AnalysisResult{
		AnalyzerName: c.GetName(),
		StartDate:    config.StartDate,
		EndDate:      config.EndDate,
		Summary: map[string]interface{}{
			"Total events":        len(filteredEvents),
			"Total duration":      totalDuration,
			"Event titles":        len(groupedByTitle),
			"All-day events":      len(allDayStats),
			"Meeting time":        categoryStats.MeetingTime,
			"Focus time":          categoryStats.FocusTime,
			"Learning time":       categoryStats.LearningTime,
			"Admin time":          categoryStats.AdminTime,
			"Total working hours": workingHoursStats.TotalWorkingHours,
			"Event categories":    len(categoryStats.Categories),
		},
		Details: map[string]interface{}{
			"events":         filteredEvents,
			"title_stats":    titleStats,
			"all_day_stats":  allDayStats,
			"category_stats": categoryStats,
			"working_hours":  workingHoursStats,
		},
	}

	c.printResults(writer, result, filteredEvents, titleStats, allDayStats, categoryStats, workingHoursStats)
	return result, nil
}

func (c *CalendarAnalyzer) readAllICSFiles(writer io.Writer) ([]Event, error) {
	var allEvents []Event

	err := filepath.Walk(c.calendarDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".ics") {
			fmt.Fprintf(writer, "Reading calendar file: %s\n", path)
			events, err := c.parseICSFile(path)
			if err != nil {
				fmt.Fprintf(writer, "Error parsing ICS file %s: %v\n", path, err)
				fmt.Fprintf(writer, "Continuing with other files...\n")
				return nil
			}
			fmt.Fprintf(writer, "Successfully parsed %d events from %s\n", len(events), path)
			allEvents = append(allEvents, events...)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	fmt.Fprintf(writer, "\nTotal events parsed from all files: %d\n", len(allEvents))
	return allEvents, nil
}

func (c *CalendarAnalyzer) parseICSFile(filePath string) ([]Event, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []Event
	var currentEvent Event
	inEvent := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "BEGIN:VEVENT" {
			inEvent = true
			currentEvent = Event{}
		} else if line == "END:VEVENT" {
			if inEvent {
				events = append(events, currentEvent)
			}
			inEvent = false
		} else if inEvent {
			if strings.HasPrefix(line, "UID:") {
				currentEvent.UID = strings.TrimPrefix(line, "UID:")
			} else if strings.HasPrefix(line, "SUMMARY:") {
				currentEvent.Summary = strings.TrimPrefix(line, "SUMMARY:")
			} else if strings.HasPrefix(line, "DTSTART") {
				dtStart := c.extractDateTime(line)
				if strings.Contains(line, "VALUE=DATE") {
					currentEvent.IsAllDay = true
				}
				if t, err := c.parseDateTime(dtStart); err == nil {
					currentEvent.Start = t
				}
			} else if strings.HasPrefix(line, "DTEND") {
				dtEnd := c.extractDateTime(line)
				if t, err := c.parseDateTime(dtEnd); err == nil {
					currentEvent.End = t
				}
			} else if strings.HasPrefix(line, "CREATED:") {
				created := strings.TrimPrefix(line, "CREATED:")
				if t, err := c.parseDateTime(created); err == nil {
					currentEvent.Created = t
				}
			}
		}
	}

	return events, scanner.Err()
}

func (c *CalendarAnalyzer) extractDateTime(line string) string {
	colonIndex := strings.LastIndex(line, ":")
	if colonIndex != -1 {
		return line[colonIndex+1:]
	}
	return ""
}

func (c *CalendarAnalyzer) parseDateTime(dtStr string) (time.Time, error) {
	if dtStr == "" {
		return time.Time{}, fmt.Errorf("empty datetime string")
	}

	// ICS datetime format: YYYYMMDDTHHMMSSZ
	if len(dtStr) >= 15 && strings.HasSuffix(dtStr, "Z") {
		return time.Parse("20060102T150405Z", dtStr)
	}
	// Try without timezone
	if len(dtStr) >= 15 && strings.Contains(dtStr, "T") {
		return time.Parse("20060102T150405", dtStr)
	}
	// Date only format: YYYYMMDD
	if len(dtStr) == 8 {
		return time.Parse("20060102", dtStr)
	}
	return time.Time{}, fmt.Errorf("unsupported datetime format: '%s'", dtStr)
}

func (c *CalendarAnalyzer) filterEventsByDateRange(events []Event, startDate, endDate time.Time) []Event {
	var filtered []Event
	for _, event := range events {
		// Include events on or after startDate and before or on endDate
		if !event.Start.IsZero() && !event.Start.Before(startDate) && event.Start.Before(endDate.AddDate(0, 0, 1)) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func (c *CalendarAnalyzer) calculateDuration(events []Event) time.Duration {
	var totalDuration time.Duration
	for _, event := range events {
		if !c.isAllDayEvent(event) && !event.Start.IsZero() && !event.End.IsZero() {
			duration := event.End.Sub(event.Start)
			if duration > 0 {
				totalDuration += duration
			}
		}
	}
	return totalDuration
}

func (c *CalendarAnalyzer) isAllDayEvent(event Event) bool {
	if event.IsAllDay {
		return true
	}

	if !event.Start.IsZero() && !event.End.IsZero() {
		duration := event.End.Sub(event.Start)
		if duration == 24*time.Hour {
			return true
		}
		hours := int(duration.Hours())
		if hours >= 24 && hours%24 == 0 {
			return true
		}
	}

	return false
}

func (c *CalendarAnalyzer) groupEventsByTitle(events []Event) map[string][]Event {
	grouped := make(map[string][]Event)
	for _, event := range events {
		title := strings.TrimSpace(event.Summary)
		if title != "" {
			grouped[title] = append(grouped[title], event)
		}
	}
	return grouped
}

func (c *CalendarAnalyzer) calculateTitleStats(groupedEvents map[string][]Event) []TitleStats {
	var stats []TitleStats

	// Sort titles for deterministic output
	var titles []string
	for title := range groupedEvents {
		titles = append(titles, title)
	}
	sort.Strings(titles)

	for _, title := range titles {
		events := groupedEvents[title]
		var duration time.Duration
		for _, event := range events {
			if !c.isAllDayEvent(event) && !event.Start.IsZero() && !event.End.IsZero() {
				d := event.End.Sub(event.Start)
				if d > 0 {
					duration += d
				}
			}
		}
		stats = append(stats, TitleStats{
			Title:    title,
			Count:    len(events),
			Duration: duration,
		})
	}
	return stats
}

func (c *CalendarAnalyzer) calculateAllDayStats(groupedEvents map[string][]Event) []TitleStats {
	var stats []TitleStats

	// Sort titles for deterministic output
	var titles []string
	for title := range groupedEvents {
		titles = append(titles, title)
	}
	sort.Strings(titles)

	for _, title := range titles {
		events := groupedEvents[title]
		allDayCount := 0
		totalDays := 0

		for _, event := range events {
			if c.isAllDayEvent(event) {
				allDayCount++
				if !event.Start.IsZero() && !event.End.IsZero() {
					duration := event.End.Sub(event.Start)
					days := int(duration.Hours() / 24)
					if days > 0 {
						totalDays += days
					} else {
						totalDays += 1
					}
				} else {
					totalDays += 1
				}
			}
		}

		if allDayCount > 0 {
			stats = append(stats, TitleStats{
				Title:    title,
				Count:    allDayCount,
				Duration: time.Duration(totalDays*24) * time.Hour,
			})
		}
	}
	return stats
}

func (c *CalendarAnalyzer) printResults(writer io.Writer, result *common.AnalysisResult, events []Event, titleStats, allDayStats []TitleStats, categoryStats *EventCategoryStats, workingHoursStats *WorkingHoursStats) {
	fmt.Fprintf(writer, "\nCalendar events from %s to %s:\n",
		result.StartDate.Format("2006-01-02"),
		result.EndDate.Format("2006-01-02"))

	for _, event := range events {
		duration := ""
		if c.isAllDayEvent(event) {
			duration = " (-)"
		} else if !event.Start.IsZero() && !event.End.IsZero() {
			d := event.End.Sub(event.Start)
			hours := int(d.Hours())
			minutes := int(d.Minutes()) % 60
			if hours > 0 {
				duration = fmt.Sprintf(" (%dh%dm)", hours, minutes)
			} else {
				duration = fmt.Sprintf(" (%dm)", minutes)
			}
		}
		fmt.Fprintf(writer, "- %s: %s%s\n", event.Start.Format("2006-01-02 15:04"), event.Summary, duration)
	}

	result.PrintSummary(writer)

	// Print title statistics
	fmt.Fprintln(writer, "\nTop events by count:")
	sortedByCount := make([]TitleStats, len(titleStats))
	copy(sortedByCount, titleStats)
	sort.Slice(sortedByCount, func(i, j int) bool {
		if sortedByCount[i].Count == sortedByCount[j].Count {
			return sortedByCount[i].Title < sortedByCount[j].Title
		}
		return sortedByCount[i].Count > sortedByCount[j].Count
	})

	for i, stat := range sortedByCount {
		hours := int(stat.Duration.Hours())
		minutes := int(stat.Duration.Minutes()) % 60
		durationStr := ""
		if hours > 0 || minutes > 0 {
			durationStr = fmt.Sprintf(" (%dh%dm)", hours, minutes)
		}
		fmt.Fprintf(writer, "%2d. %s: %d events%s\n", i+1, stat.Title, stat.Count, durationStr)
	}

	// Print duration statistics
	fmt.Fprintln(writer, "\nTop events by total duration:")
	sortedByDuration := make([]TitleStats, len(titleStats))
	copy(sortedByDuration, titleStats)
	sort.Slice(sortedByDuration, func(i, j int) bool {
		if sortedByDuration[i].Duration == sortedByDuration[j].Duration {
			return sortedByDuration[i].Title < sortedByDuration[j].Title
		}
		return sortedByDuration[i].Duration > sortedByDuration[j].Duration
	})

	for i, stat := range sortedByDuration {
		hours := int(stat.Duration.Hours())
		minutes := int(stat.Duration.Minutes()) % 60
		if hours > 0 || minutes > 0 {
			durationStr := fmt.Sprintf("%dh%dm", hours, minutes)
			fmt.Fprintf(writer, "%2d. %s: %s (%d events)\n", i+1, stat.Title, durationStr, stat.Count)
		}
	}

	// Print all-day event statistics
	if len(allDayStats) > 0 {
		fmt.Fprintln(writer, "\nAll-day events ranking by total days:")
		sortedByDays := make([]TitleStats, len(allDayStats))
		copy(sortedByDays, allDayStats)
		sort.Slice(sortedByDays, func(i, j int) bool {
			if sortedByDays[i].Duration == sortedByDays[j].Duration {
				return sortedByDays[i].Title < sortedByDays[j].Title
			}
			return sortedByDays[i].Duration > sortedByDays[j].Duration
		})

		for i, stat := range sortedByDays {
			totalDays := int(stat.Duration.Hours() / 24)
			fmt.Fprintf(writer, "%2d. %s: %d days (%d events)\n", i+1, stat.Title, totalDays, stat.Count)
		}
	}

	// Print enhanced category analysis
	fmt.Fprintln(writer, "\nWork Category Analysis:")
	fmt.Fprintf(writer, "- Meeting time: %s\n", c.formatDuration(categoryStats.MeetingTime))
	fmt.Fprintf(writer, "- Focus time: %s\n", c.formatDuration(categoryStats.FocusTime))
	fmt.Fprintf(writer, "- Learning time: %s\n", c.formatDuration(categoryStats.LearningTime))
	fmt.Fprintf(writer, "- Admin time: %s\n", c.formatDuration(categoryStats.AdminTime))

	fmt.Fprintln(writer, "\nWorking Hours Analysis:")
	fmt.Fprintf(writer, "- Total working hours: %s\n", c.formatDuration(workingHoursStats.TotalWorkingHours))
	if len(workingHoursStats.PeakHours) > 0 {
		fmt.Fprintf(writer, "- Peak activity hours: ")
		for i, hour := range workingHoursStats.PeakHours {
			if i > 0 {
				fmt.Fprint(writer, ", ")
			}
			fmt.Fprintf(writer, "%02d:00", hour)
		}
		fmt.Fprintln(writer)
	}
}

// analyzeCategoryStats categorizes events based on their titles and calculates time spent
func (c *CalendarAnalyzer) analyzeCategoryStats(events []Event) *EventCategoryStats {
	stats := &EventCategoryStats{
		Categories: make(map[string]*CategoryInfo),
	}

	for _, event := range events {
		if event.IsAllDay {
			continue // Skip all-day events for time-based analysis
		}

		duration := event.End.Sub(event.Start)
		title := strings.ToLower(event.Summary)

		// Categorize events
		category := c.categorizeEvent(title)

		if stats.Categories[category] == nil {
			stats.Categories[category] = &CategoryInfo{
				Events: make([]Event, 0),
			}
		}

		stats.Categories[category].Count++
		stats.Categories[category].Duration += duration
		stats.Categories[category].Events = append(stats.Categories[category].Events, event)

		// Update main category totals using configuration
		categoryType := c.categoryConfig.GetCategoryTime(title)
		switch categoryType {
		case "meeting":
			stats.MeetingTime += duration
		case "focus":
			stats.FocusTime += duration
		case "learning":
			stats.LearningTime += duration
		case "admin":
			stats.AdminTime += duration
		}
	}

	return stats
}

// categorizeEvent determines the category of an event based on its title
func (c *CalendarAnalyzer) categorizeEvent(title string) string {
	category := c.categoryConfig.CategorizeByKeywords(title)

	// Convert category names to display format for compatibility
	switch category {
	case "1on1 meetings":
		return "1on1 Meetings"
	case "daily standups":
		return "Daily Standups"
	case "regular meetings":
		return "Regular Meetings"
	case "general meetings":
		return "General Meetings"
	case "focus work":
		return "Focus Work"
	case "technical consultation":
		return "Technical Consultation"
	case "learning & training":
		return "Learning & Training"
	case "time off":
		return "Time Off"
	case "meeting":
		return "General Meetings"
	case "focus":
		return "Focus Work"
	case "learning":
		return "Learning & Training"
	case "admin":
		return "Admin Work"
	default:
		return "Other"
	}
}

// analyzeWorkingHours analyzes patterns in working hours
func (c *CalendarAnalyzer) analyzeWorkingHours(events []Event) *WorkingHoursStats {
	stats := &WorkingHoursStats{
		HourlyDistribution: make(map[int]time.Duration),
		DailyDistribution:  make(map[string]time.Duration),
	}

	for _, event := range events {
		if event.IsAllDay {
			continue
		}

		duration := event.End.Sub(event.Start)
		hour := event.Start.Hour()
		day := event.Start.Weekday().String()

		stats.HourlyDistribution[hour] += duration
		stats.DailyDistribution[day] += duration
		stats.TotalWorkingHours += duration
	}

	// Find peak hours (top 3 hours with most activity)
	type hourData struct {
		hour     int
		duration time.Duration
	}

	var hourList []hourData
	for hour, duration := range stats.HourlyDistribution {
		hourList = append(hourList, hourData{hour, duration})
	}

	sort.Slice(hourList, func(i, j int) bool {
		if hourList[i].duration == hourList[j].duration {
			return hourList[i].hour < hourList[j].hour
		}
		return hourList[i].duration > hourList[j].duration
	})

	// Take top 3 peak hours
	for i := 0; i < len(hourList) && i < 3; i++ {
		stats.PeakHours = append(stats.PeakHours, hourList[i].hour)
	}

	return stats
}

// formatDuration formats duration in a human-readable way
func (c *CalendarAnalyzer) formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
