package metrics

import (
	"path/filepath"
	"testing"

	"github.com/Arsenalist/prx/internal/store"
	"github.com/Arsenalist/prx/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedTestData(t *testing.T) (*sqlite.SQLiteStore, int64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s := sqlite.New(path)
	require.NoError(t, s.Open())
	require.NoError(t, s.Migrate())
	t.Cleanup(func() { s.Close() })

	instID, _ := s.UpsertInstance(store.InstanceRecord{Name: "default", Type: "github", BaseURL: "https://api.github.com"})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo"})

	mergedAt1 := "2026-03-05T14:00:00Z"
	mergedAt2 := "2026-03-12T10:00:00Z"
	closedAt := "2026-03-08T16:00:00Z"
	readyAt := "2026-03-09T10:00:00Z"

	// PR 1: alice, merged, fast
	pr1ID, _ := s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "Fast PR", State: "merged", Author: "alice",
		URL: "https://github.com/org/repo/pull/1",
		CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T14:00:00Z",
		MergedAt: &mergedAt1, Additions: 100, Deletions: 20, ChangedFiles: 5,
	})
	sha1 := "abc"
	date1 := "2026-02-28T10:00:00Z"
	s.UpsertBranchInfo(store.BranchInfoRecord{
		PRID: pr1ID, MergeBaseSHA: &sha1, FirstCommitDate: &date1,
		CommitsCount: 3, TotalAdditions: 100, TotalDeletions: 20,
	})
	s.ReplaceFileChanges(pr1ID, []store.FileChangeRecord{
		{Filename: "main.go", Additions: 80, Deletions: 15, IsTest: false},
		{Filename: "main_test.go", Additions: 20, Deletions: 5, IsTest: true},
	})

	// PR 2: bob, merged, slow, was draft
	pr2ID, _ := s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 2, Title: "Slow draft PR", State: "merged", Author: "bob",
		URL: "https://github.com/org/repo/pull/2",
		CreatedAt: "2026-03-02T08:00:00Z", UpdatedAt: "2026-03-12T10:00:00Z",
		MergedAt: &mergedAt2, ReadyForReviewAt: &readyAt,
		Additions: 300, Deletions: 50, ChangedFiles: 12,
	})
	sha2 := "def"
	date2 := "2026-02-25T10:00:00Z"
	s.UpsertBranchInfo(store.BranchInfoRecord{
		PRID: pr2ID, MergeBaseSHA: &sha2, FirstCommitDate: &date2,
		CommitsCount: 8, TotalAdditions: 300, TotalDeletions: 50,
	})
	s.ReplaceFileChanges(pr2ID, []store.FileChangeRecord{
		{Filename: "handler.go", Additions: 200, Deletions: 30, IsTest: false},
		{Filename: "handler_test.go", Additions: 100, Deletions: 20, IsTest: true},
	})

	// PR 3: alice, closed (not merged)
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 3, Title: "Closed PR", State: "closed", Author: "alice",
		URL: "https://github.com/org/repo/pull/3",
		CreatedAt: "2026-03-03T10:00:00Z", UpdatedAt: "2026-03-08T16:00:00Z",
		ClosedAt: &closedAt, Additions: 10, Deletions: 5,
	})

	// PR 4: charlie, open
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 4, Title: "Open PR", State: "open", Author: "charlie",
		URL: "https://github.com/org/repo/pull/4",
		CreatedAt: "2026-03-15T10:00:00Z", UpdatedAt: "2026-03-20T10:00:00Z",
		Additions: 50, Deletions: 10,
	})

	return s, repoID
}

func TestCalculateVolume(t *testing.T) {
	s, repoID := seedTestData(t)

	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)

	result := Calculate(prs, s, CalculateOptions{
		StartDate: "2026-03-01",
		EndDate:   "2026-03-31",
		Repos:     []string{"org/repo"},
	})

	assert.Equal(t, 4, result.Summary.TotalPRs)
	assert.Equal(t, 2, result.Summary.MergedPRs)
	assert.Equal(t, 1, result.Summary.ClosedPRs)
	assert.Equal(t, 1, result.Summary.OpenPRs)
	assert.Equal(t, 3, result.Summary.UniqueAuthors)
}

func TestCalculateLOC(t *testing.T) {
	s, repoID := seedTestData(t)

	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)

	result := Calculate(prs, s, CalculateOptions{
		StartDate: "2026-03-01",
		EndDate:   "2026-03-31",
		Repos:     []string{"org/repo"},
	})

	// LOC.Total = sum of PR-level additions+deletions (all 4 PRs)
	// (100+20) + (300+50) + (10+5) + (50+10) = 545
	assert.Equal(t, 545, result.Summary.LOC.Total)
	// LOC.Test/Production from file_changes (only PRs with file data):
	// PR1: main_test.go 25, PR2: handler_test.go 120 → test=145
	// PR1: main.go 95, PR2: handler.go 230 → prod=325
	assert.Equal(t, 145, result.Summary.LOC.Test)
	assert.Equal(t, 325, result.Summary.LOC.Production)
}

