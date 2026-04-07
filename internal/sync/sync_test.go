package sync

import (
	"path/filepath"
	"testing"

	"github.com/Arsenalist/prx/internal/provider"
	"github.com/Arsenalist/prx/internal/store"
	"github.com/Arsenalist/prx/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements provider.VCSProvider for testing.
type mockProvider struct {
	prs      []provider.PullRequest
	fullPRs  map[int]*provider.PullRequest // number -> full details
	compares map[string]*provider.BranchComparison
	timeline map[int][]provider.TimelineEvent
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) ListPullRequests(owner, repo string, opts provider.ListPROptions) ([]provider.PullRequest, error) {
	return m.prs, nil
}

func (m *mockProvider) GetPullRequest(owner, repo string, number int) (*provider.PullRequest, error) {
	if pr, ok := m.fullPRs[number]; ok {
		return pr, nil
	}
	// Return from list data
	for _, pr := range m.prs {
		if pr.Number == number {
			return &pr, nil
		}
	}
	return nil, nil
}

func (m *mockProvider) GetBranchComparison(owner, repo, base, head string) (*provider.BranchComparison, error) {
	key := base + "..." + head
	if comp, ok := m.compares[key]; ok {
		return comp, nil
	}
	return &provider.BranchComparison{}, nil
}

func (m *mockProvider) GetTimelineEvents(owner, repo string, number int) ([]provider.TimelineEvent, error) {
	if events, ok := m.timeline[number]; ok {
		return events, nil
	}
	return nil, nil
}

func (m *mockProvider) GetRateLimit() (*provider.RateLimit, error) {
	return &provider.RateLimit{Limit: 5000, Remaining: 4999, ResetAt: "2026-03-25T12:00:00Z"}, nil
}

func newTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s := sqlite.New(path)
	require.NoError(t, s.Open())
	require.NoError(t, s.Migrate())
	t.Cleanup(func() { s.Close() })
	return s
}

