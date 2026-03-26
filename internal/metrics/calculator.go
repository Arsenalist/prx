// Package metrics calculates PR analytics from stored data.
package metrics

import (
	"math"
	"sort"
	"time"

	"github.com/Arsenalist/prx/internal/store"
)

// FileChangeReader is the interface needed to read file changes for LOC classification.
type FileChangeReader interface {
	RawQuery(sql string) ([]map[string]interface{}, error)
}

// CalculateOptions controls what gets calculated.
type CalculateOptions struct {
	StartDate string
	EndDate   string
	Repos     []string
	Team      string
}

// Calculate runs all metrics calculations on the given PRs.
func Calculate(prs []store.PullRequestRecord, db FileChangeReader, opts CalculateOptions) *AnalysisResult {
	result := &AnalysisResult{
		Meta: Meta{
			Tool:    "prx",
			Version: "0.1.0",
			DateRange: Range{
				Start: opts.StartDate,
				End:   opts.EndDate,
			},
			Repos: opts.Repos,
			Team:  opts.Team,
		},
	}

	if len(prs) == 0 {
		return result
	}

	result.Summary = calculateSummary(prs, db)
	result.Summary.Review = calculateReviewSummary(prs, db)
	result.Developers = calculateDeveloperStats(prs, db, opts)
	result.SlowestPRs = calculateSlowestPRs(prs, db)
	result.ReviewerStats = calculateReviewerStats(prs, db)

	return result
}

func calculateSummary(prs []store.PullRequestRecord, db FileChangeReader) Summary {
	s := Summary{}
	authors := make(map[string]bool)
	var locPerPR []float64

	for _, pr := range prs {
		s.TotalPRs++
		authors[pr.Author] = true

		switch pr.State {
		case "merged":
			s.MergedPRs++
		case "closed":
			s.ClosedPRs++
		case "open":
			s.OpenPRs++
		}

		loc := pr.Additions + pr.Deletions
		locPerPR = append(locPerPR, float64(loc))
		s.LOC.Total += loc
	}

	s.UniqueAuthors = len(authors)

	if s.TotalPRs > 0 {
		s.LOC.AvgPerPR = float64(s.LOC.Total) / float64(s.TotalPRs)
		s.LOC.MedianPerPR = median(locPerPR)
	}

	// Calculate test/prod LOC from file_changes
	fileLOC := getFileLOC(db, prs)
	s.LOC.Test = fileLOC.test
	s.LOC.Production = fileLOC.prod

	// Calculate timing from merged PRs
	s.Timing = calculateTimingAvg(prs, db)

	return s
}

type locSums struct {
	test int
	prod int
}

func getFileLOC(db FileChangeReader, prs []store.PullRequestRecord) locSums {
	var sums locSums
	if db == nil {
		return sums
	}

	// Build list of PR IDs
	ids := make(map[int64]bool)
	for _, pr := range prs {
		if pr.ID > 0 {
			ids[pr.ID] = true
		}
	}
	if len(ids) == 0 {
		return sums
	}

	rows, err := db.RawQuery("SELECT pr_id, is_test, SUM(additions + deletions) as loc FROM file_changes GROUP BY pr_id, is_test")
	if err != nil {
		return sums
	}

	for _, row := range rows {
		prID, ok := row["pr_id"].(int64)
		if !ok || !ids[prID] {
			continue
		}
		loc, _ := row["loc"].(int64)
		isTest, _ := row["is_test"].(int64)
		if isTest == 1 {
			sums.test += int(loc)
		} else {
			sums.prod += int(loc)
		}
	}
	return sums
}

func calculateTimingAvg(prs []store.PullRequestRecord, db FileChangeReader) TimingAvg {
	var toOpen, toMerge, total []float64

	branchDates := getBranchDates(db, prs)

	for _, pr := range prs {
		if pr.State != "merged" || pr.MergedAt == nil {
			continue
		}

		mergedAt := parseTime(*pr.MergedAt)
		createdAt := parseTime(pr.CreatedAt)
		if mergedAt.IsZero() || createdAt.IsZero() {
			continue
		}

		// Time to open (coding time): branch created → PR opened
		if branchDate, ok := branchDates[pr.ID]; ok && !branchDate.IsZero() {
			toOpen = append(toOpen, hours(createdAt.Sub(branchDate)))
		}

		// Review start: ready_for_review_at if was draft, otherwise created_at
		reviewStart := createdAt
		if pr.ReadyForReviewAt != nil {
			if t := parseTime(*pr.ReadyForReviewAt); !t.IsZero() {
				reviewStart = t
			}
		}

		// Time to merge: review start → merged
		toMerge = append(toMerge, hours(mergedAt.Sub(reviewStart)))

		// Total time: branch created → merged
		if branchDate, ok := branchDates[pr.ID]; ok && !branchDate.IsZero() {
			total = append(total, hours(mergedAt.Sub(branchDate)))
		}
	}

	return TimingAvg{
		AvgTimeToOpenHours:     avg(toOpen),
		AvgTimeToMergeHours:    avg(toMerge),
		AvgTotalTimeHours:      avg(total),
		MedianTimeToMergeHours: median(toMerge),
	}
}

