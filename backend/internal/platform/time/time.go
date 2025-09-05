// Package time contains time related helpers
package time

import "time"

// Ptr returns a pointer to t or nil if t is zero
func Ptr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
