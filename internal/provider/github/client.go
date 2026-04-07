// Package github implements the VCS provider interface for GitHub.
package github

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Arsenalist/prx/internal/provider"
)

// Client implements provider.VCSProvider for GitHub (and GitHub Enterprise).
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	maxRetries int
}

// NewClient creates a GitHub API client.
func NewClient(baseURL, token string, tlsSkipVerify bool, maxRetries int) *Client {
	transport := &http.Transport{}
	if tlsSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if maxRetries <= 0 {
		maxRetries = 3
	}

	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		token:      token,
		maxRetries: maxRetries,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

func (c *Client) Name() string { return "github" }

// --- Rate Limit ---

func (c *Client) GetRateLimit() (*provider.RateLimit, error) {
	resp, err := c.doRequest("GET", "/rate_limit", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Resources struct {
			Core struct {
				Limit     int   `json:"limit"`
				Remaining int   `json:"remaining"`
				Reset     int64 `json:"reset"`
			} `json:"core"`
		} `json:"resources"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &provider.RateLimit{
		Limit:     result.Resources.Core.Limit,
		Remaining: result.Resources.Core.Remaining,
		ResetAt:   time.Unix(result.Resources.Core.Reset, 0).UTC().Format(time.RFC3339),
	}, nil
}

// --- List Pull Requests ---

func (c *Client) ListPullRequests(owner, repo string, opts provider.ListPROptions) ([]provider.PullRequest, error) {
	state := opts.State
	if state == "" {
		state = "all"
	}
	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = 100
	}

	var allPRs []provider.PullRequest
	page := 1

	for {
		path := fmt.Sprintf("/repos/%s/%s/pulls?state=%s&sort=updated&direction=desc&per_page=%d&page=%d",
			owner, repo, state, perPage, page)

		resp, err := c.doRequest("GET", path, nil)
		if err != nil {
			return nil, fmt.Errorf("listing PRs page %d: %w", page, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		var ghPRs []ghPullRequest
		if err := json.Unmarshal(body, &ghPRs); err != nil {
			return nil, fmt.Errorf("parsing PR list: %w", err)
		}

		if len(ghPRs) == 0 {
			break
		}

		allBeforeSince := true
		for _, ghPR := range ghPRs {
			pr := normalizePR(ghPR)

			// Skip PRs updated after the until bound
			if opts.Until != "" && pr.UpdatedAt > opts.Until {
				continue
			}

			// Skip PRs updated before the since bound
			if opts.Since != "" && pr.UpdatedAt < opts.Since {
				continue
			}

			allPRs = append(allPRs, pr)
			if opts.Since != "" && pr.UpdatedAt >= opts.Since {
				allBeforeSince = false
			}
		}

		// Early stop: if all PRs on this page are older than our since filter
		if opts.Since != "" && allBeforeSince {
			break
		}

		// Check for next page
		if !hasNextPage(resp) {
			break
		}
		page++
	}

	return allPRs, nil
}

// --- Get Single PR ---

func (c *Client) GetPullRequest(owner, repo string, number int) (*provider.PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ghPR ghPullRequest
	if err := json.Unmarshal(body, &ghPR); err != nil {
		return nil, err
	}

	pr := normalizePR(ghPR)
	pr.RawJSON = string(body)
	return &pr, nil
}

// --- Branch Comparison ---

func (c *Client) GetBranchComparison(owner, repo, base, head string) (*provider.BranchComparison, error) {
	path := fmt.Sprintf("/repos/%s/%s/compare/%s...%s", owner, repo, base, head)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result ghCompareResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	comp := &provider.BranchComparison{
		MergeBaseSHA: result.MergeBaseCommit.SHA,
		CommitsCount: len(result.Commits),
		RawJSON:      string(body),
	}

	if len(result.Commits) > 0 {
		comp.FirstCommitDate = result.Commits[0].Commit.Author.Date
	}

	for _, f := range result.Files {
		comp.TotalAdditions += f.Additions
		comp.TotalDeletions += f.Deletions
		comp.Files = append(comp.Files, provider.FileChange{
			Filename:  f.Filename,
			Additions: f.Additions,
			Deletions: f.Deletions,
		})
	}

	return comp, nil
}

// --- Timeline Events ---

func (c *Client) GetTimelineEvents(owner, repo string, number int) ([]provider.TimelineEvent, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/timeline?per_page=100", owner, repo, number)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ghEvents []json.RawMessage
	if err := json.Unmarshal(body, &ghEvents); err != nil {
		return nil, err
	}

	var events []provider.TimelineEvent
	for _, raw := range ghEvents {
		var evt struct {
			Event     string `json:"event"`
			CreatedAt string `json:"created_at"`
			State     string `json:"state"`
			Actor     struct {
				Login string `json:"login"`
			} `json:"actor"`
			User struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		json.Unmarshal(raw, &evt)

		te := provider.TimelineEvent{
			EventType: evt.Event,
			CreatedAt: evt.CreatedAt,
			Actor:     evt.Actor.Login,
			RawJSON:   string(raw),
		}
		// For reviewed events, the actor is in "user" field and state indicates the review outcome
		if evt.Event == "reviewed" {
			te.ReviewState = evt.State
			if te.Actor == "" {
				te.Actor = evt.User.Login
			}
		}
		events = append(events, te)
	}

	return events, nil
}

// --- HTTP helpers ---

func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * time.Second) // exponential backoff
		}

		req, err := http.NewRequest(method, url, body)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Don't retry on client errors (except 429)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API %s %s: %d %s", method, path, resp.StatusCode, string(respBody))
		}

		// Rate limited — wait and retry
		if resp.StatusCode == 429 || resp.StatusCode == 403 {
			if resetStr := resp.Header.Get("X-RateLimit-Remaining"); resetStr == "0" {
				if resetTime := resp.Header.Get("X-RateLimit-Reset"); resetTime != "" {
					if ts, err := strconv.ParseInt(resetTime, 10, 64); err == nil {
						wait := time.Until(time.Unix(ts, 0)) + time.Second
						if wait > 0 && wait < 15*time.Minute {
							time.Sleep(wait)
							resp.Body.Close()
							continue
						}
					}
				}
			}
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API rate limited: %s", string(respBody))
		}

		// Server errors — retry
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("GitHub API %s %s: %d", method, path, resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("GitHub API request failed after %d retries: %w", c.maxRetries, lastErr)
}

var linkNextPattern = regexp.MustCompile(`<[^>]+>;\s*rel="next"`)

func hasNextPage(resp *http.Response) bool {
	link := resp.Header.Get("Link")
	return linkNextPattern.MatchString(link)
}

// --- GitHub API types ---

type ghPullRequest struct {
	Number         int     `json:"number"`
	Title          string  `json:"title"`
	State          string  `json:"state"`
	Draft          bool    `json:"draft"`
	HTMLURL        string  `json:"html_url"`
	Body           string  `json:"body"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	MergedAt       *string `json:"merged_at"`
	ClosedAt       *string `json:"closed_at"`
	Additions      int     `json:"additions"`
	Deletions      int     `json:"deletions"`
	ChangedFiles   int     `json:"changed_files"`
	Comments       int     `json:"comments"`
	ReviewComments int     `json:"review_comments"`
	User           struct {
		Login string `json:"login"`
	} `json:"user"`
	MergedBy *struct {
		Login string `json:"login"`
	} `json:"merged_by"`
	Base struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
}

type ghCompareResult struct {
	MergeBaseCommit struct {
		SHA string `json:"sha"`
	} `json:"merge_base_commit"`
	Commits []struct {
		Commit struct {
			Author struct {
				Date string `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	} `json:"commits"`
	Files []struct {
		Filename  string `json:"filename"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
	} `json:"files"`
}

func normalizePR(ghPR ghPullRequest) provider.PullRequest {
	state := ghPR.State
	if ghPR.MergedAt != nil {
		state = "merged"
	}

	pr := provider.PullRequest{
		Number:             ghPR.Number,
		Title:              ghPR.Title,
		State:              state,
		Author:             ghPR.User.Login,
		URL:                ghPR.HTMLURL,
		CreatedAt:          ghPR.CreatedAt,
		UpdatedAt:          ghPR.UpdatedAt,
		MergedAt:           ghPR.MergedAt,
		ClosedAt:           ghPR.ClosedAt,
		IsDraft:            ghPR.Draft,
		Additions:          ghPR.Additions,
		Deletions:          ghPR.Deletions,
		ChangedFiles:       ghPR.ChangedFiles,
		BaseBranch:         ghPR.Base.Ref,
		HeadBranch:         ghPR.Head.Ref,
		BaseSHA:            ghPR.Base.SHA,
		HeadSHA:            ghPR.Head.SHA,
		Body:               ghPR.Body,
		CommentCount:       ghPR.Comments,
		ReviewCommentCount: ghPR.ReviewComments,
	}
	if ghPR.MergedBy != nil {
		pr.MergedBy = ghPR.MergedBy.Login
	}
	return pr
}
