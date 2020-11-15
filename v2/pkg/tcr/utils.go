package tcr

import "time"

// JSONUtcTimestamp quickly creates a string RFC3339 format in UTC
func JSONUtcTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// JSONUtcTimestampFromTime quickly creates a string RFC3339 format in UTC
func JSONUtcTimestampFromTime(t time.Time) string {
	return t.Format(time.RFC3339)
}