func TestCalculateTiming(t *testing.T) {
	s, repoID := seedTestData(t)

	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)

	result := Calculate(prs, s, CalculateOptions{
		StartDate: "2026-03-01",
		EndDate:   "2026-03-31",
		Repos:     []string{"org/repo"},
	})

	// Only merged PRs have timing
	// PR1: branch created 2/28 10:00, opened 3/1 10:00, merged 3/5 14:00
	//   time_to_open = 24h, time_to_merge = (merged - created) = ~100h, total = ~124h
	// PR2: branch 2/25 10:00, opened 3/2 8:00, ready_for_review 3/9 10:00, merged 3/12 10:00
	//   time_to_open = ~118h, draft_time = ~170h, time_to_merge = (merged - ready) = 72h, total = (merged - branch) = ~360h
	assert.Greater(t, result.Summary.Timing.AvgTimeToOpenHours, 0.0)
	assert.Greater(t, result.Summary.Timing.AvgTimeToMergeHours, 0.0)
	assert.Greater(t, result.Summary.Timing.AvgTotalTimeHours, 0.0)
}

func TestCalculateDeveloperStats(t *testing.T) {
	s, repoID := seedTestData(t)

	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)

	result := Calculate(prs, s, CalculateOptions{
		StartDate: "2026-03-01",
		EndDate:   "2026-03-31",
		Repos:     []string{"org/repo"},
	})

	// 3 developers: alice (1 merged, 1 closed), bob (1 merged), charlie (1 open)
	assert.Len(t, result.Developers, 3)

	// Find alice
	var alice, bob *DeveloperStats
	for i := range result.Developers {
		switch result.Developers[i].Login {
		case "alice":
			alice = &result.Developers[i]
		case "bob":
			bob = &result.Developers[i]
		}
	}

	require.NotNil(t, alice)
	assert.Equal(t, 1, alice.MergedPRs) // 1 merged (PR3 was closed, not merged)
	assert.Greater(t, alice.PRsPerWeek, 0.0)

	require.NotNil(t, bob)
	assert.Equal(t, 1, bob.MergedPRs)
	assert.Greater(t, bob.Timing.AvgDraftTimeHours, 0.0) // bob's PR had draft time
}

func TestCalculateSlowestPRs(t *testing.T) {
	s, repoID := seedTestData(t)

	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)

	result := Calculate(prs, s, CalculateOptions{
		StartDate: "2026-03-01",
		EndDate:   "2026-03-31",
		Repos:     []string{"org/repo"},
	})

	// Only merged PRs in slowest list, sorted by total time desc
	assert.Len(t, result.SlowestPRs, 2)
	// PR2 should be slowest (branch created 2/25, merged 3/12)
	assert.Equal(t, 2, result.SlowestPRs[0].Number)
	assert.Equal(t, 1, result.SlowestPRs[1].Number)
	assert.Greater(t, result.SlowestPRs[0].TotalTimeHours, result.SlowestPRs[1].TotalTimeHours)
}

func seedTestDataWithReviews(t *testing.T) (*sqlite.SQLiteStore, int64) {
	t.Helper()
	s, repoID := seedTestData(t)

	// Add timeline events for review metrics
	// PR 1 (alice, merged): reviewed by bob (approved) after 12 hours
	ts1 := "2026-03-01T22:00:00Z" // 12h after PR created at 10:00
	actor1 := "bob"
	reviewState1 := "approved"
	pr1Events := []store.TimelineEventRecord{
		{EventType: "reviewed", CreatedAt: &ts1, Actor: &actor1, ReviewState: &reviewState1, RawData: `{"event":"reviewed","state":"approved"}`},
	}
	s.ReplaceTimelineEvents(1, pr1Events)

	// PR 2 (bob, merged, was draft, ready_for_review at 2026-03-09T10:00:00Z):
	// First review by charlie: changes_requested at 2026-03-09T22:00:00Z (12h after ready)
	// Second review by charlie: approved at 2026-03-10T10:00:00Z
	ts2a := "2026-03-09T22:00:00Z"
	ts2b := "2026-03-10T10:00:00Z"
	actor2 := "charlie"
	reviewState2a := "changes_requested"
	reviewState2b := "approved"
	pr2Events := []store.TimelineEventRecord{
		{EventType: "reviewed", CreatedAt: &ts2a, Actor: &actor2, ReviewState: &reviewState2a, RawData: `{"event":"reviewed","state":"changes_requested"}`},
		{EventType: "reviewed", CreatedAt: &ts2b, Actor: &actor2, ReviewState: &reviewState2b, RawData: `{"event":"reviewed","state":"approved"}`},
	}
	s.ReplaceTimelineEvents(2, pr2Events)

	// Update PR comment counts
	mergedAt1 := "2026-03-05T14:00:00Z"
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "Fast PR", State: "merged", Author: "alice",
		URL: "https://github.com/org/repo/pull/1",
		CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T14:00:00Z",
		MergedAt: &mergedAt1, Additions: 100, Deletions: 20, ChangedFiles: 5,
		CommentCount: 3, ReviewCommentCount: 2,
	})
	readyAt := "2026-03-09T10:00:00Z"
	mergedAt2 := "2026-03-12T10:00:00Z"
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 2, Title: "Slow draft PR", State: "merged", Author: "bob",
		URL: "https://github.com/org/repo/pull/2",
		CreatedAt: "2026-03-02T08:00:00Z", UpdatedAt: "2026-03-12T10:00:00Z",
		MergedAt: &mergedAt2, ReadyForReviewAt: &readyAt,
		Additions: 300, Deletions: 50, ChangedFiles: 12,
		CommentCount: 5, ReviewCommentCount: 4,
	})

	return s, repoID
}

