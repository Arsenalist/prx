package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Arsenalist/prx/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleResult() *metrics.AnalysisResult {
	return &metrics.AnalysisResult{
		Meta: metrics.Meta{
			Tool:    "prx",
			Version: "0.1.0",
			DateRange: metrics.Range{
				Start: "2026-03-01",
				End:   "2026-03-31",
			},
			Repos: []string{"org/repo"},
		},
		Summary: metrics.Summary{
			TotalPRs:      4,
			MergedPRs:     2,
			ClosedPRs:     1,
			OpenPRs:       1,
			UniqueAuthors: 3,
			LOC: metrics.LOCMetrics{
				Total:       545,
				Test:        145,
				Production:  325,
				AvgPerPR:    136.25,
				MedianPerPR: 90.0,
			},
			Timing: metrics.TimingAvg{
				AvgTimeToOpenHours:     71.0,
				AvgTimeToMergeHours:    86.0,
				AvgTotalTimeHours:      242.0,
				MedianTimeToMergeHours: 72.0,
			},
		},
		Developers: []metrics.DeveloperStats{
			{
				Login:      "alice",
				MergedPRs:  1,
				PRsPerWeek: 0.23,
			},
			{
				Login:      "bob",
				MergedPRs:  1,
				PRsPerWeek: 0.23,
			},
		},
		SlowestPRs: []metrics.PRTiming{
			{
				Number:           2,
				Author:           "bob",
				Title:            "Slow draft PR",
				URL:              "https://github.com/org/repo/pull/2",
				TimeToOpenHours:  118.0,
				DraftTimeHours:   170.0,
				TimeToMergeHours: 72.0,
				TotalTimeHours:   360.0,
				LOCTotal:         350,
				LOCTest:          120,
				LOCProduction:    230,
			},
			{
				Number:           1,
				Author:           "alice",
				Title:            "Fast PR",
				URL:              "https://github.com/org/repo/pull/1",
				TimeToOpenHours:  24.0,
				TimeToMergeHours: 100.0,
				TotalTimeHours:   124.0,
				LOCTotal:         120,
				LOCTest:          25,
				LOCProduction:    95,
			},
		},
	}
}

func TestFormatJSON(t *testing.T) {
	result := sampleResult()
	output, err := FormatJSON(result)
	require.NoError(t, err)

	// Should be valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err)

	// Check key fields present
	assert.Contains(t, output, `"total_prs"`)
	assert.Contains(t, output, `"merged_prs"`)
	assert.Contains(t, output, `"developers"`)
	assert.Contains(t, output, `"slowest_prs"`)
	assert.Contains(t, output, `"meta"`)
}

func TestFormatTable(t *testing.T) {
	result := sampleResult()
	output := FormatTable(result)

	// Should contain key sections
	assert.Contains(t, output, "Summary")
	assert.Contains(t, output, "Total PRs")
	assert.Contains(t, output, "Merged")
	assert.Contains(t, output, "alice")
	assert.Contains(t, output, "bob")
	assert.Contains(t, output, "Slowest")
}

func TestFormatMarkdown(t *testing.T) {
	result := sampleResult()
	output := FormatMarkdown(result)

	// Should be valid markdown with headers
	assert.True(t, strings.HasPrefix(output, "# "))
	assert.Contains(t, output, "## Summary")
	assert.Contains(t, output, "## Developer Metrics")
	assert.Contains(t, output, "## Slowest PRs")
	assert.Contains(t, output, "| Metric |")
	assert.Contains(t, output, "alice")
}

func TestMarkdownFilename(t *testing.T) {
	assert.Equal(t, "org-repo-2026-03-25.md", MarkdownFilename([]string{"org/repo"}, "", "2026-03-25"))
	assert.Equal(t, "platform-2026-03-25.md", MarkdownFilename([]string{"org/a", "org/b"}, "platform", "2026-03-25"))
	assert.Equal(t, "multi-repo-3-repos-2026-03-25.md", MarkdownFilename([]string{"a/1", "a/2", "a/3"}, "", "2026-03-25"))
}

func TestFormatDuration(t *testing.T) {
	assert.Equal(t, "2h", FormatDuration(2.0))
	assert.Equal(t, "1d 4h", FormatDuration(28.0))
	assert.Equal(t, "3d", FormatDuration(72.0))
	assert.Equal(t, "1w 1d", FormatDuration(192.0))
	assert.Equal(t, "0h", FormatDuration(0.0))
}