func setupRepo(t *testing.T, s *sqlite.SQLiteStore) (int64, int64) {
	t.Helper()
	instID, err := s.UpsertInstance(store.InstanceRecord{Name: "default", Type: "github", BaseURL: "https://api.github.com"})
	require.NoError(t, err)
	repoID, err := s.UpsertRepository(store.RepositoryRecord{InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo"})
	require.NoError(t, err)
	return instID, repoID
}

func TestSyncNewPRs(t *testing.T) {
	s := newTestStore(t)
	instID, repoID := setupRepo(t, s)
	_ = instID

	mock := &mockProvider{
		prs: []provider.PullRequest{
			{Number: 1, Title: "PR 1", State: "merged", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z", BaseSHA: "aaa", HeadSHA: "bbb"},
			{Number: 2, Title: "PR 2", State: "open", Author: "bob", URL: "u", CreatedAt: "2026-03-10T10:00:00Z", UpdatedAt: "2026-03-15T10:00:00Z", BaseSHA: "ccc", HeadSHA: "ddd"},
		},
		fullPRs: map[int]*provider.PullRequest{
			1: {Number: 1, Title: "PR 1", State: "merged", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z", Additions: 100, Deletions: 20, BaseSHA: "aaa", HeadSHA: "bbb", RawJSON: `{"number":1}`},
			2: {Number: 2, Title: "PR 2", State: "open", Author: "bob", URL: "u", CreatedAt: "2026-03-10T10:00:00Z", UpdatedAt: "2026-03-15T10:00:00Z", Additions: 50, Deletions: 5, BaseSHA: "ccc", HeadSHA: "ddd", RawJSON: `{"number":2}`},
		},
		compares: map[string]*provider.BranchComparison{
			"aaa...bbb": {MergeBaseSHA: "aaa", FirstCommitDate: "2026-02-28T08:00:00Z", CommitsCount: 3, TotalAdditions: 100, TotalDeletions: 20, Files: []provider.FileChange{{Filename: "main.go", Additions: 100, Deletions: 20}}},
			"ccc...ddd": {MergeBaseSHA: "ccc", CommitsCount: 1, Files: []provider.FileChange{{Filename: "handler_test.go", Additions: 50, Deletions: 5}}},
		},
	}

	engine := NewEngine(mock, s)
	result, err := engine.SyncRepo(1, repoID, "org", "repo", Options{
		PerPage:      100,
		TestPatterns: CompileTestPatterns([]string{"_test\\.go$"}),
	})

	require.NoError(t, err)
	assert.Equal(t, 2, result.New)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 0, result.Skipped)

	// Verify stored data
	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)
	assert.Len(t, prs, 2)

	// Verify raw_data stored
	assert.NotNil(t, prs[0].RawData)

	// Verify file changes classified correctly
	rows, err := s.RawQuery("SELECT filename, is_test FROM file_changes ORDER BY filename")
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	// handler_test.go should be classified as test
	for _, row := range rows {
		if row["filename"] == "handler_test.go" {
			assert.Equal(t, int64(1), row["is_test"])
		}
		if row["filename"] == "main.go" {
			assert.Equal(t, int64(0), row["is_test"])
		}
	}
}

func TestSyncSkipsClosedPRs(t *testing.T) {
	s := newTestStore(t)
	_, repoID := setupRepo(t, s)

	// Pre-populate a merged PR in the DB
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "Already Merged", State: "merged", Author: "alice",
		URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z",
	})

	mock := &mockProvider{
		prs: []provider.PullRequest{
			// Same PR comes back from API as closed
			{Number: 1, Title: "Already Merged", State: "merged", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z"},
			// New open PR
			{Number: 2, Title: "New PR", State: "open", Author: "bob", URL: "u", CreatedAt: "2026-03-10T10:00:00Z", UpdatedAt: "2026-03-15T10:00:00Z", BaseSHA: "a", HeadSHA: "b"},
		},
		fullPRs: map[int]*provider.PullRequest{
			2: {Number: 2, Title: "New PR", State: "open", Author: "bob", URL: "u", CreatedAt: "2026-03-10T10:00:00Z", UpdatedAt: "2026-03-15T10:00:00Z", Additions: 30, BaseSHA: "a", HeadSHA: "b"},
		},
		compares: map[string]*provider.BranchComparison{
			"a...b": {},
		},
	}

	engine := NewEngine(mock, s)
	result, err := engine.SyncRepo(1, repoID, "org", "repo", Options{PerPage: 100})

	require.NoError(t, err)
	assert.Equal(t, 1, result.New)     // PR #2 is new
	assert.Equal(t, 0, result.Updated) // nothing updated
	assert.Equal(t, 1, result.Skipped) // PR #1 skipped (merged in both API and DB)
}

func TestSyncRefetchesOpenPRs(t *testing.T) {
	s := newTestStore(t)
	_, repoID := setupRepo(t, s)

	// Pre-populate an open PR
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "WIP", State: "open", Author: "alice",
		URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z",
	})

	mock := &mockProvider{
		prs: []provider.PullRequest{
			// Same PR still open but updated
			{Number: 1, Title: "WIP (updated)", State: "open", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-20T10:00:00Z", BaseSHA: "a", HeadSHA: "b"},
		},
		fullPRs: map[int]*provider.PullRequest{
			1: {Number: 1, Title: "WIP (updated)", State: "open", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-20T10:00:00Z", Additions: 200, BaseSHA: "a", HeadSHA: "b"},
		},
		compares: map[string]*provider.BranchComparison{"a...b": {}},
	}

	engine := NewEngine(mock, s)
	result, err := engine.SyncRepo(1, repoID, "org", "repo", Options{PerPage: 100})

	require.NoError(t, err)
	assert.Equal(t, 0, result.New)
	assert.Equal(t, 1, result.Updated) // Re-fetched because it's open
	assert.Equal(t, 0, result.Skipped)

	// Verify the title was updated
	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)
	assert.Equal(t, "WIP (updated)", prs[0].Title)
}

