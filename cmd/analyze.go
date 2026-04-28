package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Arsenalist/prx/internal/config"
	"github.com/Arsenalist/prx/internal/hooks"
	"github.com/Arsenalist/prx/internal/metrics"
	"github.com/Arsenalist/prx/internal/report"
	"github.com/Arsenalist/prx/internal/store"
	"github.com/Arsenalist/prx/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Calculate and display metrics from fetched PR data",
	RunE:  runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringSlice("team", nil, "analyze a specific team (repeatable)")
	analyzeCmd.Flags().StringSlice("repo", nil, "analyze specific repo(s) (repeatable)")
	analyzeCmd.Flags().StringSlice("author", nil, "filter to specific author(s)")
	analyzeCmd.Flags().String("start", "", "start date (YYYY-MM-DD)")
	analyzeCmd.Flags().String("end", "", "end date (YYYY-MM-DD)")
	analyzeCmd.Flags().String("preset", "", "date preset (e.g., last-30d, this-week)")
	analyzeCmd.Flags().String("group-by", "", "group results: repo, team, author")
	analyzeCmd.Flags().String("sort", "", "sort: prs-per-week, total-prs, avg-loc, avg-time-to-merge")
	analyzeCmd.Flags().Int("top", 0, "show only top N developers")
	analyzeCmd.Flags().String("output", "", "output directory for markdown files")
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	settings, err := loadSettings(db)
	if err != nil {
		return err
	}

	// Resolve date range
	startDate, endDate := resolveDateRange(cmd, settings)

	// Resolve repos to analyze
	repoFlags, _ := cmd.Flags().GetStringSlice("repo")
	teamFlags, _ := cmd.Flags().GetStringSlice("team")
	authors, _ := cmd.Flags().GetStringSlice("author")

	startStr := startDate.Format("2006-01-02")
	endStr := endDate.Format("2006-01-02")

	filters := store.PRFilters{
		Authors:   authors,
		StartDate: startStr,
		EndDate:   endStr,
	}

	// If --team specified, resolve to repo records (with IDs) from DB
	var teamRepoIDs []int64
	if len(teamFlags) > 0 {
		teamRepos, err := resolveTeamReposFromDB(db, teamFlags)
		if err != nil {
			return err
		}
		for _, r := range teamRepos {
			teamRepoIDs = append(teamRepoIDs, r.ID)
		}
	}

	repoNames, err := resolveRepoIDs(db, repoFlags, &filters, teamRepoIDs)
	if err != nil {
		return err
	}

	prs, err := db.ListPullRequests(filters)
	if err != nil {
		return fmt.Errorf("listing PRs: %w", err)
	}

	if len(prs) == 0 {
		fmt.Fprintln(os.Stderr, "No PR data found. Run `prx fetch` first.")
		return nil
	}

	result := metrics.Calculate(prs, db, metrics.CalculateOptions{
		StartDate: startStr,
		EndDate:   endStr,
		Repos:     repoNames,
	})
	result.Meta.Team = teamNameFromFlags(teamFlags)

	if err := outputResult(cmd, settings, result); err != nil {
		return err
	}

	// Run post-analyze hooks
	if hookList, ok := settings.Hooks["post-analyze"]; ok && len(hookList) > 0 {
		jsonData, _ := report.FormatJSON(result)
		hooks.Run("post-analyze", hookList, []byte(jsonData), quiet)
	}

	return nil
}

func outputResult(cmd *cobra.Command, s *config.Settings, result *metrics.AnalysisResult) error {
	outputFormat := s.Output.Format
	if format != "" {
		outputFormat = format
	}

	switch outputFormat {
	case "json":
		output, err := report.FormatJSON(result)
		if err != nil {
			return err
		}
		fmt.Println(output)

	case "table":
		fmt.Print(report.FormatTable(result))

	case "markdown":
		return writeMarkdown(cmd, s, result)

	case "all":
		// Table to stderr, JSON to stdout, markdown to file
		fmt.Fprint(os.Stderr, report.FormatTable(result))
		output, err := report.FormatJSON(result)
		if err != nil {
			return err
		}
		fmt.Println(output)
		return writeMarkdown(cmd, s, result)

	default:
		fmt.Print(report.FormatTable(result))
	}

	return nil
}

func writeMarkdown(cmd *cobra.Command, s *config.Settings, result *metrics.AnalysisResult) error {
	outputDir := s.Output.Directory
	if dir, _ := cmd.Flags().GetString("output"); dir != "" {
		outputDir = dir
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	filename := report.MarkdownFilename(result.Meta.Repos, result.Meta.Team, time.Now().Format("2006-01-02"))
	path := filepath.Join(outputDir, filename)

	content := report.FormatMarkdown(result)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing markdown report: %w", err)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "Report written to %s\n", path)
	}
	return nil
}

func resolveRepoIDs(db *sqlite.SQLiteStore, repoFlags []string, filters *store.PRFilters, teamRepoIDs []int64) ([]string, error) {
	repos, err := db.ListRepositories()
	if err != nil {
		return nil, err
	}

	// Build ID→name map for lookups
	idToName := make(map[int64]string)
	for _, r := range repos {
		idToName[r.ID] = r.FullName
	}

	// If team provided pre-resolved IDs, use them directly (no name matching)
	if len(teamRepoIDs) > 0 && len(repoFlags) == 0 {
		var names []string
		for _, id := range teamRepoIDs {
			if name, ok := idToName[id]; ok {
				names = append(names, name)
			}
		}
		filters.RepoIDs = teamRepoIDs
		return names, nil
	}

	// If team IDs + explicit repo flags, combine them
	if len(teamRepoIDs) > 0 && len(repoFlags) > 0 {
		nameToID := make(map[string]int64)
		for _, r := range repos {
			nameToID[r.FullName] = r.ID
		}
		ids := append([]int64{}, teamRepoIDs...)
		var names []string
		for _, id := range teamRepoIDs {
			if name, ok := idToName[id]; ok {
				names = append(names, name)
			}
		}
		for _, name := range repoFlags {
			if id, ok := nameToID[name]; ok {
				ids = append(ids, id)
				names = append(names, name)
			}
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("no matching repos found in database")
		}
		filters.RepoIDs = ids
		return names, nil
	}

	// No team, no flags — use all repos
	if len(repoFlags) == 0 && len(repos) > 0 {
		var ids []int64
		var names []string
		for _, r := range repos {
			ids = append(ids, r.ID)
			names = append(names, r.FullName)
		}
		filters.RepoIDs = ids
		return names, nil
	}

	// Match repo flags to DB repos by name
	nameToID := make(map[string]int64)
	for _, r := range repos {
		nameToID[r.FullName] = r.ID
	}

	var ids []int64
	var names []string
	for _, name := range repoFlags {
		if id, ok := nameToID[name]; ok {
			ids = append(ids, id)
			names = append(names, name)
		}
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no matching repos found in database")
	}

	filters.RepoIDs = ids
	return names, nil
}