func getBranchDates(db FileChangeReader, prs []store.PullRequestRecord) map[int64]time.Time {
	result := make(map[int64]time.Time)
	if db == nil {
		return result
	}

	rows, err := db.RawQuery("SELECT pr_id, first_commit_date FROM branch_info WHERE first_commit_date IS NOT NULL")
	if err != nil {
		return result
	}

	for _, row := range rows {
		prID, ok := row["pr_id"].(int64)
		if !ok {
			continue
		}
		dateStr, ok := row["first_commit_date"].(string)
		if !ok {
			continue
		}
		result[prID] = parseTime(dateStr)
	}
	return result
}

func calculateDeveloperStats(prs []store.PullRequestRecord, db FileChangeReader, opts CalculateOptions) []DeveloperStats {
	byAuthor := make(map[string][]store.PullRequestRecord)
	for _, pr := range prs {
		byAuthor[pr.Author] = append(byAuthor[pr.Author], pr)
	}

	weeks := weeksInRange(opts.StartDate, opts.EndDate)
	branchDates := getBranchDates(db, prs)
	fileLOCByPR := getFileLOCByPR(db, prs)

	var devs []DeveloperStats
	for author, authorPRs := range byAuthor {
		dev := DeveloperStats{Login: author}

		var totalLOC, testLOC, prodLOC []float64
		var toOpen, draftTime, toMerge, totalTime []float64

		for _, pr := range authorPRs {
			if pr.State == "merged" {
				dev.MergedPRs++
			}

			// LOC from file changes
			if floc, ok := fileLOCByPR[pr.ID]; ok {
				testLOC = append(testLOC, float64(floc.test))
				prodLOC = append(prodLOC, float64(floc.prod))
				totalLOC = append(totalLOC, float64(floc.test+floc.prod))
			}

			// Timing (only merged)
			if pr.State == "merged" && pr.MergedAt != nil {
				mergedAt := parseTime(*pr.MergedAt)
				createdAt := parseTime(pr.CreatedAt)

				if branchDate, ok := branchDates[pr.ID]; ok && !branchDate.IsZero() && !createdAt.IsZero() {
					toOpen = append(toOpen, hours(createdAt.Sub(branchDate)))
				}

				if pr.ReadyForReviewAt != nil {
					readyAt := parseTime(*pr.ReadyForReviewAt)
					if !readyAt.IsZero() && !createdAt.IsZero() {
						draftTime = append(draftTime, hours(readyAt.Sub(createdAt)))
					}
				}

				reviewStart := createdAt
				if pr.ReadyForReviewAt != nil {
					if t := parseTime(*pr.ReadyForReviewAt); !t.IsZero() {
						reviewStart = t
					}
				}
				if !mergedAt.IsZero() && !reviewStart.IsZero() {
					toMerge = append(toMerge, hours(mergedAt.Sub(reviewStart)))
				}

				if branchDate, ok := branchDates[pr.ID]; ok && !branchDate.IsZero() && !mergedAt.IsZero() {
					totalTime = append(totalTime, hours(mergedAt.Sub(branchDate)))
				}
			}
		}

		dev.PRsPerWeek = float64(dev.MergedPRs) / weeks
		dev.LOC.AvgTotal = avg(totalLOC)
		dev.LOC.AvgTest = avg(testLOC)
		dev.LOC.AvgProduction = avg(prodLOC)
		dev.Timing.AvgTimeToOpenHours = avg(toOpen)
		dev.Timing.AvgDraftTimeHours = avg(draftTime)
		dev.Timing.AvgTimeToMergeHours = avg(toMerge)
		dev.Timing.AvgTotalTimeHours = avg(totalTime)

		devs = append(devs, dev)
	}

	// Sort by PRs/week desc
	sort.Slice(devs, func(i, j int) bool {
		return devs[i].PRsPerWeek > devs[j].PRsPerWeek
	})

	return devs
}

