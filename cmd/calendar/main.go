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
	UID     string
	Summary string
	Start   time.Time
	End     time.Time
	Created time.Time
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
			} else if strings.HasPrefix(line, "DTSTART:") {
				dtStart := strings.TrimPrefix(line, "DTSTART:")
				if t, err := parseDateTime(dtStart); err == nil {
					currentEvent.Start = t
				}
			} else if strings.HasPrefix(line, "DTEND:") {
				dtEnd := strings.TrimPrefix(line, "DTEND:")
				if t, err := parseDateTime(dtEnd); err == nil {
					currentEvent.End = t
				}
			} else if strings.HasPrefix(line, "CREATED:") {
				created := strings.TrimPrefix(line, "CREATED:")
				if t, err := parseDateTime(created); err == nil {
					currentEvent.Created = t
				}
			}
		}
	}

	return events, scanner.Err()
}

func parseDateTime(dtStr string) (time.Time, error) {
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
	return time.Time{}, fmt.Errorf("unsupported datetime format: %s", dtStr)
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

func groupEventsByDate(events []Event) map[string][]Event {
	grouped := make(map[string][]Event)
	for _, event := range events {
		if !event.Start.IsZero() {
			dateKey := event.Start.Format("2006-01-02")
			grouped[dateKey] = append(grouped[dateKey], event)
		}
	}
	return grouped
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
				log.Printf("Error parsing %s: %v", path, err)
				return nil
			}
			allEvents = append(allEvents, events...)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error reading calendar directory: %v", err)
	}

	// Filter events by date range
	filteredEvents := filterEventsByDateRange(allEvents, startDate, endDate)

	// Sort events by start time
	sort.Slice(filteredEvents, func(i, j int) bool {
		return filteredEvents[i].Start.Before(filteredEvents[j].Start)
	})

	// Calculate statistics
	totalDuration := calculateDuration(filteredEvents)
	groupedByDate := groupEventsByDate(filteredEvents)

	// Output detailed events
	fmt.Printf("\nCalendar events from %s to %s:\n", startDateStr, endDateStr)
	for _, event := range filteredEvents {
		duration := ""
		if !event.Start.IsZero() && !event.End.IsZero() {
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

	// Events per day
	fmt.Println("\nEvents per day:")
	var sortedDates []string
	for date := range groupedByDate {
		sortedDates = append(sortedDates, date)
	}
	sort.Strings(sortedDates)

	for _, date := range sortedDates {
		events := groupedByDate[date]
		dayDuration := calculateDuration(events)
		hours := int(dayDuration.Hours())
		minutes := int(dayDuration.Minutes()) % 60
		durationStr := ""
		if hours > 0 || minutes > 0 {
			durationStr = fmt.Sprintf(" (%dh%dm)", hours, minutes)
		}
		fmt.Printf("- %s: %d events%s\n", date, len(events), durationStr)
	}
}
