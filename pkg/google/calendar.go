package google

import (
	"context"
	"fmt"
	"io"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// CalendarEvent is a calendar event fetched from Google Calendar API.
type CalendarEvent struct {
	ID       string
	Summary  string
	Start    time.Time
	End      time.Time
	IsAllDay bool
}

// FetchCalendarEvents returns events from all Google Calendars in the given date range.
func FetchCalendarEvents(start, end time.Time, writer io.Writer) ([]CalendarEvent, error) {
	ctx := context.Background()

	client, err := getHTTPClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Google: %w", err)
	}

	svc, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar service: %w", err)
	}

	timeMin := start.UTC().Format(time.RFC3339)
	timeMax := end.AddDate(0, 0, 1).UTC().Format(time.RFC3339)

	seen := make(map[string]bool)
	var events []CalendarEvent

	// Fetch only from primary calendar to avoid picking up shared/team calendars
	for _, calID := range []string{"primary"} {
		fmt.Fprintf(writer, "Fetching events from calendar: %s\n", calID)

		req := svc.Events.List(calID).
			TimeMin(timeMin).
			TimeMax(timeMax).
			SingleEvents(true).
			OrderBy("startTime").
			MaxResults(2500)

		for {
			resp, err := req.Do()
			if err != nil {
				fmt.Fprintf(writer, "  Warning: failed to fetch events from %s: %v\n", calID, err)
				break
			}

			for _, item := range resp.Items {
				if seen[item.Id] {
					continue
				}
				seen[item.Id] = true

				ev, ok := convertEvent(item)
				if !ok {
					continue
				}
				events = append(events, ev)
			}

			if resp.NextPageToken == "" {
				break
			}
			req = req.PageToken(resp.NextPageToken)
		}
	}

	fmt.Fprintf(writer, "Fetched %d events from Google Calendar API\n", len(events))
	return events, nil
}

// convertEvent converts a Google Calendar API event to CalendarEvent.
func convertEvent(item *calendar.Event) (CalendarEvent, bool) {
	ev := CalendarEvent{
		ID:      item.Id,
		Summary: item.Summary,
	}

	if item.Start == nil {
		return ev, false
	}

	if item.Start.Date != "" {
		// All-day event
		ev.IsAllDay = true
		t, err := time.Parse("2006-01-02", item.Start.Date)
		if err != nil {
			return ev, false
		}
		ev.Start = t

		if item.End != nil && item.End.Date != "" {
			t, err := time.Parse("2006-01-02", item.End.Date)
			if err != nil {
				return ev, false
			}
			ev.End = t
		}
	} else if item.Start.DateTime != "" {
		t, err := time.Parse(time.RFC3339, item.Start.DateTime)
		if err != nil {
			return ev, false
		}
		ev.Start = t

		if item.End != nil && item.End.DateTime != "" {
			t, err := time.Parse(time.RFC3339, item.End.DateTime)
			if err != nil {
				return ev, false
			}
			ev.End = t
		}
	} else {
		return ev, false
	}

	return ev, true
}
