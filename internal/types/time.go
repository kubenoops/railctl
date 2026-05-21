// Package types provides time formatting utilities.
package types

import (
	"fmt"
	"time"
)

// RelativeTime formats a time as a human-readable relative duration.
// Examples: "5s ago", "3m ago", "2h ago", "5d ago", "3mo ago", "2y ago"
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	diff := time.Since(t)
	seconds := int(diff.Seconds())

	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds ago", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm ago", seconds/60)
	case seconds < 86400:
		return fmt.Sprintf("%dh ago", seconds/3600)
	case seconds < 86400*30:
		return fmt.Sprintf("%dd ago", seconds/86400)
	case seconds < 86400*365:
		return fmt.Sprintf("%dmo ago", seconds/(86400*30))
	default:
		return fmt.Sprintf("%dy ago", seconds/(86400*365))
	}
}
