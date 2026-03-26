// Package report handles output formatting (table, JSON, markdown).
package report

import (
	"fmt"
	"strings"

	"github.com/Arsenalist/prx/internal/metrics"
)

// FormatTable returns the analysis result as formatted ASCII tables.
func FormatTable(result *metrics.AnalysisResult) string {
	var sb strings.Builder

	// Summary
	sb.WriteString("\n=== Summary ===\n\n")
	summaryRows := [][]string{
		{"Total PRs", fmt.Sprintf("%d", result.Summary.TotalPRs)},
		{"Merged", fmt.Sprintf("%d", result.Summary.MergedPRs)},
		{"Closed (not merged)", fmt.Sprintf("%d", result.Summary.ClosedPRs)},
		{"Open", fmt.Sprintf("%d", result.Summary.OpenPRs)},
		{"Unique Authors", fmt.Sprintf("%d", result.Summary.UniqueAuthors)},
		{"Total LOC", fmt.Sprintf("%d", result.Summary.LOC.Total)},
		{"  Test LOC", fmt.Sprintf("%d", result.Summary.LOC.Test)},
		{"  Production LOC", fmt.Sprintf("%d", result.Summary.LOC.Production)},
		{"Avg LOC/PR", fmt.Sprintf("%.0f", result.Summary.LOC.AvgPerPR)},
		{"Median LOC/PR", fmt.Sprintf("%.0f", result.Summary.LOC.MedianPerPR)},
		{"Avg Time to Merge", FormatDuration(result.Summary.Timing.AvgTimeToMergeHours)},
		{"Median Time to Merge", FormatDuration(result.Summary.Timing.MedianTimeToMergeHours)},
	}
	writeSimpleTable(&sb, summaryRows)

	// Review Metrics
	if result.Summary.Review.AvgTimeToFirstReviewHours > 0 || result.Summary.Review.AvgReviewCycles > 0 || result.Summary.Review.AvgCommentsPerPR > 0 {
		sb.WriteString("\n=== Review Metrics ===\n\n")
		reviewRows := [][]string{
			{"Avg Time to First Review", FormatDuration(result.Summary.Review.AvgTimeToFirstReviewHours)},
			{"Median Time to First Review", FormatDuration(result.Summary.Review.MedianTimeToFirstReviewHours)},
			{"Avg Review Cycles", fmt.Sprintf("%.1f", result.Summary.Review.AvgReviewCycles)},
			{"Avg Comments/PR", fmt.Sprintf("%.1f", result.Summary.Review.AvgCommentsPerPR)},
		}
		writeSimpleTable(&sb, reviewRows)
	}

	// Reviewer Stats
	if len(result.ReviewerStats) > 0 {
		sb.WriteString("\n=== Reviewer Stats ===\n\n")
		header := []string{"Reviewer", "Reviews", "Approvals", "Changes Req", "Avg Turnaround"}
		var rows [][]string
		for _, rs := range result.ReviewerStats {
			rows = append(rows, []string{
				rs.Login,
				fmt.Sprintf("%d", rs.ReviewsGiven),
				fmt.Sprintf("%d", rs.Approvals),
				fmt.Sprintf("%d", rs.ChangesRequested),
				FormatDuration(rs.AvgReviewTurnaroundHours),
			})
		}
		writeAlignedTable(&sb, header, rows)
	}

	// Developer metrics
	if len(result.Developers) > 0 {
		sb.WriteString("\n=== Developer Metrics ===\n\n")
		header := []string{"Developer", "Merged", "PRs/Wk", "Avg LOC", "Avg Test", "Avg Prod", "Avg→Open", "Avg→Merge", "Avg Total"}
		var rows [][]string
		for _, dev := range result.Developers {
			rows = append(rows, []string{
				dev.Login,
				fmt.Sprintf("%d", dev.MergedPRs),
				fmt.Sprintf("%.2f", dev.PRsPerWeek),
				fmt.Sprintf("%.0f", dev.LOC.AvgTotal),
				fmt.Sprintf("%.0f", dev.LOC.AvgTest),
				fmt.Sprintf("%.0f", dev.LOC.AvgProduction),
				FormatDuration(dev.Timing.AvgTimeToOpenHours),
				FormatDuration(dev.Timing.AvgTimeToMergeHours),
				FormatDuration(dev.Timing.AvgTotalTimeHours),
			})
		}
		writeAlignedTable(&sb, header, rows)
	}

	// Slowest PRs
	if len(result.SlowestPRs) > 0 {
		sb.WriteString("\n=== Slowest PRs ===\n\n")
		header := []string{"PR#", "Author", "Title", "→Open", "Draft", "→Merge", "Total", "LOC"}
		var rows [][]string
		for _, pr := range result.SlowestPRs {
			title := pr.Title
			if len(title) > 40 {
				title = title[:37] + "..."
			}
			rows = append(rows, []string{
				fmt.Sprintf("#%d", pr.Number),
				pr.Author,
				title,
				FormatDuration(pr.TimeToOpenHours),
				FormatDuration(pr.DraftTimeHours),
				FormatDuration(pr.TimeToMergeHours),
				FormatDuration(pr.TotalTimeHours),
				fmt.Sprintf("%d", pr.LOCTotal),
			})
		}
		writeAlignedTable(&sb, header, rows)
	}

	return sb.String()
}

// FormatDuration formats hours into a human-readable duration.
func FormatDuration(hours float64) string {
	if hours <= 0 {
		return "0h"
	}

	totalHours := int(hours)
	weeks := totalHours / (7 * 24)
	days := (totalHours % (7 * 24)) / 24
	hrs := totalHours % 24

	var parts []string
	if weeks > 0 {
		parts = append(parts, fmt.Sprintf("%dw", weeks))
	}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hrs > 0 && weeks == 0 {
		parts = append(parts, fmt.Sprintf("%dh", hrs))
	}

	if len(parts) == 0 {
		return "0h"
	}
	return strings.Join(parts, " ")
}

// writeSimpleTable writes a 2-column key/value table.
func writeSimpleTable(sb *strings.Builder, rows [][]string) {
	maxKey := 0
	for _, row := range rows {
		if len(row[0]) > maxKey {
			maxKey = len(row[0])
		}
	}
	for _, row := range rows {
		fmt.Fprintf(sb, "  %-*s  %s\n", maxKey, row[0], row[1])
	}
}

// writeAlignedTable writes a table with header and aligned columns.
func writeAlignedTable(sb *strings.Builder, header []string, rows [][]string) {
	// Calculate column widths
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Header
	sb.WriteString("  ")
	for i, h := range header {
		if i > 0 {
			sb.WriteString("  ")
		}
		fmt.Fprintf(sb, "%-*s", widths[i], h)
	}
	sb.WriteString("\n")

	// Separator
	sb.WriteString("  ")
	for i, w := range widths {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(strings.Repeat("─", w))
	}
	sb.WriteString("\n")

	// Rows
	for _, row := range rows {
		sb.WriteString("  ")
		for i, cell := range row {
			if i > 0 {
				sb.WriteString("  ")
			}
			if i < len(widths) {
				fmt.Fprintf(sb, "%-*s", widths[i], cell)
			}
		}
		sb.WriteString("\n")
	}
}
