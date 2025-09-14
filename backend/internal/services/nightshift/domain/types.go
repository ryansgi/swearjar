package domain

import "time"

// HourRef mirrors backfill's helper for clarity
type HourRef struct{ Year, Month, Day, Hour int }

// UTC returns the time.Time for the HourRef in UTC
func (h HourRef) UTC() time.Time {
	return time.Date(h.Year, time.Month(h.Month), h.Day, h.Hour, 0, 0, 0, time.UTC)
}
