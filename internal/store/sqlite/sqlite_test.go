package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/Arsenalist/prx/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *SQLiteStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s := New(path)
	require.NoError(t, s.Open())
	require.NoError(t, s.Migrate())
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenAndMigrate(t *testing.T) {
	s := newTestDB(t)
	stats, err := s.Stats()
	require.NoError(t, err)
	assert.NotEmpty(t, stats.DatabasePath)
	// All tables should exist with 0 rows
	for _, table := range []string{"instances", "repositories", "teams", "pull_requests", "branch_info", "file_changes", "timeline_events", "fetch_metadata"} {
		assert.Contains(t, stats.Tables, table, "table %s should exist", table)
		assert.Equal(t, int64(0), stats.Tables[table])
	}
}

func TestUpsertInstance(t *testing.T) {
	s := newTestDB(t)

	id, err := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)

	// Upsert same name should return same ID
	id2, err := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://updated.github.com",
	})
	require.NoError(t, err)
	assert.Equal(t, id, id2)

	// Verify update happened
	inst, err := s.GetInstanceByName("default")
	require.NoError(t, err)
	assert.Equal(t, "https://updated.github.com", inst.BaseURL)
}

func TestUpsertRepository(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})

	id, err := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)

	// Upsert same should return same ID
	id2, err := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})
	require.NoError(t, err)
	assert.Equal(t, id, id2)

	// List repos
	repos, err := s.ListRepositories()
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "org/repo", repos[0].FullName)
}

func TestUpsertPullRequest(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})

	rawJSON := `{"id": 123, "labels": [{"name": "bug"}]}`
	pr := store.PullRequestRecord{
		RepoID:    repoID,
		Number:    42,
		Title:     "Fix the bug",
		State:     "merged",
		Author:    "alice",
		URL:       "https://github.com/org/repo/pull/42",
		CreatedAt: "2026-03-01T10:00:00Z",
		UpdatedAt: "2026-03-05T14:00:00Z",
		Additions: 100,
		Deletions: 20,
		RawData:   &rawJSON,
	}

	id, err := s.UpsertPullRequest(pr)
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)

	// Upsert same PR (same repo+number) should update, return same ID
	pr.Title = "Fix the bug (updated)"
	pr.State = "closed"
	id2, err := s.UpsertPullRequest(pr)
	require.NoError(t, err)
	assert.Equal(t, id, id2)

	// Verify the update
	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, "Fix the bug (updated)", prs[0].Title)
	assert.Equal(t, "closed", prs[0].State)
	assert.NotNil(t, prs[0].RawData)
	assert.Contains(t, *prs[0].RawData, "labels")
}

func TestGetPRState(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})

	// Non-existent PR
	result, err := s.GetPRState(repoID, 999)
	require.NoError(t, err)
	assert.Nil(t, result)

	// Insert a PR
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "PR 1", State: "merged", Author: "alice",
		URL: "https://example.com/1", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T14:00:00Z",
	})

	result, err = s.GetPRState(repoID, 1)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "merged", result.State)
	assert.Equal(t, "2026-03-05T14:00:00Z", result.UpdatedAt)
}

func TestBranchInfo(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})
	prID, _ := s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "PR 1", State: "open", Author: "alice",
		URL: "https://example.com/1", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T14:00:00Z",
	})

	sha := "abc123"
	date := "2026-02-28T08:00:00Z"
	err := s.UpsertBranchInfo(store.BranchInfoRecord{
		PRID: prID, MergeBaseSHA: &sha, FirstCommitDate: &date,
		CommitsCount: 5, TotalAdditions: 200, TotalDeletions: 50,
	})
	require.NoError(t, err)

	// Upsert again (update)
	err = s.UpsertBranchInfo(store.BranchInfoRecord{
		PRID: prID, MergeBaseSHA: &sha, FirstCommitDate: &date,
		CommitsCount: 7, TotalAdditions: 250, TotalDeletions: 60,
	})
	require.NoError(t, err)
}

func TestFileChanges(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})
	prID, _ := s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "PR 1", State: "open", Author: "alice",
		URL: "https://example.com/1", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T14:00:00Z",
	})

	files := []store.FileChangeRecord{
		{Filename: "main.go", Additions: 50, Deletions: 10, IsTest: false},
		{Filename: "main_test.go", Additions: 30, Deletions: 5, IsTest: true},
	}
	err := s.ReplaceFileChanges(prID, files)
	require.NoError(t, err)

	// Replace should delete old and insert new
	files2 := []store.FileChangeRecord{
		{Filename: "handler.go", Additions: 20, Deletions: 3, IsTest: false},
	}
	err = s.ReplaceFileChanges(prID, files2)
	require.NoError(t, err)

	// Verify via raw query
	rows, err := s.RawQuery("SELECT COUNT(*) as cnt FROM file_changes WHERE pr_id = 1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows[0]["cnt"])
}

func TestTimelineEvents(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})
	prID, _ := s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "PR 1", State: "open", Author: "alice",
		URL: "https://example.com/1", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T14:00:00Z",
	})

	ts := "2026-03-02T10:00:00Z"
	actor := "alice"
	events := []store.TimelineEventRecord{
		{EventType: "ready_for_review", CreatedAt: &ts, Actor: &actor, RawData: `{"event":"ready_for_review"}`},
		{EventType: "reviewed", CreatedAt: &ts, Actor: &actor, RawData: `{"event":"reviewed"}`},
	}
	err := s.ReplaceTimelineEvents(prID, events)
	require.NoError(t, err)

	// Replace should delete old and insert new
	err = s.ReplaceTimelineEvents(prID, events[:1])
	require.NoError(t, err)

	rows, err := s.RawQuery("SELECT COUNT(*) as cnt FROM timeline_events WHERE pr_id = 1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows[0]["cnt"])
}

