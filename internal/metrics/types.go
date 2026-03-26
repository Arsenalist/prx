package metrics

// AnalysisResult holds the complete output of an analysis run.
type AnalysisResult struct {
	Meta          Meta             `json:"meta"`
	Summary       Summary          `json:"summary"`
	Developers    []DeveloperStats `json:"developers"`
	SlowestPRs    []PRTiming       `json:"slowest_prs"`
	RepoBreakdown []RepoStats      `json:"repo_breakdown,omitempty"`
	ReviewerStats []ReviewerStats  `json:"reviewer_stats,omitempty"`
	PRs           []PRDetail       `json:"prs,omitempty"`
}

// Meta contains metadata about the analysis run.
type Meta struct {
	Tool      string   `json:"tool"`
	Version   string   `json:"version"`
	DateRange Range    `json:"date_range"`
	Repos     []string `json:"repos"`
	Team      string   `json:"team,omitempty"`
}

// Range represents a date range.
type Range struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Summary holds aggregate metrics.
type Summary struct {
	TotalPRs      int           `json:"total_prs"`
	MergedPRs     int           `json:"merged_prs"`
	ClosedPRs     int           `json:"closed_prs"`
	OpenPRs       int           `json:"open_prs"`
	UniqueAuthors int           `json:"unique_authors"`
	LOC           LOCMetrics    `json:"loc"`
	Timing        TimingAvg     `json:"timing"`
	Review        ReviewMetrics `json:"review"`
}

// ReviewMetrics holds aggregate review metrics.
type ReviewMetrics struct {
	AvgTimeToFirstReviewHours    float64 `json:"avg_time_to_first_review_hours"`
	MedianTimeToFirstReviewHours float64 `json:"median_time_to_first_review_hours"`
	AvgReviewCycles              float64 `json:"avg_review_cycles"`
	AvgCommentsPerPR             float64 `json:"avg_comments_per_pr"`
}

// ReviewerStats holds per-reviewer metrics.
type ReviewerStats struct {
	Login                    string  `json:"login"`
	ReviewsGiven             int     `json:"reviews_given"`
	Approvals                int     `json:"approvals"`
	ChangesRequested         int     `json:"changes_requested"`
	AvgReviewTurnaroundHours float64 `json:"avg_review_turnaround_hours"`
}

// LOCMetrics holds lines of code metrics.
type LOCMetrics struct {
	Total      int     `json:"total"`
	Test       int     `json:"test"`
	Production int     `json:"production"`
	AvgPerPR   float64 `json:"avg_per_pr"`
	MedianPerPR float64 `json:"median_per_pr"`
}

// TimingAvg holds average timing metrics in hours.
type TimingAvg struct {
	AvgTimeToOpenHours    float64 `json:"avg_time_to_open_hours"`
	AvgTimeToMergeHours   float64 `json:"avg_time_to_merge_hours"`
	AvgTotalTimeHours     float64 `json:"avg_total_time_hours"`
	MedianTimeToMergeHours float64 `json:"median_time_to_merge_hours"`
}

// DeveloperStats holds per-developer metrics.
type DeveloperStats struct {
	Login      string  `json:"login"`
	MergedPRs  int     `json:"merged_prs"`
	PRsPerWeek float64 `json:"prs_per_week"`
	LOC        struct {
		AvgTotal      float64 `json:"avg_total"`
		AvgTest       float64 `json:"avg_test"`
		AvgProduction float64 `json:"avg_production"`
	} `json:"loc"`
	Timing struct {
		AvgTimeToOpenHours  float64 `json:"avg_time_to_open_hours"`
		AvgDraftTimeHours   float64 `json:"avg_draft_time_hours"`
		AvgTimeToMergeHours float64 `json:"avg_time_to_merge_hours"`
		AvgTotalTimeHours   float64 `json:"avg_total_time_hours"`
	} `json:"timing"`
}

// PRTiming holds timing info for a single PR (for slowest PRs table).
type PRTiming struct {
	Repo              string  `json:"repo"`
	Number            int     `json:"number"`
	Author            string  `json:"author"`
	Title             string  `json:"title"`
	URL               string  `json:"url"`
	TimeToOpenHours   float64 `json:"time_to_open_hours"`
	DraftTimeHours    float64 `json:"draft_time_hours"`
	TimeToMergeHours  float64 `json:"time_to_merge_hours"`
	TotalTimeHours    float64 `json:"total_time_hours"`
	LOCTotal          int     `json:"loc_total"`
	LOCTest           int     `json:"loc_test"`
	LOCProduction     int     `json:"loc_production"`
}

// PRDetail holds per-PR detail for JSON output.
type PRDetail struct {
	Repo               string  `json:"repo"`
	Number             int     `json:"number"`
	Title              string  `json:"title"`
	Author             string  `json:"author"`
	State              string  `json:"state"`
	URL                string  `json:"url"`
	Body               string  `json:"body"`
	CreatedAt          string  `json:"created_at"`
	MergedAt           *string `json:"merged_at,omitempty"`
	CommentCount       int     `json:"comment_count"`
	ReviewCommentCount int     `json:"review_comment_count"`
}

// RepoStats holds per-repo breakdown.
type RepoStats struct {
	Repo          string `json:"repo"`
	MergedPRs     int    `json:"merged_prs"`
	UniqueAuthors int    `json:"unique_authors"`
	LOCTotal      int    `json:"loc_total"`
	LOCTest       int    `json:"loc_test"`
	LOCProduction int    `json:"loc_production"`
}