func getFileLOCByPR(db FileChangeReader, prs []store.PullRequestRecord) map[int64]locSums {
	result := make(map[int64]locSums)
	if db == nil {
		return result
	}

	rows, err := db.RawQuery("SELECT pr_id, is_test, SUM(additions + deletions) as loc FROM file_changes GROUP BY pr_id, is_test")
	if err != nil {
		return result
	}

	for _, row := range rows {
		prID, ok := row["pr_id"].(int64)
		if !ok {
			continue
		}
		loc, _ := row["loc"].(int64)
		isTest, _ := row["is_test"].(int64)
		sums := result[prID]
		if isTest == 1 {
			sums.test += int(loc)
		} else {
			sums.prod += int(loc)
		}
		result[prID] = sums
	}
	return result
}

func calculateSlowestPRs(prs []store.PullRequestRecord, db FileChangeReader) []PRTiming {
	branchDates := getBranchDates(db, prs)
	fileLOCByPR := getFileLOCByPR(db, prs)

	var timings []PRTiming

	for _, pr := range prs {
		if pr.State != "merged" || pr.MergedAt == nil {
			continue
		}

		mergedAt := parseTime(*pr.MergedAt)
		createdAt := parseTime(pr.CreatedAt)
		if mergedAt.IsZero() {
			continue
		}

		timing := PRTiming{
			Number: pr.Number,
			Author: pr.Author,
			Title:  pr.Title,
			URL:    pr.URL,
		}

		if branchDate, ok := branchDates[pr.ID]; ok && !branchDate.IsZero() {
			timing.TimeToOpenHours = hours(createdAt.Sub(branchDate))
			timing.TotalTimeHours = hours(mergedAt.Sub(branchDate))
		}

		if pr.ReadyForReviewAt != nil {
			readyAt := parseTime(*pr.ReadyForReviewAt)
			if !readyAt.IsZero() {
				timing.DraftTimeHours = hours(readyAt.Sub(createdAt))
				timing.TimeToMergeHours = hours(mergedAt.Sub(readyAt))
			}
		} else {
			timing.TimeToMergeHours = hours(mergedAt.Sub(createdAt))
		}

		if floc, ok := fileLOCByPR[pr.ID]; ok {
			timing.LOCTest = floc.test
			timing.LOCProduction = floc.prod
			timing.LOCTotal = floc.test + floc.prod
		}

		timings = append(timings, timing)
	}

	// Sort by total time desc
	sort.Slice(timings, func(i, j int) bool {
		return timings[i].TotalTimeHours > timings[j].TotalTimeHours
	})

	// Cap at 20
	if len(timings) > 20 {
		timings = timings[:20]
	}

	return timings
}

// --- Review metrics ---

type reviewEvent struct {
	prID        int64
	actor       string
	reviewState string
	createdAt   string
}

func getReviewEvents(db FileChangeReader, prs []store.PullRequestRecord) map[int64][]reviewEvent {
	result := make(map[int64][]reviewEvent)
	if db == nil {
		return result
	}

	ids := make(map[int64]bool)
	for _, pr := range prs {
		if pr.ID > 0 {
			ids[pr.ID] = true
		}
	}
	if len(ids) == 0 {
		return result
	}

	rows, err := db.RawQuery("SELECT pr_id, actor, review_state, created_at FROM timeline_events WHERE event_type = 'reviewed' AND review_state IS NOT NULL ORDER BY created_at")
	if err != nil {
		return result
	}

	for _, row := range rows {
		prID, ok := row["pr_id"].(int64)
		if !ok || !ids[prID] {
			continue
		}
		evt := reviewEvent{prID: prID}
		if v, ok := row["actor"].(string); ok {
			evt.actor = v
		}
		if v, ok := row["review_state"].(string); ok {
			evt.reviewState = v
		}
		if v, ok := row["created_at"].(string); ok {
			evt.createdAt = v
		}
		result[prID] = append(result[prID], evt)
	}
	return result
}

