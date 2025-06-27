package calendar

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dev-stats/pkg/common"
)

// CalendarAnalyzer implements the Analyzer interface for Calendar
type CalendarAnalyzer struct {
	calendarDir string
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

// NewCalendarAnalyzer creates a new Calendar analyzer
func NewCalendarAnalyzer() *CalendarAnalyzer {
	return &CalendarAnalyzer{
		calendarDir: "storage/calendar",
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
func (c *CalendarAnalyzer) Analyze(config *common.Config) (*common.AnalysisResult, error) {
	if err := c.ValidateConfig(); err != nil {
		return nil, err
	}

	fmt.Printf("Analyzing calendar events from directory: %s\n", c.calendarDir)

	// Read all ICS files
	allEvents, err := c.readAllICSFiles()
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

	// Create result
	result := &common.AnalysisResult{
		AnalyzerName: c.GetName(),
		StartDate:    config.StartDate,
		EndDate:      config.EndDate,
		Summary: map[string]interface{}{
			"Total events":   len(filteredEvents),
			"Total duration": totalDuration,
			"Event titles":   len(groupedByTitle),
			"All-day events": len(allDayStats),
		},
		Details: map[string]interface{}{
			"events":        filteredEvents,
			"title_stats":   titleStats,
			"all_day_stats": allDayStats,
		},
	}

	c.printResults(result, filteredEvents, titleStats, allDayStats)
	return result, nil
}

func (c *CalendarAnalyzer) readAllICSFiles() ([]Event, error) {
	var allEvents []Event

	err := filepath.Walk(c.calendarDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".ics") {
			fmt.Printf("Reading calendar file: %s\n", path)
			events, err := c.parseICSFile(path)
			if err != nil {
				fmt.Printf("Error parsing ICS file %s: %v\n", path, err)
				fmt.Printf("Continuing with other files...\n")
				return nil
			}
			fmt.Printf("Successfully parsed %d events from %s\n", len(events), path)
			allEvents = append(allEvents, events...)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	fmt.Printf("\nTotal events parsed from all files: %d\n", len(allEvents))
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
		if !event.Start.IsZero() && event.Start.After(startDate) && event.Start.Before(endDate.AddDate(0, 0, 1)) {
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
	for title, events := range groupedEvents {
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
	for title, events := range groupedEvents {
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

func (c *CalendarAnalyzer) printResults(result *common.AnalysisResult, events []Event, titleStats, allDayStats []TitleStats) {
	fmt.Printf("\nCalendar events from %s to %s:\n",
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
		fmt.Printf("- %s: %s%s\n", event.Start.Format("2006-01-02 15:04"), event.Summary, duration)
	}

	result.PrintSummary()

	// Print title statistics
	fmt.Println("\nTop events by count:")
	sortedByCount := make([]TitleStats, len(titleStats))
	copy(sortedByCount, titleStats)
	sort.Slice(sortedByCount, func(i, j int) bool {
		return sortedByCount[i].Count > sortedByCount[j].Count
	})

	for i, stat := range sortedByCount {
		hours := int(stat.Duration.Hours())
		minutes := int(stat.Duration.Minutes()) % 60
		durationStr := ""
		if hours > 0 || minutes > 0 {
			durationStr = fmt.Sprintf(" (%dh%dm)", hours, minutes)
		}
		fmt.Printf("%2d. %s: %d events%s\n", i+1, stat.Title, stat.Count, durationStr)
	}

	// Print duration statistics
	fmt.Println("\nTop events by total duration:")
	sortedByDuration := make([]TitleStats, len(titleStats))
	copy(sortedByDuration, titleStats)
	sort.Slice(sortedByDuration, func(i, j int) bool {
		return sortedByDuration[i].Duration > sortedByDuration[j].Duration
	})

	for i, stat := range sortedByDuration {
		hours := int(stat.Duration.Hours())
		minutes := int(stat.Duration.Minutes()) % 60
		if hours > 0 || minutes > 0 {
			durationStr := fmt.Sprintf("%dh%dm", hours, minutes)
			fmt.Printf("%2d. %s: %s (%d events)\n", i+1, stat.Title, durationStr, stat.Count)
		}
	}

	// Print all-day event statistics
	if len(allDayStats) > 0 {
		fmt.Println("\nAll-day events ranking by total days:")
		sortedByDays := make([]TitleStats, len(allDayStats))
		copy(sortedByDays, allDayStats)
		sort.Slice(sortedByDays, func(i, j int) bool {
			return sortedByDays[i].Duration > sortedByDays[j].Duration
		})

		for i, stat := range sortedByDays {
			totalDays := int(stat.Duration.Hours() / 24)
			fmt.Printf("%2d. %s: %d days (%d events)\n", i+1, stat.Title, totalDays, stat.Count)
		}
	}
}
