package sqlite

import (
	"fmt"
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

func TestListPullRequestsDateRangeIncludesActivity(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{
		Name: "default", Type: "github", BaseURL: "https://api.github.com",
	})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{
		InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo",
	})

	// PR created before range, but updated during range
	_, err := s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "Old PR updated in range", State: "open", Author: "alice", URL: "u",
		CreatedAt: "2026-01-05T10:00:00Z", UpdatedAt: "2026-02-10T10:00:00Z",
	})
	require.NoError(t, err)

	// PR created before range, merged during range
	mergedAt := "2026-02-05T10:00:00Z"
	_, err = s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 2, Title: "Old PR merged in range", State: "merged", Author: "bob", URL: "u",
		CreatedAt: "2026-01-10T10:00:00Z", UpdatedAt: "2026-01-15T10:00:00Z", MergedAt: &mergedAt,
	})
	require.NoError(t, err)

	// PR created before range, closed during range
	closedAt := "2026-02-20T10:00:00Z"
	_, err = s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 3, Title: "Old PR closed in range", State: "closed", Author: "charlie", URL: "u",
		CreatedAt: "2026-01-03T10:00:00Z", UpdatedAt: "2026-01-04T10:00:00Z", ClosedAt: &closedAt,
	})
	require.NoError(t, err)

	// PR created during range
	_, err = s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 4, Title: "PR created in range", State: "open", Author: "alice", URL: "u",
		CreatedAt: "2026-02-12T10:00:00Z", UpdatedAt: "2026-02-12T10:00:00Z",
	})
	require.NoError(t, err)

	// PR with no activity in range (created and last updated before range)
	oldMergedAt := "2025-12-10T10:00:00Z"
	_, err = s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 5, Title: "Completely outside range", State: "merged", Author: "alice", URL: "u",
		CreatedAt: "2025-12-01T10:00:00Z", UpdatedAt: "2025-12-05T10:00:00Z", MergedAt: &oldMergedAt,
	})
	require.NoError(t, err)

	// Date range: Feb 2026
	prs, err := s.ListPullRequests(store.PRFilters{StartDate: "2026-02-01", EndDate: "2026-02-28"})
	require.NoError(t, err)

	// Should include PRs 1-4 (all had activity in Feb), exclude PR 5
	assert.Len(t, prs, 4)
	numbers := make([]int, len(prs))
	for i, pr := range prs {
		numbers[i] = pr.Number
	}
	assert.Contains(t, numbers, 1, "PR updated in range should be included")
	assert.Contains(t, numbers, 2, "PR merged in range should be included")
	assert.Contains(t, numbers, 3, "PR closed in range should be included")
	assert.Contains(t, numbers, 4, "PR created in range should be included")
	assert.NotContains(t, numbers, 5, "PR with no activity in range should be excluded")
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

func TestListAndDeleteInstances(t *testing.T) {
	s := newTestDB(t)
	s.UpsertInstance(store.InstanceRecord{Name: "github", Type: "github", BaseURL: "https://api.github.com", TokenEnv: "GH_TOKEN"})
	s.UpsertInstance(store.InstanceRecord{Name: "enterprise", Type: "github", BaseURL: "https://ghe.corp.com/api/v3", TokenEnv: "GHE_TOKEN", TLSSkipVerify: true})

	instances, err := s.ListInstances()
	require.NoError(t, err)
	assert.Len(t, instances, 2)
	assert.Equal(t, "enterprise", instances[0].Name) // ordered by name
	assert.Equal(t, "GHE_TOKEN", instances[0].TokenEnv)
	assert.True(t, instances[0].TLSSkipVerify)
	assert.Equal(t, "github", instances[1].Name)
	assert.Equal(t, "GH_TOKEN", instances[1].TokenEnv)
	assert.False(t, instances[1].TLSSkipVerify)

	// Delete
	require.NoError(t, s.DeleteInstance("enterprise"))
	instances, _ = s.ListInstances()
	assert.Len(t, instances, 1)

	// Delete non-existent
	assert.Error(t, s.DeleteInstance("nope"))
}

