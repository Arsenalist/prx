package cmd

import (
	"time"

	"github.com/Arsenalist/prx/internal/config"
	"github.com/spf13/cobra"
)

// resolveDateRange determines the start/end dates from CLI flags, presets, or settings defaults.
func resolveDateRange(cmd *cobra.Command, s *config.Settings) (time.Time, time.Time) {
	now := time.Now()

	// CLI flags take highest priority
	startStr, _ := cmd.Flags().GetString("start")
	endStr, _ := cmd.Flags().GetString("end")
	preset, _ := cmd.Flags().GetString("preset")

	// Fall back to settings
	if startStr == "" {
		startStr = s.DateRange.Start
	}
	if endStr == "" {
		endStr = s.DateRange.End
	}
	if preset == "" && startStr == "" && endStr == "" {
		preset = s.DateRange.Preset
	}

	// If explicit dates provided, use them
	if startStr != "" || endStr != "" {
		start := parseDate(startStr, now.AddDate(0, -1, 0))
		end := parseDate(endStr, now)
		return start, end
	}

	// Resolve preset
	start, end := resolvePreset(preset, now)
	return start, end
}

// resolvePreset maps a preset name to a date range.
func resolvePreset(preset string, now time.Time) (time.Time, time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch preset {
	case "last-7d":
		return today.AddDate(0, 0, -7), today
	case "last-14d":
		return today.AddDate(0, 0, -14), today
	case "last-30d":
		return today.AddDate(0, 0, -30), today
	case "last-90d":
		return today.AddDate(0, 0, -90), today
	case "this-week":
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return today.AddDate(0, 0, -(weekday - 1)), today
	case "last-week":
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startOfThisWeek := today.AddDate(0, 0, -(weekday - 1))
		return startOfThisWeek.AddDate(0, 0, -7), startOfThisWeek.AddDate(0, 0, -1)
	case "this-month":
		return time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location()), today
	case "last-month":
		firstOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		return firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1)
	case "this-quarter":
		q := (int(today.Month()) - 1) / 3
		return time.Date(today.Year(), time.Month(q*3+1), 1, 0, 0, 0, 0, today.Location()), today
	default:
		// Default to last 30 days
		return today.AddDate(0, 0, -30), today
	}
}

// parseDate parses a YYYY-MM-DD string, returning fallback on failure.
func parseDate(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fallback
	}
	return t
}
