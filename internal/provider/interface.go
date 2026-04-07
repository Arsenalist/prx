// Package provider defines the VCS provider abstraction.
package provider

// VCSProvider is the interface for interacting with a version control hosting platform.
type VCSProvider interface {
	// ListPullRequests returns PRs for a repo, sorted by updated_at desc.
	ListPullRequests(owner, repo string, opts ListPROptions) ([]PullRequest, error)

	// GetPullRequest returns full details for a single PR.
	GetPullRequest(owner, repo string, number int) (*PullRequest, error)

	// GetBranchComparison compares two refs and returns commit/file info.
	GetBranchComparison(owner, repo, base, head string) (*BranchComparison, error)

	// GetTimelineEvents returns timeline events for a PR.
	GetTimelineEvents(owner, repo string, number int) ([]TimelineEvent, error)

	// GetRateLimit returns the current rate limit status.
	GetRateLimit() (*RateLimit, error)

	// Name returns the provider name (e.g., "github").
	Name() string
}

// ListPROptions controls PR listing behavior.
type ListPROptions struct {
	State   string // "open", "closed", "all"
	PerPage int
	Since   string // ISO 8601 — only PRs updated after this time
	Until   string // ISO 8601 — only PRs updated before this time
}

// PullRequest is the normalized PR representation across providers.
type PullRequest struct {
	Number             int
	Title              string
	State              string // "open", "closed", "merged"
	Author             string
	URL                string
	CreatedAt          string
	UpdatedAt          string
	MergedAt           *string
	ClosedAt           *string
	IsDraft            bool
	Additions          int
	Deletions          int
	ChangedFiles       int
	BaseBranch         string
	HeadBranch         string
	BaseSHA            string
	HeadSHA            string
	Body               string
	RawJSON            string // Full API response as JSON
	MergedBy           string
	CommentCount       int
	ReviewCommentCount int
}

// BranchComparison holds the result of comparing two branches.
type BranchComparison struct {
	MergeBaseSHA    string
	FirstCommitDate string
	CommitsCount    int
	TotalAdditions  int
	TotalDeletions  int
	Files           []FileChange
	RawJSON         string
}

// FileChange represents a single file's changes in a PR.
type FileChange struct {
	Filename  string
	Additions int
	Deletions int
}

// TimelineEvent is a normalized event from a PR's timeline.
type TimelineEvent struct {
	EventType   string
	CreatedAt   string
	Actor       string
	RawJSON     string
	ReviewState string // For "reviewed" events: approved, changes_requested, commented, dismissed
}

// RateLimit holds API rate limit information.
type RateLimit struct {
	Limit     int
	Remaining int
	ResetAt   string // ISO 8601
}
