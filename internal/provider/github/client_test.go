package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Arsenalist/prx/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPullRequestsUntilFiltering(t *testing.T) {
	// PRs sorted by updated_at desc (as GitHub returns them)
	prs := []ghPullRequest{
		{Number: 3, Title: "Too new", State: "closed", UpdatedAt: "2026-04-15T10:00:00Z", CreatedAt: "2026-04-10T10:00:00Z", User: struct{ Login string `json:"login"` }{Login: "alice"}},
		{Number: 2, Title: "In range", State: "closed", UpdatedAt: "2026-03-20T10:00:00Z", CreatedAt: "2026-03-15T10:00:00Z", User: struct{ Login string `json:"login"` }{Login: "bob"}},
		{Number: 1, Title: "Also in range", State: "closed", UpdatedAt: "2026-03-10T10:00:00Z", CreatedAt: "2026-03-01T10:00:00Z", User: struct{ Login string `json:"login"` }{Login: "alice"}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prs)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", false, 1)
	result, err := client.ListPullRequests("org", "repo", provider.ListPROptions{
		State:   "all",
		PerPage: 100,
		Until:   "2026-04-01T00:00:00Z",
	})

	require.NoError(t, err)
	// PR #3 (updated 2026-04-15) should be excluded, PRs #2 and #1 included
	assert.Len(t, result, 2)
	assert.Equal(t, 2, result[0].Number)
	assert.Equal(t, 1, result[1].Number)
}

func TestListPullRequestsSinceAndUntilFiltering(t *testing.T) {
	prs := []ghPullRequest{
		{Number: 4, Title: "Too new", State: "closed", UpdatedAt: "2026-04-15T10:00:00Z", CreatedAt: "2026-04-10T10:00:00Z", User: struct{ Login string `json:"login"` }{Login: "alice"}},
		{Number: 3, Title: "In range", State: "closed", UpdatedAt: "2026-03-20T10:00:00Z", CreatedAt: "2026-03-15T10:00:00Z", User: struct{ Login string `json:"login"` }{Login: "bob"}},
		{Number: 2, Title: "Too old", State: "closed", UpdatedAt: "2026-02-15T10:00:00Z", CreatedAt: "2026-02-10T10:00:00Z", User: struct{ Login string `json:"login"` }{Login: "alice"}},
		{Number: 1, Title: "Way too old", State: "closed", UpdatedAt: "2026-01-10T10:00:00Z", CreatedAt: "2026-01-05T10:00:00Z", User: struct{ Login string `json:"login"` }{Login: "alice"}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prs)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", false, 1)
	result, err := client.ListPullRequests("org", "repo", provider.ListPROptions{
		State:   "all",
		PerPage: 100,
		Since:   "2026-03-01T00:00:00Z",
		Until:   "2026-04-01T00:00:00Z",
	})

	require.NoError(t, err)
	// Only PR #3 should be in range (updated between March 1 and April 1)
	assert.Len(t, result, 1)
	assert.Equal(t, 3, result[0].Number)
}

func TestListPullRequestsNoFilters(t *testing.T) {
	prs := []ghPullRequest{
		{Number: 2, Title: "PR 2", State: "open", UpdatedAt: "2026-03-20T10:00:00Z", CreatedAt: "2026-03-15T10:00:00Z", User: struct{ Login string `json:"login"` }{Login: "alice"}},
		{Number: 1, Title: "PR 1", State: "closed", UpdatedAt: "2026-03-10T10:00:00Z", CreatedAt: "2026-03-01T10:00:00Z", User: struct{ Login string `json:"login"` }{Login: "bob"}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prs)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", false, 1)
	result, err := client.ListPullRequests("org", "repo", provider.ListPROptions{
		State:   "all",
		PerPage: 100,
	})

	require.NoError(t, err)
	// All PRs should be returned when no filters
	assert.Len(t, result, 2)
}
