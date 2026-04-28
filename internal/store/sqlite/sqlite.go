// Package sqlite implements the storage provider interface using SQLite.
package sqlite

import (
	"database/sql"
	"encoding/json"
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
		version = 1
	}

	if version < 2 {
		// V2 adds columns to instances and creates settings table.
		// Execute ALTER TABLE statements individually (SQLite doesn't support multiple in one Exec).
		for _, stmt := range []string{
			"ALTER TABLE instances ADD COLUMN token_env TEXT NOT NULL DEFAULT ''",
			"ALTER TABLE instances ADD COLUMN tls_skip_verify INTEGER NOT NULL DEFAULT 0",
			`CREATE TABLE IF NOT EXISTS settings (
				key   TEXT PRIMARY KEY,
				value TEXT NOT NULL
			)`,
		} {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("applying schema v2: %w", err)
			}
		}
		_, err := s.db.Exec("INSERT INTO schema_version (version, applied_at) VALUES (2, ?)", time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("recording schema version: %w", err)
		}
		version = 2
	}

	if version < 3 {
		for _, stmt := range []string{
			"ALTER TABLE pull_requests ADD COLUMN merged_by TEXT",
			"ALTER TABLE pull_requests ADD COLUMN comment_count INTEGER NOT NULL DEFAULT 0",
			"ALTER TABLE pull_requests ADD COLUMN review_comment_count INTEGER NOT NULL DEFAULT 0",
			"ALTER TABLE timeline_events ADD COLUMN review_state TEXT",
			"CREATE INDEX IF NOT EXISTS idx_timeline_event_type ON timeline_events(pr_id, event_type)",
		} {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("applying schema v3: %w", err)
			}
		}
		_, err := s.db.Exec("INSERT INTO schema_version (version, applied_at) VALUES (3, ?)", time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("recording schema version: %w", err)
		}
	}

	return nil
}

// --- Instance operations ---

