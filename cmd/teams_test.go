package cmd

import (
	"path/filepath"
	"testing"

	"github.com/Arsenalist/prx/internal/store"
	"github.com/Arsenalist/prx/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db := sqlite.New(path)
	require.NoError(t, db.Open())
	require.NoError(t, db.Migrate())
	t.Cleanup(func() { db.Close() })
	return db
}

func TestResolveTeamReposFromDB(t *testing.T) {
	db := setupTestDB(t)

	// Setup: instance + repos + team
	instID, err := db.UpsertInstance(store.InstanceRecord{Name: "github", Type: "github", BaseURL: "https://api.github.com", TokenEnv: "GH_TOKEN"})
	require.NoError(t, err)

	repo1ID, _ := db.UpsertRepository(store.RepositoryRecord{InstanceID: instID, Owner: "org", Name: "api", FullName: "org/api"})
	repo2ID, _ := db.UpsertRepository(store.RepositoryRecord{InstanceID: instID, Owner: "org", Name: "web", FullName: "org/web"})

	teamID, _ := db.UpsertTeam(store.TeamRecord{Name: "platform", DisplayName: "Platform Team"})
	require.NoError(t, db.AddTeamRepo(teamID, repo1ID))
	require.NoError(t, db.AddTeamRepo(teamID, repo2ID))

	t.Run("single team", func(t *testing.T) {
		repos, err := resolveTeamReposFromDB(db, []string{"platform"})
		require.NoError(t, err)
		assert.Len(t, repos, 2)
		names := []string{repos[0].FullName, repos[1].FullName}
		assert.Contains(t, names, "org/api")
		assert.Contains(t, names, "org/web")
		// Verify IDs are populated
		for _, r := range repos {
			assert.NotZero(t, r.ID)
			assert.NotZero(t, r.InstanceID)
		}
	})

	t.Run("unknown team", func(t *testing.T) {
		repos, err := resolveTeamReposFromDB(db, []string{"nonexistent"})
		require.NoError(t, err)
		assert.Len(t, repos, 0)
	})
}

func TestResolveTeamReposFromDB_ReturnsIDs(t *testing.T) {
	db := setupTestDB(t)

	// Create two instances with a repo that has the SAME full_name
	inst1ID, err := db.UpsertInstance(store.InstanceRecord{Name: "github-cloud", Type: "github", BaseURL: "https://api.github.com"})
	require.NoError(t, err)
	inst2ID, err := db.UpsertInstance(store.InstanceRecord{Name: "github-enterprise", Type: "github", BaseURL: "https://github.corp.com/api/v3"})
	require.NoError(t, err)

	// Same full_name, different instances → different repo IDs
	repo1ID, _ := db.UpsertRepository(store.RepositoryRecord{InstanceID: inst1ID, Owner: "org", Name: "api", FullName: "org/api"})
	repo2ID, _ := db.UpsertRepository(store.RepositoryRecord{InstanceID: inst2ID, Owner: "org", Name: "api", FullName: "org/api"})
	require.NotEqual(t, repo1ID, repo2ID, "repos should have different IDs")

	// Team uses repo from instance 2
	teamID, _ := db.UpsertTeam(store.TeamRecord{Name: "enterprise-team"})
	require.NoError(t, db.AddTeamRepo(teamID, repo2ID))

	repos, err := resolveTeamReposFromDB(db, []string{"enterprise-team"})
	require.NoError(t, err)
	require.Len(t, repos, 1)
	// The resolved repo must have the correct ID from instance 2, not instance 1
	assert.Equal(t, repo2ID, repos[0].ID, "should return the exact repo ID from the team, not a name-matched one")
	assert.Equal(t, inst2ID, repos[0].InstanceID)
}

func TestResolveRepoIDs_WithDuplicateFullNames(t *testing.T) {
	db := setupTestDB(t)

	inst1ID, _ := db.UpsertInstance(store.InstanceRecord{Name: "cloud", Type: "github", BaseURL: "https://api.github.com"})
	inst2ID, _ := db.UpsertInstance(store.InstanceRecord{Name: "enterprise", Type: "github", BaseURL: "https://github.corp.com/api/v3"})

	repo1ID, _ := db.UpsertRepository(store.RepositoryRecord{InstanceID: inst1ID, Owner: "org", Name: "api", FullName: "org/api"})
	repo2ID, _ := db.UpsertRepository(store.RepositoryRecord{InstanceID: inst2ID, Owner: "org", Name: "api", FullName: "org/api"})

	// When team provides repo IDs directly, resolveRepoIDs should use those IDs
	// instead of doing a name-based lookup that would be ambiguous
	filters := store.PRFilters{}
	teamRepoIDs := []int64{repo2ID}
	names, err := resolveRepoIDs(db, nil, &filters, teamRepoIDs)
	require.NoError(t, err)
	assert.Equal(t, []int64{repo2ID}, filters.RepoIDs, "should use exact team repo IDs")
	assert.Contains(t, names, "org/api")

	// When no team IDs and no repo flags, should return all repos
	filters2 := store.PRFilters{}
	names2, err := resolveRepoIDs(db, nil, &filters2, nil)
	require.NoError(t, err)
	assert.Len(t, filters2.RepoIDs, 2)
	assert.Contains(t, filters2.RepoIDs, repo1ID)
	assert.Contains(t, filters2.RepoIDs, repo2ID)
	assert.Len(t, names2, 2)
}

func TestTeamNameFromFlags(t *testing.T) {
	assert.Equal(t, "", teamNameFromFlags(nil))
	assert.Equal(t, "platform", teamNameFromFlags([]string{"platform"}))
	assert.Equal(t, "platform+data", teamNameFromFlags([]string{"platform", "data"}))
}
