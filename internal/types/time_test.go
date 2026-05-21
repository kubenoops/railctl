package types

import (
	"strings"
	"testing"
	"time"
)

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time",
			input:    time.Time{},
			expected: "",
		},
		{
			name:     "seconds ago",
			input:    time.Now().Add(-30 * time.Second),
			expected: "30s ago",
		},
		{
			name:     "minutes ago",
			input:    time.Now().Add(-5 * time.Minute),
			expected: "5m ago",
		},
		{
			name:     "hours ago",
			input:    time.Now().Add(-3 * time.Hour),
			expected: "3h ago",
		},
		{
			name:     "days ago",
			input:    time.Now().Add(-5 * 24 * time.Hour),
			expected: "5d ago",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RelativeTime(tc.input)
			if result != tc.expected {
				t.Errorf("RelativeTime() = %q, expected %q", result, tc.expected)
			}
		})
	}
}

func TestRelativeTime_Months(t *testing.T) {
	// 60 days ago should show as "2mo ago"
	oldDate := time.Now().Add(-60 * 24 * time.Hour)
	result := RelativeTime(oldDate)

	if !strings.HasSuffix(result, "mo ago") {
		t.Errorf("expected months format (Xmo ago), got %q", result)
	}
}

func TestRelativeTime_Years(t *testing.T) {
	// 400 days ago should show as "1y ago"
	oldDate := time.Now().Add(-400 * 24 * time.Hour)
	result := RelativeTime(oldDate)

	if !strings.HasSuffix(result, "y ago") {
		t.Errorf("expected years format (Xy ago), got %q", result)
	}
}
