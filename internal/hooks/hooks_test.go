package hooks

import (
	"testing"

	"github.com/Arsenalist/prx/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHooks(t *testing.T) {
	t.Run("echo hook receives stdin", func(t *testing.T) {
		hooks := []config.Hook{
			{Name: "test-echo", Command: "cat", Timeout: 5},
		}
		results := Run("post-analyze", hooks, []byte(`{"total_prs": 42}`), false)
		require.Len(t, results, 1)
		assert.Equal(t, "test-echo", results[0].Name)
		assert.NoError(t, results[0].Err)
		assert.Contains(t, results[0].Output, "42")
	})

	t.Run("failing hook captures error", func(t *testing.T) {
		hooks := []config.Hook{
			{Name: "bad", Command: "false", Timeout: 5},
		}
		results := Run("post-analyze", hooks, nil, false)
		require.Len(t, results, 1)
		assert.Error(t, results[0].Err)
	})

	t.Run("no hooks returns empty", func(t *testing.T) {
		results := Run("post-analyze", nil, nil, false)
		assert.Len(t, results, 0)
	})

	t.Run("hook with env vars", func(t *testing.T) {
		hooks := []config.Hook{
			{
				Name:    "env-test",
				Command: "echo $PRX_EVENT",
				Timeout: 5,
				Env:     map[string]string{"CUSTOM": "val"},
			},
		}
		results := Run("post-analyze", hooks, nil, false)
		require.Len(t, results, 1)
		assert.NoError(t, results[0].Err)
		assert.Contains(t, results[0].Output, "post-analyze")
	})
}
