// Package store defines the storage provider abstraction.
package store

import "time"

// Store is the interface all storage providers must implement.
type Store interface {
	// Lifecycle
	Open() error
	Close() error
	Migrate() error

	// Instance/Repo management
	UpsertInstance(inst InstanceRecord) (int64, error)
	UpsertRepository(repo RepositoryRecord) (int64, error)

	// Instance management
	ListInstances() ([]InstanceRecord, error)
	DeleteInstance(name string) error

	// Repository management
	DeleteRepository(instanceID int64, fullName string) error

	// Team management
	UpsertTeam(team TeamRecord) (int64, error)
	SetTeamRepos(teamID int64, repoIDs []int64) error
	ListTeams() ([]TeamRecord, error)
	GetTeamByName(name string) (*TeamRecord, error)
	DeleteTeam(name string) error
	GetTeamRepos(teamID int64) ([]RepositoryRecord, error)
	AddTeamRepo(teamID int64, repoID int64) error
	RemoveTeamRepo(teamID int64, repoID int64) error

	// Settings
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
	DeleteSetting(key string) error
	ListSettings() (map[string]string, error)

	// Pull request operations
	UpsertPullRequest(pr PullRequestRecord) (int64, error)
	UpsertBranchInfo(info BranchInfoRecord) error
	ReplaceFileChanges(prID int64, files []FileChangeRecord) error
	ReplaceTimelineEvents(prID int64, events []TimelineEventRecord) error

	// Read operations
	GetPRState(repoID int64, number int) (*PRStateResult, error)
	ListPullRequests(filters PRFilters) ([]PullRequestRecord, error)
	GetFetchMetadata(repoID int64) (*FetchMetadataRecord, error)
	UpdateFetchMetadata(meta FetchMetadataRecord) error
	ListRepositories() ([]RepositoryRecord, error)
	GetRepositoryByName(instanceID int64, fullName string) (*RepositoryRecord, error)
	FindRepositoriesByFullName(fullName string) ([]RepositoryRecord, error)
	GetInstanceByName(name string) (*InstanceRecord, error)

	// Power user / agent
	RawQuery(sql string) ([]map[string]interface{}, error)
	Stats() (*StoreStats, error)
	ResetAll() error
}

// InstanceRecord represents a row in the instances table.
type InstanceRecord struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	BaseURL       string `json:"base_url"`
	TokenEnv      string `json:"token_env"`
	TLSSkipVerify bool   `json:"tls_skip_verify"`
}

// RepositoryRecord represents a row in the repositories table.
type RepositoryRecord struct {
	ID         int64  `json:"id"`
	InstanceID int64  `json:"instance_id"`
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	FullName   string `json:"full_name"`
}

// TeamRecord represents a row in the teams table.
type TeamRecord struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// PullRequestRecord represents a row in the pull_requests table.
type PullRequestRecord struct {
	ID                 int64   `json:"id"`
	RepoID             int64   `json:"repo_id"`
	Number             int     `json:"number"`
	Title              string  `json:"title"`
	State              string  `json:"state"`
	Author             string  `json:"author"`
	URL                string  `json:"url"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	MergedAt           *string `json:"merged_at"`
	ClosedAt           *string `json:"closed_at"`
	IsDraft            bool    `json:"is_draft"`
	ReadyForReviewAt   *string `json:"ready_for_review_at"`
	Additions          int     `json:"additions"`
	Deletions          int     `json:"deletions"`
	ChangedFiles       int     `json:"changed_files"`
	BaseBranch         *string `json:"base_branch"`
	HeadBranch         *string `json:"head_branch"`
	Body               *string `json:"body"`
	RawData            *string `json:"raw_data"`
	MergedBy           *string `json:"merged_by"`
	CommentCount       int     `json:"comment_count"`
	ReviewCommentCount int     `json:"review_comment_count"`
}

// BranchInfoRecord represents a row in the branch_info table.
type BranchInfoRecord struct {
	PRID            int64   `json:"pr_id"`
	MergeBaseSHA    *string `json:"merge_base_sha"`
	FirstCommitDate *string `json:"first_commit_date"`
	CommitsCount    int     `json:"commits_count"`
	TotalAdditions  int     `json:"total_additions"`
	TotalDeletions  int     `json:"total_deletions"`
	RawData         *string `json:"raw_data"`
}

// FileChangeRecord represents a row in the file_changes table.
type FileChangeRecord struct {
	ID        int64  `json:"id"`
	PRID      int64  `json:"pr_id"`
	Filename  string `json:"filename"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	IsTest    bool   `json:"is_test"`
}

// TimelineEventRecord represents a row in the timeline_events table.
type TimelineEventRecord struct {
	ID          int64   `json:"id"`
	PRID        int64   `json:"pr_id"`
	EventType   string  `json:"event_type"`
	CreatedAt   *string `json:"created_at"`
	Actor       *string `json:"actor"`
	RawData     string  `json:"raw_data"`
	ReviewState *string `json:"review_state"`
}

// FetchMetadataRecord represents a row in the fetch_metadata table.
type FetchMetadataRecord struct {
	RepoID        int64  `json:"repo_id"`
	LastFetchAt   string `json:"last_fetch_at"`
	LastUpdatedAt string `json:"last_updated_at"`
	PRCount       int    `json:"pr_count"`
}

// PRStateResult is a lightweight result for the smart sync decision matrix.
type PRStateResult struct {
	State     string
	UpdatedAt string
}

// PRFilters controls which PRs to return from ListPullRequests.
type PRFilters struct {
	RepoIDs   []int64
	Authors   []string
	States    []string
	StartDate string // YYYY-MM-DD, filters created_at >=
	EndDate   string // YYYY-MM-DD, filters created_at <=
}

// StoreStats contains summary stats about the database.
type StoreStats struct {
	DatabasePath string
	SizeBytes    int64
	Tables       map[string]int64 // table name -> row count
	FetchedAt    *time.Time
}
