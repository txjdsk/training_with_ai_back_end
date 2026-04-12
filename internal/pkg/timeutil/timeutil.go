package timeutil

import (
	"errors"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// ParseQueryTime parses RFC3339 or YYYY-MM-DD query values.
// If isEnd is true and the input is date-only, it returns the end of that day.
func ParseQueryTime(value string, isEnd bool) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return &parsed, nil
	}

	parsed, err := time.ParseInLocation(dateLayout, trimmed, time.Local)
	if err != nil {
		return nil, errors.New("invalid time format")
	}

	if isEnd {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}

	return &parsed, nil
}