func TestSyncRefetchesNewlyMergedPR(t *testing.T) {
	s := newTestStore(t)
	_, repoID := setupRepo(t, s)

	// PR was open in DB
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "Feature", State: "open", Author: "alice",
		URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z",
	})

	// API says it's now merged
	mergedAt := "2026-03-20T14:00:00Z"
	mock := &mockProvider{
		prs: []provider.PullRequest{
			{Number: 1, Title: "Feature", State: "merged", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-20T14:00:00Z", MergedAt: &mergedAt, BaseSHA: "a", HeadSHA: "b"},
		},
		fullPRs: map[int]*provider.PullRequest{
			1: {Number: 1, Title: "Feature", State: "merged", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-20T14:00:00Z", MergedAt: &mergedAt, Additions: 150, BaseSHA: "a", HeadSHA: "b"},
		},
		compares: map[string]*provider.BranchComparison{"a...b": {}},
	}

	engine := NewEngine(mock, s)
	result, err := engine.SyncRepo(1, repoID, "org", "repo", Options{PerPage: 100})

	require.NoError(t, err)
	assert.Equal(t, 0, result.New)
	assert.Equal(t, 1, result.Updated) // Was open, now merged → re-fetch
	assert.Equal(t, 0, result.Skipped)
}

func TestSyncFullMode(t *testing.T) {
	s := newTestStore(t)
	_, repoID := setupRepo(t, s)

	// Pre-populate a merged PR
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "Old", State: "merged", Author: "alice",
		URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z",
	})

	mock := &mockProvider{
		prs: []provider.PullRequest{
			{Number: 1, Title: "Old", State: "merged", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z", BaseSHA: "a", HeadSHA: "b"},
		},
		fullPRs: map[int]*provider.PullRequest{
			1: {Number: 1, Title: "Old", State: "merged", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z", BaseSHA: "a", HeadSHA: "b"},
		},
		compares: map[string]*provider.BranchComparison{"a...b": {}},
	}

	engine := NewEngine(mock, s)
	result, err := engine.SyncRepo(1, repoID, "org", "repo", Options{Full: true, PerPage: 100})

	require.NoError(t, err)
	assert.Equal(t, 1, result.New) // Full mode fetches everything (1 PR, counted as new)
	assert.Equal(t, 0, result.Skipped)
}

func TestSyncDryRun(t *testing.T) {
	s := newTestStore(t)
	_, repoID := setupRepo(t, s)

	mock := &mockProvider{
		prs: []provider.PullRequest{
			{Number: 1, Title: "New", State: "open", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z"},
		},
	}

	engine := NewEngine(mock, s)
	result, err := engine.SyncRepo(1, repoID, "org", "repo", Options{DryRun: true, PerPage: 100})

	require.NoError(t, err)
	assert.Equal(t, 1, result.New)

	// DB should still be empty (dry run)
	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)
	assert.Len(t, prs, 0)
}

func TestSyncTimelineExtractsReadyForReview(t *testing.T) {
	s := newTestStore(t)
	_, repoID := setupRepo(t, s)

	mock := &mockProvider{
		prs: []provider.PullRequest{
			{Number: 1, Title: "Draft PR", State: "open", Author: "alice", URL: "u", IsDraft: false, CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-15T10:00:00Z", BaseSHA: "a", HeadSHA: "b"},
		},
		fullPRs: map[int]*provider.PullRequest{
			1: {Number: 1, Title: "Draft PR", State: "open", Author: "alice", URL: "u", IsDraft: false, CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-15T10:00:00Z", Additions: 50, BaseSHA: "a", HeadSHA: "b"},
		},
		compares: map[string]*provider.BranchComparison{"a...b": {}},
		timeline: map[int][]provider.TimelineEvent{
			1: {
				{EventType: "committed", CreatedAt: "2026-03-01T10:00:00Z", Actor: "alice", RawJSON: `{}`},
				{EventType: "ready_for_review", CreatedAt: "2026-03-05T14:00:00Z", Actor: "alice", RawJSON: `{"event":"ready_for_review"}`},
				{EventType: "reviewed", CreatedAt: "2026-03-06T10:00:00Z", Actor: "bob", RawJSON: `{}`},
			},
		},
	}

	engine := NewEngine(mock, s)
	_, err := engine.SyncRepo(1, repoID, "org", "repo", Options{PerPage: 100})
	require.NoError(t, err)

	// Verify ready_for_review_at was extracted
	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)
	require.Len(t, prs, 1)
	require.NotNil(t, prs[0].ReadyForReviewAt)
	assert.Equal(t, "2026-03-05T14:00:00Z", *prs[0].ReadyForReviewAt)
}

func TestSyncMapsNewProviderFields(t *testing.T) {
	s := newTestStore(t)
	_, repoID := setupRepo(t, s)

	mergedAt := "2026-03-05T14:00:00Z"
	mock := &mockProvider{
		prs: []provider.PullRequest{
			{Number: 1, Title: "PR 1", State: "merged", Author: "alice", URL: "u",
				CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z",
				MergedAt: &mergedAt, BaseSHA: "aaa", HeadSHA: "bbb",
				MergedBy: "bob", CommentCount: 5, ReviewCommentCount: 3},
		},
		fullPRs: map[int]*provider.PullRequest{
			1: {Number: 1, Title: "PR 1", State: "merged", Author: "alice", URL: "u",
				CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z",
				MergedAt: &mergedAt, Additions: 100, Deletions: 20, BaseSHA: "aaa", HeadSHA: "bbb",
				MergedBy: "bob", CommentCount: 5, ReviewCommentCount: 3,
				RawJSON: `{"number":1,"merged_by":{"login":"bob"},"comments":5,"review_comments":3}`},
		},
		compares: map[string]*provider.BranchComparison{
			"aaa...bbb": {MergeBaseSHA: "aaa", CommitsCount: 1},
		},
		timeline: map[int][]provider.TimelineEvent{
			1: {
				{EventType: "reviewed", CreatedAt: "2026-03-03T10:00:00Z", Actor: "reviewer1",
					ReviewState: "approved", RawJSON: `{"event":"reviewed","state":"approved"}`},
			},
		},
	}

	engine := NewEngine(mock, s)
	_, err := engine.SyncRepo(1, repoID, "org", "repo", Options{PerPage: 100})
	require.NoError(t, err)

	// Verify new PR fields stored
	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)
	require.Len(t, prs, 1)
	require.NotNil(t, prs[0].MergedBy)
	assert.Equal(t, "bob", *prs[0].MergedBy)
	assert.Equal(t, 5, prs[0].CommentCount)
	assert.Equal(t, 3, prs[0].ReviewCommentCount)

	// Verify review_state stored in timeline events
	rows, err := s.RawQuery("SELECT review_state FROM timeline_events WHERE event_type = 'reviewed'")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "approved", rows[0]["review_state"])
}

