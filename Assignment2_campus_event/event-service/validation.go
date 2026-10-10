package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const dateLayout = "2006-01-02"

const maxCapacity = 5000

// validateEventFields checks every field of a create or update request.
// It trims whitespace in place and returns all problems in one message,
// so the client can fix everything in one go.
func validateEventFields(title, venue, eventDate, organizer *string, capacity int32) error {
	*title = strings.TrimSpace(*title)
	*venue = strings.TrimSpace(*venue)
	*eventDate = strings.TrimSpace(*eventDate)
	*organizer = strings.TrimSpace(*organizer)

	var problems []string

	switch n := utf8.RuneCountInString(*title); {
	case n == 0:
		problems = append(problems, "title is required")
	case n < 3 || n > 100:
		problems = append(problems, "title must be between 3 and 100 characters")
	}

	switch n := utf8.RuneCountInString(*venue); {
	case n == 0:
		problems = append(problems, "venue is required")
	case n > 100:
		problems = append(problems, "venue must be at most 100 characters")
	}

	if *eventDate == "" {
		problems = append(problems, "event_date is required")
	} else if _, err := time.Parse(dateLayout, *eventDate); err != nil {
		problems = append(problems, "event_date must be a real date in the format YYYY-MM-DD")
	}

	if capacity < 1 || capacity > maxCapacity {
		problems = append(problems, fmt.Sprintf("capacity must be between 1 and %d", maxCapacity))
	}

	switch n := utf8.RuneCountInString(*organizer); {
	case n == 0:
		problems = append(problems, "organizer is required")
	case n > 100:
		problems = append(problems, "organizer must be at most 100 characters")
	}

	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}
