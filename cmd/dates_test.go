package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolvePreset(t *testing.T) {
	// Fixed reference time: Wednesday, March 25, 2026
	now := time.Date(2026, 3, 25, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		preset    string
		wantStart string
		wantEnd   string
	}{
		{"last-7d", "2026-03-18", "2026-03-25"},
		{"last-14d", "2026-03-11", "2026-03-25"},
		{"last-30d", "2026-02-23", "2026-03-25"},
		{"last-90d", "2025-12-25", "2026-03-25"},
		{"this-week", "2026-03-23", "2026-03-25"},  // Monday of this week
		{"this-month", "2026-03-01", "2026-03-25"},
		{"this-quarter", "2026-01-01", "2026-03-25"},
		{"", "2026-02-23", "2026-03-25"}, // default = last-30d
	}

	for _, tt := range tests {
		t.Run(tt.preset, func(t *testing.T) {
			start, end := resolvePreset(tt.preset, now)
			assert.Equal(t, tt.wantStart, start.Format("2006-01-02"))
			assert.Equal(t, tt.wantEnd, end.Format("2006-01-02"))
		})
	}
}

func TestResolvePresetLastWeek(t *testing.T) {
	// Wednesday March 25 -> this week starts Monday March 23
	// last week = Monday March 16 to Sunday March 22
	now := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	start, end := resolvePreset("last-week", now)
	assert.Equal(t, "2026-03-16", start.Format("2006-01-02"))
	assert.Equal(t, "2026-03-22", end.Format("2006-01-02"))
}

func TestResolvePresetLastMonth(t *testing.T) {
	now := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	start, end := resolvePreset("last-month", now)
	assert.Equal(t, "2026-02-01", start.Format("2006-01-02"))
	assert.Equal(t, "2026-02-28", end.Format("2006-01-02"))
}

func TestParseDate(t *testing.T) {
	fallback := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("valid", func(t *testing.T) {
		got := parseDate("2026-03-15", fallback)
		assert.Equal(t, "2026-03-15", got.Format("2006-01-02"))
	})

	t.Run("empty", func(t *testing.T) {
		got := parseDate("", fallback)
		assert.Equal(t, fallback, got)
	})

	t.Run("invalid", func(t *testing.T) {
		got := parseDate("not-a-date", fallback)
		assert.Equal(t, fallback, got)
	})
}