func (s *SQLiteStore) UpsertInstance(inst store.InstanceRecord) (int64, error) {
	tlsSkip := 0
	if inst.TLSSkipVerify {
		tlsSkip = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO instances (name, type, base_url, token_env, tls_skip_verify) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET type=excluded.type, base_url=excluded.base_url,
			token_env=excluded.token_env, tls_skip_verify=excluded.tls_skip_verify
	`, inst.Name, inst.Type, inst.BaseURL, inst.TokenEnv, tlsSkip)
	if err != nil {
		return 0, err
	}

	var id int64
	err = s.db.QueryRow("SELECT id FROM instances WHERE name = ?", inst.Name).Scan(&id)
	return id, err
}

func (s *SQLiteStore) GetInstanceByName(name string) (*store.InstanceRecord, error) {
	var inst store.InstanceRecord
	var tlsSkip int
	err := s.db.QueryRow("SELECT id, name, type, base_url, token_env, tls_skip_verify FROM instances WHERE name = ?", name).
		Scan(&inst.ID, &inst.Name, &inst.Type, &inst.BaseURL, &inst.TokenEnv, &tlsSkip)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	inst.TLSSkipVerify = tlsSkip == 1
	return &inst, nil
}

func (s *SQLiteStore) ListInstances() ([]store.InstanceRecord, error) {
	rows, err := s.db.Query("SELECT id, name, type, base_url, token_env, tls_skip_verify FROM instances ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []store.InstanceRecord
	for rows.Next() {
		var inst store.InstanceRecord
		var tlsSkip int
		if err := rows.Scan(&inst.ID, &inst.Name, &inst.Type, &inst.BaseURL, &inst.TokenEnv, &tlsSkip); err != nil {
			return nil, err
		}
		inst.TLSSkipVerify = tlsSkip == 1
		instances = append(instances, inst)
	}
	return instances, rows.Err()
}

func (s *SQLiteStore) DeleteInstance(name string) error {
	result, err := s.db.Exec("DELETE FROM instances WHERE name = ?", name)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("instance %q not found", name)
	}
	return nil
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

func (s *SQLiteStore) DeleteRepository(instanceID int64, fullName string) error {
	result, err := s.db.Exec("DELETE FROM repositories WHERE instance_id = ? AND full_name = ?", instanceID, fullName)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("repository %q not found", fullName)
	}
	return nil
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

func (s *SQLiteStore) ListTeams() ([]store.TeamRecord, error) {
	rows, err := s.db.Query("SELECT id, name, display_name FROM teams ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []store.TeamRecord
	for rows.Next() {
		var t store.TeamRecord
		if err := rows.Scan(&t.ID, &t.Name, &t.DisplayName); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (s *SQLiteStore) GetTeamByName(name string) (*store.TeamRecord, error) {
	var t store.TeamRecord
	err := s.db.QueryRow("SELECT id, name, display_name FROM teams WHERE name = ?", name).
		Scan(&t.ID, &t.Name, &t.DisplayName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *SQLiteStore) DeleteTeam(name string) error {
	// Get team ID first to clean up team_repos
	var teamID int64
	err := s.db.QueryRow("SELECT id FROM teams WHERE name = ?", name).Scan(&teamID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("team %q not found", name)
	}
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM team_repos WHERE team_id = ?", teamID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM teams WHERE id = ?", teamID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetTeamRepos(teamID int64) ([]store.RepositoryRecord, error) {
	rows, err := s.db.Query(`
		SELECT r.id, r.instance_id, r.owner, r.name, r.full_name
		FROM repositories r
		JOIN team_repos tr ON tr.repo_id = r.id
		WHERE tr.team_id = ?
		ORDER BY r.full_name
	`, teamID)
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

func (s *SQLiteStore) AddTeamRepo(teamID int64, repoID int64) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO team_repos (team_id, repo_id) VALUES (?, ?)", teamID, repoID)
	return err
}

func (s *SQLiteStore) RemoveTeamRepo(teamID int64, repoID int64) error {
	result, err := s.db.Exec("DELETE FROM team_repos WHERE team_id = ? AND repo_id = ?", teamID, repoID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("repo not in team")
	}
	return nil
}

// --- Settings operations ---

func (s *SQLiteStore) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *SQLiteStore) SetSetting(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, key, value)
	return err
}

func (s *SQLiteStore) DeleteSetting(key string) error {
	_, err := s.db.Exec("DELETE FROM settings WHERE key = ?", key)
	return err
}

func (s *SQLiteStore) ListSettings() (map[string]string, error) {
	rows, err := s.db.Query("SELECT key, value FROM settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, rows.Err()
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
			base_branch, head_branch, body, raw_data, merged_by, comment_count, review_comment_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo_id, number) DO UPDATE SET
			title=excluded.title, state=excluded.state, author=excluded.author, url=excluded.url,
			created_at=excluded.created_at, updated_at=excluded.updated_at, merged_at=excluded.merged_at,
			closed_at=excluded.closed_at, is_draft=excluded.is_draft, ready_for_review_at=excluded.ready_for_review_at,
			additions=excluded.additions, deletions=excluded.deletions, changed_files=excluded.changed_files,
			base_branch=excluded.base_branch, head_branch=excluded.head_branch, body=excluded.body, raw_data=excluded.raw_data,
			merged_by=excluded.merged_by, comment_count=excluded.comment_count, review_comment_count=excluded.review_comment_count
	`, pr.RepoID, pr.Number, pr.Title, pr.State, pr.Author, pr.URL, pr.CreatedAt, pr.UpdatedAt,
		pr.MergedAt, pr.ClosedAt, isDraft, pr.ReadyForReviewAt, pr.Additions, pr.Deletions, pr.ChangedFiles,
		pr.BaseBranch, pr.HeadBranch, pr.Body, pr.RawData, pr.MergedBy, pr.CommentCount, pr.ReviewCommentCount)
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
		base_branch, head_branch, body, raw_data, merged_by, comment_count, review_comment_count FROM pull_requests WHERE 1=1`

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

	if filters.StartDate != "" && filters.EndDate != "" {
		endDateFull := filters.EndDate + "T23:59:59Z"
		query += ` AND (
			(created_at >= ? AND created_at <= ?)
			OR (updated_at >= ? AND updated_at <= ?)
			OR (merged_at >= ? AND merged_at <= ?)
			OR (closed_at >= ? AND closed_at <= ?)
		)`
		args = append(args, filters.StartDate, endDateFull, filters.StartDate, endDateFull, filters.StartDate, endDateFull, filters.StartDate, endDateFull)
	} else if filters.StartDate != "" {
		query += ` AND (
			created_at >= ? OR updated_at >= ? OR merged_at >= ? OR closed_at >= ?
		)`
		args = append(args, filters.StartDate, filters.StartDate, filters.StartDate, filters.StartDate)
	} else if filters.EndDate != "" {
		endDateFull := filters.EndDate + "T23:59:59Z"
		query += " AND created_at <= ?"
		args = append(args, endDateFull)
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
			&pr.BaseBranch, &pr.HeadBranch, &pr.Body, &pr.RawData,
			&pr.MergedBy, &pr.CommentCount, &pr.ReviewCommentCount); err != nil {
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

	stmt, err := tx.Prepare("INSERT INTO timeline_events (pr_id, event_type, created_at, actor, raw_data, review_state) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		if _, err := stmt.Exec(prID, e.EventType, e.CreatedAt, e.Actor, e.RawData, e.ReviewState); err != nil {
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
	tables := []string{"instances", "repositories", "teams", "pull_requests", "branch_info", "file_changes", "timeline_events", "fetch_metadata", "settings"}
	for _, table := range tables {
		var count int64
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			return nil, err
		}
		stats.Tables[table] = count
	}

	return stats, nil
}

// BackfillExtractedFields parses raw_data JSON from existing rows to populate
// the new extracted columns (merged_by, comment_count, review_comment_count, review_state).
func (s *SQLiteStore) BackfillExtractedFields() (int, error) {
	updated := 0

	// Backfill pull_requests
	rows, err := s.db.Query("SELECT id, raw_data FROM pull_requests WHERE raw_data IS NOT NULL AND raw_data != ''")
	if err != nil {
		return 0, fmt.Errorf("querying pull_requests: %w", err)
	}
	defer rows.Close()

	type prUpdate struct {
		id               int64
		mergedBy         *string
		commentCount     int
		reviewCommentCount int
	}
	var prUpdates []prUpdate

	for rows.Next() {
		var id int64
		var rawData string
		if err := rows.Scan(&id, &rawData); err != nil {
			continue
		}
		var parsed struct {
			MergedBy       *struct{ Login string } `json:"merged_by"`
			Comments       int                     `json:"comments"`
			ReviewComments int                     `json:"review_comments"`
		}
		if err := json.Unmarshal([]byte(rawData), &parsed); err != nil {
			continue
		}
		u := prUpdate{id: id, commentCount: parsed.Comments, reviewCommentCount: parsed.ReviewComments}
		if parsed.MergedBy != nil {
			login := parsed.MergedBy.Login
			u.mergedBy = &login
		}
		prUpdates = append(prUpdates, u)
	}
	rows.Close()

	for _, u := range prUpdates {
		_, err := s.db.Exec("UPDATE pull_requests SET merged_by = ?, comment_count = ?, review_comment_count = ? WHERE id = ?",
			u.mergedBy, u.commentCount, u.reviewCommentCount, u.id)
		if err == nil {
			updated++
		}
	}

	// Backfill timeline_events review_state
	evtRows, err := s.db.Query("SELECT id, raw_data FROM timeline_events WHERE event_type = 'reviewed' AND (review_state IS NULL OR review_state = '')")
	if err != nil {
		return updated, fmt.Errorf("querying timeline_events: %w", err)
	}
	defer evtRows.Close()

	type evtUpdate struct {
		id    int64
		state string
	}
	var evtUpdates []evtUpdate

	for evtRows.Next() {
		var id int64
		var rawData string
		if err := evtRows.Scan(&id, &rawData); err != nil {
			continue
		}
		var parsed struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal([]byte(rawData), &parsed); err != nil {
			continue
		}
		if parsed.State != "" {
			evtUpdates = append(evtUpdates, evtUpdate{id: id, state: parsed.State})
		}
	}
	evtRows.Close()

	for _, u := range evtUpdates {
		_, err := s.db.Exec("UPDATE timeline_events SET review_state = ? WHERE id = ?", u.state, u.id)
		if err == nil {
			updated++
		}
	}

	return updated, nil
}

// ResetAll deletes all data from every table except schema_version.
func (s *SQLiteStore) ResetAll() error {
	tables := []string{
		"timeline_events", "file_changes", "branch_info", "fetch_metadata",
		"pull_requests", "team_repos", "repositories", "teams", "instances", "settings",
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, table := range tables {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			tx.Rollback()
			return fmt.Errorf("clearing %s: %w", table, err)
		}
	}
	return tx.Commit()
}
