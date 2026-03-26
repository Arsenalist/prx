// Package sync implements the smart fetch/sync engine with conflict resolution.
package sync

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Arsenalist/prx/internal/provider"
	"github.com/Arsenalist/prx/internal/store"
)

// Result holds the outcome of a sync operation for one repo.
type Result struct {
	Repo     string
	New      int
	Updated  int
	Skipped  int
	Errors   int
	Duration time.Duration
}

// Options controls sync behavior.
type Options struct {
	Full         bool     // Ignore incremental state, re-fetch everything
	DryRun       bool     // Don't write to DB
	States       []string // PR states to fetch
	PerPage      int
	TestPatterns []*regexp.Regexp
	Verbose      bool
	Log          func(format string, args ...interface{}) // Progress callback
}

// Engine orchestrates the fetch-and-store pipeline.
type Engine struct {
	provider provider.VCSProvider
	store    store.Store
}

// NewEngine creates a sync engine.
func NewEngine(p provider.VCSProvider, s store.Store) *Engine {
	return &Engine{provider: p, store: s}
}

// SyncRepo fetches PR data for a single repo and stores it.
func (e *Engine) SyncRepo(instanceID, repoID int64, owner, repo string, opts Options) (*Result, error) {
	start := time.Now()
	result := &Result{Repo: owner + "/" + repo}
	log := opts.Log
	if log == nil {
		log = func(string, ...interface{}) {}
	}

	// Determine the "since" timestamp for incremental fetch
	var since string
	if !opts.Full {
		meta, err := e.store.GetFetchMetadata(repoID)
		if err != nil {
			return nil, fmt.Errorf("reading fetch metadata: %w", err)
		}
		if meta != nil {
			since = meta.LastUpdatedAt
			if opts.Verbose {
				log("  Last fetch: %s (%d PRs)", meta.LastFetchAt, meta.PRCount)
			}
		} else if opts.Verbose {
			log("  First fetch for this repo")
		}
	}

	// Fetch PR list from API
	state := "all"
	if len(opts.States) > 0 {
		state = strings.Join(opts.States, ",")
		// GitHub API only accepts single state values; use "all" if multiple
		if len(opts.States) > 1 {
			state = "all"
		}
	}

	if since != "" {
		log("  Fetching PRs updated since %s...", since[:min(10, len(since))])
	} else {
		log("  Fetching all PRs...")
	}

	prs, err := e.provider.ListPullRequests(owner, repo, provider.ListPROptions{
		State:   state,
		PerPage: opts.PerPage,
		Since:   since,
	})
	if err != nil {
		return nil, fmt.Errorf("listing PRs: %w", err)
	}

	total := len(prs)
	log("  Found %d PRs to process", total)

	var maxUpdatedAt string

	for i, pr := range prs {
		if pr.UpdatedAt > maxUpdatedAt {
			maxUpdatedAt = pr.UpdatedAt
		}

		action := e.decideAction(repoID, pr, opts.Full)
		pos := fmt.Sprintf("[%d/%d]", i+1, total)

		switch action {
		case actionSkip:
			result.Skipped++
			if opts.Verbose {
				log("  %s #%d %s — skipped (already stored)", pos, pr.Number, truncate(pr.Title, 40))
			}
			continue

		case actionFetchFull:
			if opts.DryRun {
				log("  %s #%d %s — would fetch (new)", pos, pr.Number, truncate(pr.Title, 40))
				result.New++
				continue
			}
			if opts.Verbose {
				log("  %s #%d %s — fetching...", pos, pr.Number, truncate(pr.Title, 40))
			} else if total > 5 && (i+1)%10 == 0 {
				log("  %s processing...", pos)
			}
			if err := e.fetchAndStoreFull(repoID, owner, repo, pr, opts); err != nil {
				log("  %s #%d — error: %v", pos, pr.Number, err)
				result.Errors++
				continue
			}
			result.New++

		case actionRefetch:
			if opts.DryRun {
				log("  %s #%d %s — would re-fetch (updated)", pos, pr.Number, truncate(pr.Title, 40))
				result.Updated++
				continue
			}
			if opts.Verbose {
				log("  %s #%d %s — re-fetching (state changed)...", pos, pr.Number, truncate(pr.Title, 40))
			} else if total > 5 && (i+1)%10 == 0 {
				log("  %s processing...", pos)
			}
			if err := e.fetchAndStoreFull(repoID, owner, repo, pr, opts); err != nil {
				log("  %s #%d — error: %v", pos, pr.Number, err)
				result.Errors++
				continue
			}
			result.Updated++
		}
	}

	// Update fetch metadata
	if !opts.DryRun {
		totalPRs := result.New + result.Updated + result.Skipped
		if maxUpdatedAt == "" {
			maxUpdatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		e.store.UpdateFetchMetadata(store.FetchMetadataRecord{
			RepoID:        repoID,
			LastFetchAt:   time.Now().UTC().Format(time.RFC3339),
			LastUpdatedAt: maxUpdatedAt,
			PRCount:       totalPRs,
		})
	}

	result.Duration = time.Since(start)
	return result, nil
}

type action int

const (
	actionSkip action = iota
	actionFetchFull
	actionRefetch
)

// decideAction implements the smart sync decision matrix from SPEC §7.1.
func (e *Engine) decideAction(repoID int64, pr provider.PullRequest, full bool) action {
	if full {
		return actionFetchFull
	}

	existing, err := e.store.GetPRState(repoID, pr.Number)
	if err != nil || existing == nil {
		// New PR — full fetch
		return actionFetchFull
	}

	// PR exists in DB
	apiState := pr.State
	dbState := existing.State

	// Closed/merged in API and DB → skip (immutable)
	if (apiState == "closed" || apiState == "merged") && (dbState == "closed" || dbState == "merged") {
		return actionSkip
	}

	// Any other state change → re-fetch
	return actionRefetch
}

func (e *Engine) fetchAndStoreFull(repoID int64, owner, repo string, listPR provider.PullRequest, opts Options) error {
	log := opts.Log
	if log == nil {
		log = func(string, ...interface{}) {}
	}

	// Get full PR details (list endpoint doesn't include additions/deletions)
	if opts.Verbose {
		log("    → fetching PR details")
	}
	fullPR, err := e.provider.GetPullRequest(owner, repo, listPR.Number)
	if err != nil {
		return fmt.Errorf("getting PR #%d details: %w", listPR.Number, err)
	}

	// Store PR
	prID, err := e.store.UpsertPullRequest(prToRecord(repoID, fullPR))
	if err != nil {
		return fmt.Errorf("storing PR #%d: %w", listPR.Number, err)
	}

	// Fetch branch comparison (using SHAs, works even if branch is deleted)
	if fullPR.BaseSHA != "" && fullPR.HeadSHA != "" {
		if opts.Verbose {
			log("    → fetching file changes")
		}
		comp, err := e.provider.GetBranchComparison(owner, repo, fullPR.BaseSHA, fullPR.HeadSHA)
		if err != nil {
			if opts.Verbose {
				log("    → warning: could not get file changes: %v", err)
			}
		} else {
			rawJSON := comp.RawJSON
			e.store.UpsertBranchInfo(store.BranchInfoRecord{
				PRID:            prID,
				MergeBaseSHA:    &comp.MergeBaseSHA,
				FirstCommitDate: &comp.FirstCommitDate,
				CommitsCount:    comp.CommitsCount,
				TotalAdditions:  comp.TotalAdditions,
				TotalDeletions:  comp.TotalDeletions,
				RawData:         &rawJSON,
			})

			// Classify and store file changes
			var files []store.FileChangeRecord
			for _, f := range comp.Files {
				files = append(files, store.FileChangeRecord{
					Filename:  f.Filename,
					Additions: f.Additions,
					Deletions: f.Deletions,
					IsTest:    isTestFile(f.Filename, opts.TestPatterns),
				})
			}
			e.store.ReplaceFileChanges(prID, files)
			if opts.Verbose {
				log("    → stored %d file changes (%d+/%d-)", len(files), comp.TotalAdditions, comp.TotalDeletions)
			}
		}
	}

	// Fetch timeline events
	if opts.Verbose {
		log("    → fetching timeline events")
	}
	events, err := e.provider.GetTimelineEvents(owner, repo, listPR.Number)
	if err != nil {
		if opts.Verbose {
			log("    → warning: could not get timeline: %v", err)
		}
	} else {
		var records []store.TimelineEventRecord
		for _, evt := range events {
			actor := evt.Actor
			createdAt := evt.CreatedAt
			rec := store.TimelineEventRecord{
				EventType: evt.EventType,
				CreatedAt: &createdAt,
				Actor:     &actor,
				RawData:   evt.RawJSON,
			}
			if evt.ReviewState != "" {
				reviewState := evt.ReviewState
				rec.ReviewState = &reviewState
			}
			records = append(records, rec)
		}
		e.store.ReplaceTimelineEvents(prID, records)
		if opts.Verbose {
			log("    → stored %d timeline events", len(records))
		}

		// Extract ready_for_review_at and update PR
		for _, evt := range events {
			if evt.EventType == "ready_for_review" {
				readyAt := evt.CreatedAt
				updated := prToRecord(repoID, fullPR)
				updated.ReadyForReviewAt = &readyAt
				e.store.UpsertPullRequest(updated)
				break
			}
		}
	}

	return nil
}

func prToRecord(repoID int64, pr *provider.PullRequest) store.PullRequestRecord {
	baseBranch := pr.BaseBranch
	headBranch := pr.HeadBranch
	body := pr.Body
	rawData := pr.RawJSON

	rec := store.PullRequestRecord{
		RepoID:             repoID,
		Number:             pr.Number,
		Title:              pr.Title,
		State:              pr.State,
		Author:             pr.Author,
		URL:                pr.URL,
		CreatedAt:          pr.CreatedAt,
		UpdatedAt:          pr.UpdatedAt,
		MergedAt:           pr.MergedAt,
		ClosedAt:           pr.ClosedAt,
		IsDraft:            pr.IsDraft,
		Additions:          pr.Additions,
		Deletions:          pr.Deletions,
		ChangedFiles:       pr.ChangedFiles,
		BaseBranch:         &baseBranch,
		HeadBranch:         &headBranch,
		Body:               &body,
		CommentCount:       pr.CommentCount,
		ReviewCommentCount: pr.ReviewCommentCount,
	}
	if rawData != "" {
		rec.RawData = &rawData
	}
	if pr.MergedBy != "" {
		mergedBy := pr.MergedBy
		rec.MergedBy = &mergedBy
	}
	return rec
}

func isTestFile(filename string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(filename) {
			return true
		}
	}
	return false
}

// CompileTestPatterns compiles string patterns into regexps, skipping invalid ones.
func CompileTestPatterns(patterns []string) []*regexp.Regexp {
	var compiled []*regexp.Regexp
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