func TestFetchMetadata(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})

	// No metadata yet
	meta, err := s.GetFetchMetadata(repoID)
	require.NoError(t, err)
	assert.Nil(t, meta)

	// Set metadata
	err = s.UpdateFetchMetadata(store.FetchMetadataRecord{
		RepoID: repoID, LastFetchAt: "2026-03-25T10:00:00Z", LastUpdatedAt: "2026-03-24T14:00:00Z", PRCount: 42,
	})
	require.NoError(t, err)

	// Read back
	meta, err = s.GetFetchMetadata(repoID)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, 42, meta.PRCount)
	assert.Equal(t, "2026-03-25T10:00:00Z", meta.LastFetchAt)

	// Update
	err = s.UpdateFetchMetadata(store.FetchMetadataRecord{
		RepoID: repoID, LastFetchAt: "2026-03-25T12:00:00Z", LastUpdatedAt: "2026-03-25T11:00:00Z", PRCount: 50,
	})
	require.NoError(t, err)

	meta, err = s.GetFetchMetadata(repoID)
	require.NoError(t, err)
	assert.Equal(t, 50, meta.PRCount)
}

func TestListPullRequestsFilters(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})

	// Insert several PRs
	for i, pr := range []store.PullRequestRecord{
		{RepoID: repoID, Number: 1, Title: "PR 1", State: "merged", Author: "alice", URL: "u", CreatedAt: "2026-01-15T10:00:00Z", UpdatedAt: "2026-01-20T10:00:00Z"},
		{RepoID: repoID, Number: 2, Title: "PR 2", State: "open", Author: "bob", URL: "u", CreatedAt: "2026-02-10T10:00:00Z", UpdatedAt: "2026-02-15T10:00:00Z"},
		{RepoID: repoID, Number: 3, Title: "PR 3", State: "merged", Author: "alice", URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z"},
		{RepoID: repoID, Number: 4, Title: "PR 4", State: "closed", Author: "charlie", URL: "u", CreatedAt: "2026-03-10T10:00:00Z", UpdatedAt: "2026-03-12T10:00:00Z"},
	} {
		_, err := s.UpsertPullRequest(pr)
		require.NoError(t, err, "PR %d", i)
	}

	// No filters — all PRs
	prs, err := s.ListPullRequests(store.PRFilters{})
	require.NoError(t, err)
	assert.Len(t, prs, 4)

	// Filter by repo
	prs, err = s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)
	assert.Len(t, prs, 4)

	// Filter by author
	prs, err = s.ListPullRequests(store.PRFilters{Authors: []string{"alice"}})
	require.NoError(t, err)
	assert.Len(t, prs, 2)

	// Filter by state
	prs, err = s.ListPullRequests(store.PRFilters{States: []string{"merged"}})
	require.NoError(t, err)
	assert.Len(t, prs, 2)

	// Filter by date range
	prs, err = s.ListPullRequests(store.PRFilters{StartDate: "2026-02-01", EndDate: "2026-02-28"})
	require.NoError(t, err)
	assert.Len(t, prs, 1)
	assert.Equal(t, 2, prs[0].Number)

	// Combined filters
	prs, err = s.ListPullRequests(store.PRFilters{
		Authors: []string{"alice"}, States: []string{"merged"}, StartDate: "2026-02-01",
	})
	require.NoError(t, err)
	assert.Len(t, prs, 1)
	assert.Equal(t, 3, prs[0].Number)
}

func TestRawQuery(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})

	raw := `{"user":{"login":"alice"},"labels":[{"name":"bug"}]}`
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "PR 1", State: "merged", Author: "alice",
		URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z",
		RawData: &raw,
	})

	// Query using json_extract on the raw blob
	rows, err := s.RawQuery("SELECT number, json_extract(raw_data, '$.user.login') as login FROM pull_requests")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(1), rows[0]["number"])
	assert.Equal(t, "alice", rows[0]["login"])
}

func TestTeams(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repo1ID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "core", FullName: "org/core",
	})
	repo2ID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "infra", FullName: "org/infra",
	})

	teamID, err := s.UpsertTeam(store.TeamRecord{Name: "platform", DisplayName: "Platform Engineering"})
	require.NoError(t, err)

	err = s.SetTeamRepos(teamID, []int64{repo1ID, repo2ID})
	require.NoError(t, err)

	// Upsert again should not error
	teamID2, err := s.UpsertTeam(store.TeamRecord{Name: "platform", DisplayName: "Platform Eng (updated)"})
	require.NoError(t, err)
	assert.Equal(t, teamID, teamID2)
}

func TestStoreStats(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "PR 1", State: "open", Author: "alice",
		URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T10:00:00Z",
	})

	stats, err := s.Stats()
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.Tables["instances"])
	assert.Equal(t, int64(1), stats.Tables["repositories"])
	assert.Equal(t, int64(1), stats.Tables["pull_requests"])
	assert.Greater(t, stats.SizeBytes, int64(0))
}