func calculateReviewSummary(prs []store.PullRequestRecord, db FileChangeReader) ReviewMetrics {
	reviewEvents := getReviewEvents(db, prs)

	var timeToFirstReview []float64
	var reviewCycles []float64
	var commentsPerPR []float64

	for _, pr := range prs {
		if pr.State != "merged" {
			continue
		}

		// Review cycles: count changes_requested events
		events := reviewEvents[pr.ID]
		cycles := 0
		for _, evt := range events {
			if evt.reviewState == "changes_requested" {
				cycles++
			}
		}
		reviewCycles = append(reviewCycles, float64(cycles))

		// Time to first review: earliest non-author reviewed event after ready time
		reviewStart := parseTime(pr.CreatedAt)
		if pr.ReadyForReviewAt != nil {
			if t := parseTime(*pr.ReadyForReviewAt); !t.IsZero() {
				reviewStart = t
			}
		}

		for _, evt := range events {
			if evt.actor != pr.Author && evt.createdAt != "" {
				reviewedAt := parseTime(evt.createdAt)
				if !reviewedAt.IsZero() && !reviewStart.IsZero() {
					timeToFirstReview = append(timeToFirstReview, hours(reviewedAt.Sub(reviewStart)))
				}
				break // first non-author review only
			}
		}

		// Comments per PR
		totalComments := pr.CommentCount + pr.ReviewCommentCount
		if totalComments > 0 || pr.CommentCount > 0 || pr.ReviewCommentCount > 0 {
			commentsPerPR = append(commentsPerPR, float64(totalComments))
		}
	}

	return ReviewMetrics{
		AvgTimeToFirstReviewHours:    avg(timeToFirstReview),
		MedianTimeToFirstReviewHours: median(timeToFirstReview),
		AvgReviewCycles:              avg(reviewCycles),
		AvgCommentsPerPR:             avg(commentsPerPR),
	}
}

func calculateReviewerStats(prs []store.PullRequestRecord, db FileChangeReader) []ReviewerStats {
	reviewEvents := getReviewEvents(db, prs)

	// Build PR lookup for review start time
	prMap := make(map[int64]store.PullRequestRecord)
	for _, pr := range prs {
		prMap[pr.ID] = pr
	}

	type reviewerAgg struct {
		reviewsGiven     int
		approvals        int
		changesRequested int
		turnarounds      []float64
	}
	byReviewer := make(map[string]*reviewerAgg)

	for prID, events := range reviewEvents {
		pr, ok := prMap[prID]
		if !ok {
			continue
		}

		reviewStart := parseTime(pr.CreatedAt)
		if pr.ReadyForReviewAt != nil {
			if t := parseTime(*pr.ReadyForReviewAt); !t.IsZero() {
				reviewStart = t
			}
		}

		// Track first review per reviewer per PR for turnaround
		firstReviewByReviewer := make(map[string]bool)

		for _, evt := range events {
			if evt.actor == "" || evt.actor == pr.Author {
				continue
			}
			agg, ok := byReviewer[evt.actor]
			if !ok {
				agg = &reviewerAgg{}
				byReviewer[evt.actor] = agg
			}
			agg.reviewsGiven++
			switch evt.reviewState {
			case "approved":
				agg.approvals++
			case "changes_requested":
				agg.changesRequested++
			}

			if !firstReviewByReviewer[evt.actor] {
				firstReviewByReviewer[evt.actor] = true
				if evt.createdAt != "" && !reviewStart.IsZero() {
					reviewedAt := parseTime(evt.createdAt)
					if !reviewedAt.IsZero() {
						agg.turnarounds = append(agg.turnarounds, hours(reviewedAt.Sub(reviewStart)))
					}
				}
			}
		}
	}

	var stats []ReviewerStats
	for login, agg := range byReviewer {
		stats = append(stats, ReviewerStats{
			Login:                    login,
			ReviewsGiven:             agg.reviewsGiven,
			Approvals:                agg.approvals,
			ChangesRequested:         agg.changesRequested,
			AvgReviewTurnaroundHours: avg(agg.turnarounds),
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].ReviewsGiven > stats[j].ReviewsGiven
	})

	return stats
}

// --- Helpers ---

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, _ = time.Parse("2006-01-02T15:04:05Z", s)
	}
	return t
}

func hours(d time.Duration) float64 {
	return math.Round(d.Hours()*10) / 10
}

func weeksInRange(start, end string) float64 {
	s := parseTime(start + "T00:00:00Z")
	e := parseTime(end + "T23:59:59Z")
	if s.IsZero() || e.IsZero() {
		e = time.Now()
		s = e.AddDate(0, 0, -30) // default 30 days
	}
	weeks := e.Sub(s).Hours() / (24 * 7)
	if weeks < 1.0/7.0 { // minimum 1 day
		weeks = 1.0 / 7.0
	}
	return weeks
}

func avg(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}
