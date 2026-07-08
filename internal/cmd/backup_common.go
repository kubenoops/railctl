package cmd

import (
	"time"
)

// formatBackupTime renders an RFC3339 timestamp as "2006-01-02 15:04".
func formatBackupTime(ts string) string {
	if ts == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Format("2006-01-02 15:04")
}
