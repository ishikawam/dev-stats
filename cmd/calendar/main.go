package main

import (
	"bufio"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Event struct {
	UID      string
	Summary  string
	Start    time.Time
	End      time.Time
	Created  time.Time
	IsAllDay bool
}

func parseICSFile(filePath string) ([]Event, error) {
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
				dtStart := extractDateTime(line)
				// Check if this is an all-day event (VALUE=DATE format)
				if strings.Contains(line, "VALUE=DATE") {
					currentEvent.IsAllDay = true
				}
				if t, err := parseDateTime(dtStart); err == nil {
					currentEvent.Start = t
				} else {
					log.Printf("Warning: Failed to parse DTSTART '%s' (extracted: '%s'): %v", line, dtStart, err)
				}
			} else if strings.HasPrefix(line, "DTEND") {
				dtEnd := extractDateTime(line)
				if t, err := parseDateTime(dtEnd); err == nil {
					currentEvent.End = t
				} else {
					log.Printf("Warning: Failed to parse DTEND '%s' (extracted: '%s'): %v", line, dtEnd, err)
				}
			} else if strings.HasPrefix(line, "CREATED:") {
				created := strings.TrimPrefix(line, "CREATED:")
				if t, err := parseDateTime(created); err == nil {
					currentEvent.Created = t
				} else {
					log.Printf("Warning: Failed to parse CREATED '%s': %v", created, err)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading ICS file %s: %v", filePath, err)
		return events, err
	}
	return events, nil
}

func extractDateTime(line string) string {
	// Handle lines like "DTSTART;TZID=Asia/Tokyo:20230519T120000", "DTSTART:20230519T120000Z", or "DTSTART;VALUE=DATE:20140220"
	colonIndex := strings.LastIndex(line, ":")
	if colonIndex != -1 {
		return line[colonIndex+1:]
	}
	log.Printf("Warning: Could not extract datetime from line: %s", line)
	return ""
}

func parseDateTime(dtStr string) (time.Time, error) {
	if dtStr == "" {
		return time.Time{}, fmt.Errorf("empty datetime string")
	}

	// ICS datetime format: YYYYMMDDTHHMMSSZ
	if len(dtStr) >= 15 && strings.HasSuffix(dtStr, "Z") {
		t, err := time.Parse("20060102T150405Z", dtStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse UTC format '%s': %v", dtStr, err)
		}
		return t, nil
	}
	// Try without timezone
	if len(dtStr) >= 15 && strings.Contains(dtStr, "T") {
		t, err := time.Parse("20060102T150405", dtStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse local format '%s': %v", dtStr, err)
		}
		return t, nil
	}
	// Date only format: YYYYMMDD
	if len(dtStr) == 8 {
		t, err := time.Parse("20060102", dtStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse date format '%s': %v", dtStr, err)
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported datetime format: '%s' (length: %d)", dtStr, len(dtStr))
}

func filterEventsByDateRange(events []Event, startDate, endDate time.Time) []Event {
	var filtered []Event
	for _, event := range events {
		if !event.Start.IsZero() && event.Start.After(startDate) && event.Start.Before(endDate.AddDate(0, 0, 1)) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func calculateDuration(events []Event) time.Duration {
	var totalDuration time.Duration
	for _, event := range events {
		if !event.Start.IsZero() && !event.End.IsZero() {
			duration := event.End.Sub(event.Start)
			if duration > 0 {
				totalDuration += duration
			}
		}
	}
	return totalDuration
}

func isAllDayEvent(event Event) bool {
	// Already marked as all-day (VALUE=DATE format)
	if event.IsAllDay {
		return true
	}

	// Check if it's a 24-hour event (common pattern for all-day events)
	if !event.Start.IsZero() && !event.End.IsZero() {
		duration := event.End.Sub(event.Start)
		// 24-hour events are often all-day events
		if duration == 24*time.Hour {
			return true
		}
		// Some all-day events might be longer (multi-day)
		hours := int(duration.Hours())
		if hours >= 24 && hours%24 == 0 {
			return true
		}
	}

	return false
}

func calculateDurationExcludingAllDay(events []Event) time.Duration {
	var totalDuration time.Duration
	for _, event := range events {
		if !isAllDayEvent(event) && !event.Start.IsZero() && !event.End.IsZero() {
			duration := event.End.Sub(event.Start)
			if duration > 0 {
				totalDuration += duration
			}
		}
	}
	return totalDuration
}

func groupEventsByTitle(events []Event) map[string][]Event {
	grouped := make(map[string][]Event)
	for _, event := range events {
		title := strings.TrimSpace(event.Summary)
		if title != "" {
			grouped[title] = append(grouped[title], event)
		}
	}
	return grouped
}

type TitleStats struct {
	Title    string
	Count    int
	Duration time.Duration
}

func calculateTitleStats(groupedEvents map[string][]Event) []TitleStats {
	var stats []TitleStats
	for title, events := range groupedEvents {
		durationExcludingAllDay := calculateDurationExcludingAllDay(events)
		stats = append(stats, TitleStats{
			Title:    title,
			Count:    len(events),
			Duration: durationExcludingAllDay, // Use duration excluding all-day events
		})
	}
	return stats
}

func calculateAllDayStats(groupedEvents map[string][]Event) []TitleStats {
	var stats []TitleStats
	for title, events := range groupedEvents {
		allDayCount := 0
		totalDays := 0

		for _, event := range events {
			if isAllDayEvent(event) {
				allDayCount++
				if !event.Start.IsZero() && !event.End.IsZero() {
					duration := event.End.Sub(event.Start)
					days := int(duration.Hours() / 24)
					if days > 0 {
						totalDays += days
					} else {
						totalDays += 1 // At least 1 day for all-day events
					}
				} else {
					totalDays += 1 // Default to 1 day if no duration info
				}
			}
		}

		if allDayCount > 0 {
			stats = append(stats, TitleStats{
				Title:    title,
				Count:    allDayCount,
				Duration: time.Duration(totalDays*24) * time.Hour, // Convert days to duration for sorting
			})
		}
	}
	return stats
}

func main() {
	godotenv.Load()

	startDateStr := os.Getenv("START_DATE")
	endDateStr := os.Getenv("END_DATE")

	if startDateStr == "" || endDateStr == "" {
		log.Fatalf("Environment variables START_DATE and END_DATE must be set.")
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		log.Fatalf("Invalid START_DATE format: %v", err)
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		log.Fatalf("Invalid END_DATE format: %v", err)
	}

	calendarDir := "storage/calendar"
	var allEvents []Event

	// Read all ICS files in the calendar directory
	err = filepath.Walk(calendarDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".ics") {
			fmt.Printf("Reading calendar file: %s\n", path)
			events, err := parseICSFile(path)
			if err != nil {
				log.Printf("Error parsing ICS file %s: %v", path, err)
				log.Printf("Continuing with other files...")
				return nil
			}
			fmt.Printf("Successfully parsed %d events from %s\n", len(events), path)
			allEvents = append(allEvents, events...)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error reading calendar directory: %v", err)
	}

	fmt.Printf("\nTotal events parsed from all files: %d\n", len(allEvents))

	// Filter events by date range
	filteredEvents := filterEventsByDateRange(allEvents, startDate, endDate)

	// Sort events by start time
	sort.Slice(filteredEvents, func(i, j int) bool {
		return filteredEvents[i].Start.Before(filteredEvents[j].Start)
	})

	// Calculate statistics
	totalDuration := calculateDuration(filteredEvents)
	groupedByTitle := groupEventsByTitle(filteredEvents)
	titleStats := calculateTitleStats(groupedByTitle)
	allDayStats := calculateAllDayStats(groupedByTitle)

	// Output detailed events
	fmt.Printf("\nCalendar events from %s to %s:\n", startDateStr, endDateStr)
	for _, event := range filteredEvents {
		duration := ""
		if isAllDayEvent(event) {
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

	// Output summary
	fmt.Printf("\nCalendar summary from %s to %s:\n", startDateStr, endDateStr)
	fmt.Printf("Total events: %d\n", len(filteredEvents))

	if totalDuration > 0 {
		totalHours := int(totalDuration.Hours())
		totalMinutes := int(totalDuration.Minutes()) % 60
		fmt.Printf("Total duration: %dh%dm\n", totalHours, totalMinutes)
	}

	// Events by title ranking
	fmt.Println("\nEvents by title ranking:")

	// Sort by count (descending)
	sortedByCount := make([]TitleStats, len(titleStats))
	copy(sortedByCount, titleStats)
	sort.Slice(sortedByCount, func(i, j int) bool {
		return sortedByCount[i].Count > sortedByCount[j].Count
	})

	fmt.Println("\nTop events by count (all):")
	for i, stat := range sortedByCount {
		hours := int(stat.Duration.Hours())
		minutes := int(stat.Duration.Minutes()) % 60
		durationStr := ""
		if hours > 0 || minutes > 0 {
			durationStr = fmt.Sprintf(" (%dh%dm)", hours, minutes)
		}
		fmt.Printf("%2d. %s: %d events%s\n", i+1, stat.Title, stat.Count, durationStr)
	}

	// Sort by duration (descending)
	sortedByDuration := make([]TitleStats, len(titleStats))
	copy(sortedByDuration, titleStats)
	sort.Slice(sortedByDuration, func(i, j int) bool {
		return sortedByDuration[i].Duration > sortedByDuration[j].Duration
	})

	fmt.Println("\nTop events by total duration (all):")
	for i, stat := range sortedByDuration {
		hours := int(stat.Duration.Hours())
		minutes := int(stat.Duration.Minutes()) % 60
		durationStr := ""
		if hours > 0 || minutes > 0 {
			durationStr = fmt.Sprintf("%dh%dm", hours, minutes)
		} else {
			continue // Skip events with no duration
		}
		fmt.Printf("%2d. %s: %s (%d events)\n", i+1, stat.Title, durationStr, stat.Count)
	}

	// All-day events ranking
	if len(allDayStats) > 0 {
		// Sort by total days (descending)
		sortedByDays := make([]TitleStats, len(allDayStats))
		copy(sortedByDays, allDayStats)
		sort.Slice(sortedByDays, func(i, j int) bool {
			return sortedByDays[i].Duration > sortedByDays[j].Duration
		})

		fmt.Println("\nAll-day events ranking by total days (all):")
		for i, stat := range sortedByDays {
			totalDays := int(stat.Duration.Hours() / 24)
			fmt.Printf("%2d. %s: %d days (%d events)\n", i+1, stat.Title, totalDays, stat.Count)
		}
	}
}