func TestCalculateReviewMetrics(t *testing.T) {
	s, repoID := seedTestDataWithReviews(t)

	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)

	result := Calculate(prs, s, CalculateOptions{
		StartDate: "2026-03-01",
		EndDate:   "2026-03-31",
		Repos:     []string{"org/repo"},
	})

	// Review metrics should be populated
	review := result.Summary.Review

	// Time to first review:
	// PR1: created 3/1 10:00, first review 3/1 22:00 → 12h
	// PR2: ready_for_review 3/9 10:00, first review 3/9 22:00 → 12h
	// Avg = 12h
	assert.InDelta(t, 12.0, review.AvgTimeToFirstReviewHours, 0.5)
	assert.InDelta(t, 12.0, review.MedianTimeToFirstReviewHours, 0.5)

	// Review cycles (changes_requested count):
	// PR1: 0 changes_requested
	// PR2: 1 changes_requested
	// Avg = 0.5
	assert.InDelta(t, 0.5, review.AvgReviewCycles, 0.1)

	// Comments per PR (only merged PRs with comment data):
	// PR1: 3 + 2 = 5
	// PR2: 5 + 4 = 9
	// Avg = 7
	assert.InDelta(t, 7.0, review.AvgCommentsPerPR, 0.1)
}

func TestCalculateReviewerStats(t *testing.T) {
	s, repoID := seedTestDataWithReviews(t)

	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)

	result := Calculate(prs, s, CalculateOptions{
		StartDate: "2026-03-01",
		EndDate:   "2026-03-31",
		Repos:     []string{"org/repo"},
	})

	// Should have reviewer stats for bob and charlie
	require.NotEmpty(t, result.ReviewerStats)

	reviewerMap := make(map[string]ReviewerStats)
	for _, rs := range result.ReviewerStats {
		reviewerMap[rs.Login] = rs
	}

	// bob: 1 review, 1 approval, 0 changes_requested
	bob, ok := reviewerMap["bob"]
	require.True(t, ok, "bob should be in reviewer stats")
	assert.Equal(t, 1, bob.ReviewsGiven)
	assert.Equal(t, 1, bob.Approvals)
	assert.Equal(t, 0, bob.ChangesRequested)

	// charlie: 2 reviews, 1 approval, 1 changes_requested
	charlie, ok := reviewerMap["charlie"]
	require.True(t, ok, "charlie should be in reviewer stats")
	assert.Equal(t, 2, charlie.ReviewsGiven)
	assert.Equal(t, 1, charlie.Approvals)
	assert.Equal(t, 1, charlie.ChangesRequested)
}

func TestCalculateEmptyReviewMetrics(t *testing.T) {
	// With no review events, review metrics should be zero
	s, repoID := seedTestData(t)

	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)

	result := Calculate(prs, s, CalculateOptions{
		StartDate: "2026-03-01",
		EndDate:   "2026-03-31",
		Repos:     []string{"org/repo"},
	})

	assert.Equal(t, 0.0, result.Summary.Review.AvgTimeToFirstReviewHours)
	assert.Equal(t, 0.0, result.Summary.Review.AvgReviewCycles)
	assert.Empty(t, result.ReviewerStats)
}

func TestMedian(t *testing.T) {
	assert.Equal(t, 3.0, median([]float64{1, 3, 5}))
	assert.Equal(t, 2.5, median([]float64{1, 2, 3, 4}))
	assert.Equal(t, 5.0, median([]float64{5}))
	assert.Equal(t, 0.0, median([]float64{}))
}

func TestAvg(t *testing.T) {
	assert.Equal(t, 3.0, avg([]float64{1, 3, 5}))
	assert.Equal(t, 0.0, avg([]float64{}))
}