func TestSyncSinceOverridesDBMetadata(t *testing.T) {
	s := newTestStore(t)
	_, repoID := setupRepo(t, s)

	// Set fetch metadata so incremental since would be "2026-03-01T00:00:00Z"
	s.UpdateFetchMetadata(store.FetchMetadataRecord{
		RepoID:        repoID,
		LastFetchAt:   "2026-03-10T00:00:00Z",
		LastUpdatedAt: "2026-03-01T00:00:00Z",
		PRCount:       5,
	})

	var capturedOpts provider.ListPROptions
	mock := &mockProviderCapture{
		mockProvider: mockProvider{
			prs: []provider.PullRequest{},
		},
		onList: func(opts provider.ListPROptions) {
			capturedOpts = opts
		},
	}

	engine := NewEngine(mock, s)
	_, err := engine.SyncRepo(1, repoID, "org", "repo", Options{
		PerPage: 100,
		Since:   "2026-02-01T00:00:00Z", // Earlier than DB metadata
	})

	require.NoError(t, err)
	// Command-line since should override DB metadata's "2026-03-01"
	assert.Equal(t, "2026-02-01T00:00:00Z", capturedOpts.Since)
}

func TestSyncUntilPassedToProvider(t *testing.T) {
	s := newTestStore(t)
	_, repoID := setupRepo(t, s)

	var capturedOpts provider.ListPROptions
	mock := &mockProviderCapture{
		mockProvider: mockProvider{
			prs: []provider.PullRequest{},
		},
		onList: func(opts provider.ListPROptions) {
			capturedOpts = opts
		},
	}

	engine := NewEngine(mock, s)
	_, err := engine.SyncRepo(1, repoID, "org", "repo", Options{
		PerPage: 100,
		Since:   "2026-01-01T00:00:00Z",
		Until:   "2026-03-31T00:00:00Z",
	})

	require.NoError(t, err)
	assert.Equal(t, "2026-01-01T00:00:00Z", capturedOpts.Since)
	assert.Equal(t, "2026-03-31T00:00:00Z", capturedOpts.Until)
}

func TestSyncWithoutSinceUsesDBMetadata(t *testing.T) {
	s := newTestStore(t)
	_, repoID := setupRepo(t, s)

	// Set fetch metadata
	s.UpdateFetchMetadata(store.FetchMetadataRecord{
		RepoID:        repoID,
		LastFetchAt:   "2026-03-10T00:00:00Z",
		LastUpdatedAt: "2026-03-05T00:00:00Z",
		PRCount:       5,
	})

	var capturedOpts provider.ListPROptions
	mock := &mockProviderCapture{
		mockProvider: mockProvider{
			prs: []provider.PullRequest{},
		},
		onList: func(opts provider.ListPROptions) {
			capturedOpts = opts
		},
	}

	engine := NewEngine(mock, s)
	_, err := engine.SyncRepo(1, repoID, "org", "repo", Options{
		PerPage: 100,
		// No Since set — should fall back to DB metadata
	})

	require.NoError(t, err)
	assert.Equal(t, "2026-03-05T00:00:00Z", capturedOpts.Since)
}

// mockProviderCapture wraps mockProvider to capture ListPullRequests options.
type mockProviderCapture struct {
	mockProvider
	onList func(opts provider.ListPROptions)
}

func (m *mockProviderCapture) ListPullRequests(owner, repo string, opts provider.ListPROptions) ([]provider.PullRequest, error) {
	if m.onList != nil {
		m.onList(opts)
	}
	return m.mockProvider.ListPullRequests(owner, repo, opts)
}

func TestCompileTestPatterns(t *testing.T) {
	patterns := CompileTestPatterns([]string{
		"_test\\.go$",
		"\\.test\\.ts$",
		"[invalid",    // Should be skipped
		"/__tests__/",
	})
	assert.Len(t, patterns, 3) // Invalid one dropped
}
