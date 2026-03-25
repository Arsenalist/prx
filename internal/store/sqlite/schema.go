package sqlite

const schemaV2 = `
ALTER TABLE instances ADD COLUMN token_env TEXT NOT NULL DEFAULT '';
ALTER TABLE instances ADD COLUMN tls_skip_verify INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`

const schemaV1 = `
CREATE TABLE IF NOT EXISTS instances (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    type        TEXT NOT NULL DEFAULT 'github',
    base_url    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS repositories (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL REFERENCES instances(id),
    owner       TEXT NOT NULL,
    name        TEXT NOT NULL,
    full_name   TEXT NOT NULL,
    UNIQUE(instance_id, full_name)
);

CREATE TABLE IF NOT EXISTS teams (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL UNIQUE,
    display_name TEXT
);

CREATE TABLE IF NOT EXISTS team_repos (
    team_id     INTEGER NOT NULL REFERENCES teams(id),
    repo_id     INTEGER NOT NULL REFERENCES repositories(id),
    PRIMARY KEY (team_id, repo_id)
);

CREATE TABLE IF NOT EXISTS pull_requests (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id             INTEGER NOT NULL REFERENCES repositories(id),
    number              INTEGER NOT NULL,
    title               TEXT NOT NULL,
    state               TEXT NOT NULL,
    author              TEXT NOT NULL,
    url                 TEXT NOT NULL,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    merged_at           TEXT,
    closed_at           TEXT,
    is_draft            INTEGER NOT NULL DEFAULT 0,
    ready_for_review_at TEXT,
    additions           INTEGER NOT NULL DEFAULT 0,
    deletions           INTEGER NOT NULL DEFAULT 0,
    changed_files       INTEGER NOT NULL DEFAULT 0,
    base_branch         TEXT,
    head_branch         TEXT,
    body                TEXT,
    raw_data            TEXT,
    UNIQUE(repo_id, number)
);

CREATE TABLE IF NOT EXISTS branch_info (
    pr_id               INTEGER PRIMARY KEY REFERENCES pull_requests(id),
    merge_base_sha      TEXT,
    first_commit_date   TEXT,
    commits_count       INTEGER NOT NULL DEFAULT 0,
    total_additions     INTEGER NOT NULL DEFAULT 0,
    total_deletions     INTEGER NOT NULL DEFAULT 0,
    raw_data            TEXT
);

CREATE TABLE IF NOT EXISTS file_changes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    pr_id       INTEGER NOT NULL REFERENCES pull_requests(id),
    filename    TEXT NOT NULL,
    additions   INTEGER NOT NULL DEFAULT 0,
    deletions   INTEGER NOT NULL DEFAULT 0,
    is_test     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS timeline_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    pr_id       INTEGER NOT NULL REFERENCES pull_requests(id),
    event_type  TEXT NOT NULL,
    created_at  TEXT,
    actor       TEXT,
    raw_data    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS fetch_metadata (
    repo_id         INTEGER PRIMARY KEY REFERENCES repositories(id),
    last_fetch_at   TEXT NOT NULL,
    last_updated_at TEXT,
    pr_count        INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS schema_version (
    version     INTEGER NOT NULL,
    applied_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_file_changes_pr_id ON file_changes(pr_id);
CREATE INDEX IF NOT EXISTS idx_timeline_pr_id ON timeline_events(pr_id);
CREATE INDEX IF NOT EXISTS idx_prs_repo_state ON pull_requests(repo_id, state);
CREATE INDEX IF NOT EXISTS idx_prs_author ON pull_requests(author);
CREATE INDEX IF NOT EXISTS idx_prs_merged_at ON pull_requests(merged_at);
CREATE INDEX IF NOT EXISTS idx_prs_created_at ON pull_requests(created_at);
CREATE INDEX IF NOT EXISTS idx_prs_updated_at ON pull_requests(updated_at);
`
