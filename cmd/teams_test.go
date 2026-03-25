package cmd

import (
	"testing"

	"github.com/Arsenalist/prx/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestResolveTeamRepos(t *testing.T) {
	cfg := &config.Config{
		Teams: map[string]config.Team{
			"platform": {
				DisplayName: "Platform Team",
				Repos: []config.RepoRef{
					{Instance: "github", Repo: "org/api"},
					{Instance: "github", Repo: "org/web"},
				},
			},
			"data": {
				Repos: []config.RepoRef{
					{Instance: "enterprise", Repo: "corp/data-pipeline"},
				},
			},
		},
	}

	t.Run("single team", func(t *testing.T) {
		refs := resolveTeamRepos(cfg, []string{"platform"})
		assert.Len(t, refs, 2)
		assert.Equal(t, "org/api", refs[0].repo)
		assert.Equal(t, "org/web", refs[1].repo)
	})

	t.Run("multiple teams", func(t *testing.T) {
		refs := resolveTeamRepos(cfg, []string{"platform", "data"})
		assert.Len(t, refs, 3)
	})

	t.Run("unknown team", func(t *testing.T) {
		refs := resolveTeamRepos(cfg, []string{"nonexistent"})
		assert.Len(t, refs, 0)
	})
}

func TestCountInstances(t *testing.T) {
	team := config.Team{
		Repos: []config.RepoRef{
			{Instance: "github", Repo: "org/a"},
			{Instance: "github", Repo: "org/b"},
			{Instance: "enterprise", Repo: "corp/c"},
		},
	}
	assert.Equal(t, 2, countInstances(team))
}

func TestTeamNameFromFlags(t *testing.T) {
	assert.Equal(t, "", teamNameFromFlags(nil))
	assert.Equal(t, "platform", teamNameFromFlags([]string{"platform"}))
	assert.Equal(t, "platform+data", teamNameFromFlags([]string{"platform", "data"}))
}
