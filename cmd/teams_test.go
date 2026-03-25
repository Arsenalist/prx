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
		names, err := resolveTeamReposFromDB(db, []string{"platform"})
		require.NoError(t, err)
		assert.Len(t, names, 2)
		assert.Contains(t, names, "org/api")
		assert.Contains(t, names, "org/web")
	})

	t.Run("unknown team", func(t *testing.T) {
		names, err := resolveTeamReposFromDB(db, []string{"nonexistent"})
		require.NoError(t, err)
		assert.Len(t, names, 0)
	})
}

func TestTeamNameFromFlags(t *testing.T) {
	assert.Equal(t, "", teamNameFromFlags(nil))
	assert.Equal(t, "platform", teamNameFromFlags([]string{"platform"}))
	assert.Equal(t, "platform+data", teamNameFromFlags([]string{"platform", "data"}))
}
