// Package sqlite implements the storage provider interface using SQLite.
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Arsenalist/prx/internal/store"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements store.Store backed by SQLite.
type SQLiteStore struct {
	path string
	db   *sql.DB
}

// New creates a new SQLiteStore. Call Open() to connect.
func New(path string) *SQLiteStore {
	return &SQLiteStore{path: path}
}

func (s *SQLiteStore) Open() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite", s.path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	s.db = db
	return nil
}

func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *SQLiteStore) Migrate() error {
	// Check current version
	var version int
	row := s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&version); err != nil {
		// Table doesn't exist yet, that's fine
		version = 0
	}

	if version < 1 {
		if _, err := s.db.Exec(schemaV1); err != nil {
			return fmt.Errorf("applying schema v1: %w", err)
		}
		_, err := s.db.Exec("INSERT INTO schema_version (version, applied_at) VALUES (1, ?)", time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("recording schema version: %w", err)
		}
	}

	return nil
}

// --- Instance operations ---

func (s *SQLiteStore) UpsertInstance(inst store.InstanceRecord) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO instances (name, type, base_url) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET type=excluded.type, base_url=excluded.base_url
	`, inst.Name, inst.Type, inst.BaseURL)
	if err != nil {
		return 0, err
	}

	// ON CONFLICT doesn't set last_insert_rowid, so query it
	var id int64
	err = s.db.QueryRow("SELECT id FROM instances WHERE name = ?", inst.Name).Scan(&id)
	if err != nil {
		return 0, err
	}
	_ = res
	return id, nil
}

func (s *SQLiteStore) GetInstanceByName(name string) (*store.InstanceRecord, error) {
	var inst store.InstanceRecord
	err := s.db.QueryRow("SELECT id, name, type, base_url FROM instances WHERE name = ?", name).
		Scan(&inst.ID, &inst.Name, &inst.Type, &inst.BaseURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inst, nil
}

// --- Repository operations ---

func (s *SQLiteStore) UpsertRepository(repo store.RepositoryRecord) (int64, error) {
	_, err := s.db.Exec(`
		INSERT INTO repositories (instance_id, owner, name, full_name) VALUES (?, ?, ?, ?)
		ON CONFLICT(instance_id, full_name) DO UPDATE SET owner=excluded.owner, name=excluded.name
	`, repo.InstanceID, repo.Owner, repo.Name, repo.FullName)
	if err != nil {
		return 0, err
	}

	var id int64
	err = s.db.QueryRow("SELECT id FROM repositories WHERE instance_id = ? AND full_name = ?",
		repo.InstanceID, repo.FullName).Scan(&id)
	return id, err
}

func (s *SQLiteStore) ListRepositories() ([]store.RepositoryRecord, error) {
	rows, err := s.db.Query("SELECT id, instance_id, owner, name, full_name FROM repositories ORDER BY full_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []store.RepositoryRecord
	for rows.Next() {
		var r store.RepositoryRecord
		if err := rows.Scan(&r.ID, &r.InstanceID, &r.Owner, &r.Name, &r.FullName); err != nil {
			return nil, err
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

func (s *SQLiteStore) GetRepositoryByName(instanceID int64, fullName string) (*store.RepositoryRecord, error) {
	var r store.RepositoryRecord
	err := s.db.QueryRow("SELECT id, instance_id, owner, name, full_name FROM repositories WHERE instance_id = ? AND full_name = ?",
		instanceID, fullName).Scan(&r.ID, &r.InstanceID, &r.Owner, &r.Name, &r.FullName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &r, err
}

// --- Team operations ---

func (s *SQLiteStore) UpsertTeam(team store.TeamRecord) (int64, error) {
	_, err := s.db.Exec(`
		INSERT INTO teams (name, display_name) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET display_name=excluded.display_name
	`, team.Name, team.DisplayName)
	if err != nil {
		return 0, err
	}

	var id int64
	err = s.db.QueryRow("SELECT id FROM teams WHERE name = ?", team.Name).Scan(&id)
	return id, err
}

func (s *SQLiteStore) SetTeamRepos(teamID int64, repoIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM team_repos WHERE team_id = ?", teamID); err != nil {
		return err
	}
	for _, repoID := range repoIDs {
		if _, err := tx.Exec("INSERT INTO team_repos (team_id, repo_id) VALUES (?, ?)", teamID, repoID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Pull request operations ---

func (s *SQLiteStore) UpsertPullRequest(pr store.PullRequestRecord) (int64, error) {
	isDraft := 0
	if pr.IsDraft {
		isDraft = 1
	}

	_, err := s.db.Exec(`
		INSERT INTO pull_requests (repo_id, number, title, state, author, url, created_at, updated_at,
			merged_at, closed_at, is_draft, ready_for_review_at, additions, deletions, changed_files,
			base_branch, head_branch, body, raw_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo_id, number) DO UPDATE SET
			title=excluded.title, state=excluded.state, author=excluded.author, url=excluded.url,
			created_at=excluded.created_at, updated_at=excluded.updated_at, merged_at=excluded.merged_at,
			closed_at=excluded.closed_at, is_draft=excluded.is_draft, ready_for_review_at=excluded.ready_for_review_at,
			additions=excluded.additions, deletions=excluded.deletions, changed_files=excluded.changed_files,
			base_branch=excluded.base_branch, head_branch=excluded.head_branch, body=excluded.body, raw_data=excluded.raw_data
	`, pr.RepoID, pr.Number, pr.Title, pr.State, pr.Author, pr.URL, pr.CreatedAt, pr.UpdatedAt,
		pr.MergedAt, pr.ClosedAt, isDraft, pr.ReadyForReviewAt, pr.Additions, pr.Deletions, pr.ChangedFiles,
		pr.BaseBranch, pr.HeadBranch, pr.Body, pr.RawData)
	if err != nil {
		return 0, err
	}

	var id int64
	err = s.db.QueryRow("SELECT id FROM pull_requests WHERE repo_id = ? AND number = ?",
		pr.RepoID, pr.Number).Scan(&id)
	return id, err
}

func (s *SQLiteStore) GetPRState(repoID int64, number int) (*store.PRStateResult, error) {
	var result store.PRStateResult
	err := s.db.QueryRow("SELECT state, updated_at FROM pull_requests WHERE repo_id = ? AND number = ?",
		repoID, number).Scan(&result.State, &result.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *SQLiteStore) ListPullRequests(filters store.PRFilters) ([]store.PullRequestRecord, error) {
	query := `SELECT id, repo_id, number, title, state, author, url, created_at, updated_at,
		merged_at, closed_at, is_draft, ready_for_review_at, additions, deletions, changed_files,
		base_branch, head_branch, body, raw_data FROM pull_requests WHERE 1=1`

	var args []interface{}

	if len(filters.RepoIDs) > 0 {
		placeholders := make([]string, len(filters.RepoIDs))
		for i, id := range filters.RepoIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		query += " AND repo_id IN (" + strings.Join(placeholders, ",") + ")"
	}

	if len(filters.Authors) > 0 {
		placeholders := make([]string, len(filters.Authors))
		for i, a := range filters.Authors {
			placeholders[i] = "?"
			args = append(args, a)
		}
		query += " AND author IN (" + strings.Join(placeholders, ",") + ")"
	}

	if len(filters.States) > 0 {
		placeholders := make([]string, len(filters.States))
		for i, st := range filters.States {
			placeholders[i] = "?"
			args = append(args, st)
		}
		query += " AND state IN (" + strings.Join(placeholders, ",") + ")"
	}

	if filters.StartDate != "" {
		query += " AND created_at >= ?"
		args = append(args, filters.StartDate)
	}
	if filters.EndDate != "" {
		query += " AND created_at <= ?"
		args = append(args, filters.EndDate+"T23:59:59Z")
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []store.PullRequestRecord
	for rows.Next() {
		var pr store.PullRequestRecord
		var isDraft int
		if err := rows.Scan(&pr.ID, &pr.RepoID, &pr.Number, &pr.Title, &pr.State, &pr.Author,
			&pr.URL, &pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt, &pr.ClosedAt, &isDraft,
			&pr.ReadyForReviewAt, &pr.Additions, &pr.Deletions, &pr.ChangedFiles,
			&pr.BaseBranch, &pr.HeadBranch, &pr.Body, &pr.RawData); err != nil {
			return nil, err
		}
		pr.IsDraft = isDraft == 1
		prs = append(prs, pr)
	}
	return prs, rows.Err()
}

// --- Branch info ---

func (s *SQLiteStore) UpsertBranchInfo(info store.BranchInfoRecord) error {
	_, err := s.db.Exec(`
		INSERT INTO branch_info (pr_id, merge_base_sha, first_commit_date, commits_count, total_additions, total_deletions, raw_data)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(pr_id) DO UPDATE SET
			merge_base_sha=excluded.merge_base_sha, first_commit_date=excluded.first_commit_date,
			commits_count=excluded.commits_count, total_additions=excluded.total_additions,
			total_deletions=excluded.total_deletions, raw_data=excluded.raw_data
	`, info.PRID, info.MergeBaseSHA, info.FirstCommitDate, info.CommitsCount, info.TotalAdditions, info.TotalDeletions, info.RawData)
	return err
}

// --- File changes ---

func (s *SQLiteStore) ReplaceFileChanges(prID int64, files []store.FileChangeRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM file_changes WHERE pr_id = ?", prID); err != nil {
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO file_changes (pr_id, filename, additions, deletions, is_test) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range files {
		isTest := 0
		if f.IsTest {
			isTest = 1
		}
		if _, err := stmt.Exec(prID, f.Filename, f.Additions, f.Deletions, isTest); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Timeline events ---

func (s *SQLiteStore) ReplaceTimelineEvents(prID int64, events []store.TimelineEventRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM timeline_events WHERE pr_id = ?", prID); err != nil {
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO timeline_events (pr_id, event_type, created_at, actor, raw_data) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		if _, err := stmt.Exec(prID, e.EventType, e.CreatedAt, e.Actor, e.RawData); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Fetch metadata ---

func (s *SQLiteStore) GetFetchMetadata(repoID int64) (*store.FetchMetadataRecord, error) {
	var meta store.FetchMetadataRecord
	err := s.db.QueryRow("SELECT repo_id, last_fetch_at, last_updated_at, pr_count FROM fetch_metadata WHERE repo_id = ?",
		repoID).Scan(&meta.RepoID, &meta.LastFetchAt, &meta.LastUpdatedAt, &meta.PRCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *SQLiteStore) UpdateFetchMetadata(meta store.FetchMetadataRecord) error {
	_, err := s.db.Exec(`
		INSERT INTO fetch_metadata (repo_id, last_fetch_at, last_updated_at, pr_count)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(repo_id) DO UPDATE SET
			last_fetch_at=excluded.last_fetch_at, last_updated_at=excluded.last_updated_at, pr_count=excluded.pr_count
	`, meta.RepoID, meta.LastFetchAt, meta.LastUpdatedAt, meta.PRCount)
	return err
}

// --- Power user / agent ---

func (s *SQLiteStore) RawQuery(query string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			row[col] = values[i]
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) Stats() (*store.StoreStats, error) {
	stats := &store.StoreStats{
		DatabasePath: s.path,
		Tables:       make(map[string]int64),
	}

	// File size
	info, err := os.Stat(s.path)
	if err == nil {
		stats.SizeBytes = info.Size()
	}

	// Row counts for each table
	tables := []string{"instances", "repositories", "teams", "pull_requests", "branch_info", "file_changes", "timeline_events", "fetch_metadata"}
	for _, table := range tables {
		var count int64
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			return nil, err
		}
		stats.Tables[table] = count
	}

	return stats, nil
}