func TestDeleteRepository(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{Name: "gh", Type: "github", BaseURL: "https://api.github.com"})
	s.UpsertRepository(store.RepositoryRecord{InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo"})

	repos, _ := s.ListRepositories()
	assert.Len(t, repos, 1)

	require.NoError(t, s.DeleteRepository(instID, "org/repo"))
	repos, _ = s.ListRepositories()
	assert.Len(t, repos, 0)

	assert.Error(t, s.DeleteRepository(instID, "nope"))
}

func TestTeamCRUD(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{Name: "gh", Type: "github", BaseURL: "https://api.github.com"})
	repoID1, _ := s.UpsertRepository(store.RepositoryRecord{InstanceID: instID, Owner: "org", Name: "api", FullName: "org/api"})
	repoID2, _ := s.UpsertRepository(store.RepositoryRecord{InstanceID: instID, Owner: "org", Name: "web", FullName: "org/web"})

	// Create team
	teamID, err := s.UpsertTeam(store.TeamRecord{Name: "platform", DisplayName: "Platform Team"})
	require.NoError(t, err)

	// List teams
	teams, err := s.ListTeams()
	require.NoError(t, err)
	assert.Len(t, teams, 1)
	assert.Equal(t, "platform", teams[0].Name)
	assert.Equal(t, "Platform Team", teams[0].DisplayName)

	// Get by name
	team, err := s.GetTeamByName("platform")
	require.NoError(t, err)
	assert.Equal(t, teamID, team.ID)

	// Not found
	team, err = s.GetTeamByName("nope")
	require.NoError(t, err)
	assert.Nil(t, team)

	// Add repos
	require.NoError(t, s.AddTeamRepo(teamID, repoID1))
	require.NoError(t, s.AddTeamRepo(teamID, repoID2))
	// Duplicate add is no-op
	require.NoError(t, s.AddTeamRepo(teamID, repoID1))

	repos, err := s.GetTeamRepos(teamID)
	require.NoError(t, err)
	assert.Len(t, repos, 2)

	// Remove repo
	require.NoError(t, s.RemoveTeamRepo(teamID, repoID1))
	repos, _ = s.GetTeamRepos(teamID)
	assert.Len(t, repos, 1)
	assert.Equal(t, "org/web", repos[0].FullName)

	// Remove non-existent
	assert.Error(t, s.RemoveTeamRepo(teamID, repoID1))

	// Delete team
	require.NoError(t, s.DeleteTeam("platform"))
	teams, _ = s.ListTeams()
	assert.Len(t, teams, 0)

	// Delete non-existent
	assert.Error(t, s.DeleteTeam("nope"))
}

func TestSettingsCRUD(t *testing.T) {
	s := newTestDB(t)

	// Get missing key returns empty
	val, err := s.GetSetting("fetch.per_page")
	require.NoError(t, err)
	assert.Equal(t, "", val)

	// Set and get
	require.NoError(t, s.SetSetting("fetch.per_page", "100"))
	val, _ = s.GetSetting("fetch.per_page")
	assert.Equal(t, "100", val)

	// Update
	require.NoError(t, s.SetSetting("fetch.per_page", "50"))
	val, _ = s.GetSetting("fetch.per_page")
	assert.Equal(t, "50", val)

	// Set multiple
	require.NoError(t, s.SetSetting("output.format", "json"))
	require.NoError(t, s.SetSetting("date_range.preset", "last-30d"))

	// List
	settings, err := s.ListSettings()
	require.NoError(t, err)
	assert.Len(t, settings, 3)
	assert.Equal(t, "50", settings["fetch.per_page"])
	assert.Equal(t, "json", settings["output.format"])

	// Delete
	require.NoError(t, s.DeleteSetting("fetch.per_page"))
	val, _ = s.GetSetting("fetch.per_page")
	assert.Equal(t, "", val)

	settings, _ = s.ListSettings()
	assert.Len(t, settings, 2)
}

func TestMigrateV3AddsNewColumns(t *testing.T) {
	s := newTestDB(t) // newTestDB runs Migrate() which should apply V3

	// Verify new columns exist by inserting and reading PR with new fields
	instID, _ := s.UpsertInstance(store.InstanceRecord{Name: "gh", Type: "github", BaseURL: "https://api.github.com"})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo"})

	mergedAt := "2026-03-05T14:00:00Z"
	mergedBy := "bob"
	prID, err := s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "PR 1", State: "merged", Author: "alice",
		URL: "https://example.com/1", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T14:00:00Z",
		MergedAt: &mergedAt, MergedBy: &mergedBy, CommentCount: 5, ReviewCommentCount: 3,
	})
	require.NoError(t, err)

	// Read back and verify new fields
	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)
	require.Len(t, prs, 1)
	require.NotNil(t, prs[0].MergedBy)
	assert.Equal(t, "bob", *prs[0].MergedBy)
	assert.Equal(t, 5, prs[0].CommentCount)
	assert.Equal(t, 3, prs[0].ReviewCommentCount)

	// Verify timeline events with review_state
	ts := "2026-03-03T10:00:00Z"
	actor := "reviewer"
	events := []store.TimelineEventRecord{
		{EventType: "reviewed", CreatedAt: &ts, Actor: &actor, ReviewState: strPtr("approved"), RawData: `{"event":"reviewed","state":"approved"}`},
		{EventType: "reviewed", CreatedAt: &ts, Actor: &actor, ReviewState: strPtr("changes_requested"), RawData: `{"event":"reviewed","state":"changes_requested"}`},
	}
	err = s.ReplaceTimelineEvents(prID, events)
	require.NoError(t, err)

	// Verify review_state was stored
	rows, err := s.RawQuery("SELECT event_type, review_state FROM timeline_events WHERE pr_id = " + fmt.Sprintf("%d", prID) + " ORDER BY id")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "approved", rows[0]["review_state"])
	assert.Equal(t, "changes_requested", rows[1]["review_state"])

	// Verify the index exists
	indexRows, err := s.RawQuery("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_timeline_event_type'")
	require.NoError(t, err)
	assert.Len(t, indexRows, 1)
}

func TestBackfillExtractedFields(t *testing.T) {
	s := newTestDB(t)
	instID, _ := s.UpsertInstance(store.InstanceRecord{Name: "gh", Type: "github", BaseURL: "https://api.github.com"})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo"})

	// Insert PR with raw_data containing merged_by, comments, review_comments
	rawData := `{"merged_by":{"login":"merger"},"comments":7,"review_comments":4}`
	prID, err := s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "PR with raw", State: "merged", Author: "alice",
		URL: "u", CreatedAt: "2026-03-01T10:00:00Z", UpdatedAt: "2026-03-05T14:00:00Z",
		RawData: &rawData,
	})
	require.NoError(t, err)

	// Insert timeline event with raw_data containing state for reviewed event
	ts := "2026-03-02T10:00:00Z"
	actor := "bob"
	reviewRaw := `{"event":"reviewed","state":"approved","user":{"login":"bob"}}`
	s.ReplaceTimelineEvents(prID, []store.TimelineEventRecord{
		{EventType: "reviewed", CreatedAt: &ts, Actor: &actor, RawData: reviewRaw},
	})

	// Run backfill
	updated, err := s.BackfillExtractedFields()
	require.NoError(t, err)
	assert.Greater(t, updated, 0)

	// Verify PR fields were populated
	prs, err := s.ListPullRequests(store.PRFilters{RepoIDs: []int64{repoID}})
	require.NoError(t, err)
	require.Len(t, prs, 1)
	require.NotNil(t, prs[0].MergedBy)
	assert.Equal(t, "merger", *prs[0].MergedBy)
	assert.Equal(t, 7, prs[0].CommentCount)
	assert.Equal(t, 4, prs[0].ReviewCommentCount)

	// Verify timeline review_state was populated
	rows, err := s.RawQuery("SELECT review_state FROM timeline_events WHERE pr_id = " + fmt.Sprintf("%d", prID))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "approved", rows[0]["review_state"])
}

func strPtr(s string) *string { return &s }

func TestMigrateV1ToV2(t *testing.T) {
	// Create a V1-only database, then migrate to V2
	path := filepath.Join(t.TempDir(), "migrate.db")
	s := New(path)
	require.NoError(t, s.Open())

	// Apply only V1
	_, err := s.db.Exec(schemaV1)
	require.NoError(t, err)
	_, err = s.db.Exec("INSERT INTO schema_version (version, applied_at) VALUES (1, '2026-01-01T00:00:00Z')")
	require.NoError(t, err)

	// Insert a V1 instance (without token_env/tls_skip_verify columns)
	_, err = s.db.Exec("INSERT INTO instances (name, type, base_url) VALUES ('old', 'github', 'https://api.github.com')")
	require.NoError(t, err)

	// Now migrate — should add new columns
	require.NoError(t, s.Migrate())

	// Verify old instance got defaults for new columns
	inst, err := s.GetInstanceByName("old")
	require.NoError(t, err)
	assert.Equal(t, "", inst.TokenEnv)
	assert.False(t, inst.TLSSkipVerify)

	// Verify settings table works
	require.NoError(t, s.SetSetting("test.key", "test.value"))
	val, _ := s.GetSetting("test.key")
	assert.Equal(t, "test.value", val)

	s.Close()
}

func TestResetAll(t *testing.T) {
	s := newTestDB(t)

	// Populate data
	instID, _ := s.UpsertInstance(store.InstanceRecord{Name: "gh", Type: "github", BaseURL: "https://api.github.com", TokenEnv: "GH_TOKEN"})
	repoID, _ := s.UpsertRepository(store.RepositoryRecord{InstanceID: instID, Owner: "org", Name: "repo", FullName: "org/repo"})
	teamID, _ := s.UpsertTeam(store.TeamRecord{Name: "team1"})
	s.AddTeamRepo(teamID, repoID)
	s.SetSetting("foo", "bar")
	s.UpsertPullRequest(store.PullRequestRecord{
		RepoID: repoID, Number: 1, Title: "PR", State: "merged",
		Author: "alice", URL: "https://example.com", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	})

	// Verify data exists
	stats, _ := s.Stats()
	totalBefore := int64(0)
	for _, count := range stats.Tables {
		totalBefore += count
	}
	assert.Greater(t, totalBefore, int64(0))

	// Reset
	require.NoError(t, s.ResetAll())

	// Verify all tables are empty
	stats, _ = s.Stats()
	for table, count := range stats.Tables {
		assert.Equal(t, int64(0), count, "table %s should be empty after reset", table)
	}

	// Verify DB is still functional (can insert again)
	_, err := s.UpsertInstance(store.InstanceRecord{Name: "new", Type: "github", BaseURL: "https://api.github.com", TokenEnv: "TOKEN"})
	assert.NoError(t, err)
}
