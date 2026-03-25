package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Arsenalist/prx/internal/config"
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
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	db := sqlite.New(cfg.Storage.SQLite.Path)
	if err := db.Open(); err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		return fmt.Errorf("migrating database: %w", err)
	}

	// Resolve date range
	startDate, endDate := resolveDateRange(cmd, cfg)

	// Resolve repos to analyze
	repoFlags, _ := cmd.Flags().GetStringSlice("repo")
	authors, _ := cmd.Flags().GetStringSlice("author")

	startStr := startDate.Format("2006-01-02")
	endStr := endDate.Format("2006-01-02")

	filters := store.PRFilters{
		Authors:   authors,
		StartDate: startStr,
		EndDate:   endStr,
	}

	repoNames, err := resolveRepoIDs(db, cfg, repoFlags, &filters)
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

	return outputResult(cmd, cfg, result)
}

func outputResult(cmd *cobra.Command, cfg *config.Config, result *metrics.AnalysisResult) error {
	outputFormat := cfg.Output.Format
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
		return writeMarkdown(cmd, cfg, result)

	case "all":
		// Table to stderr, JSON to stdout, markdown to file
		fmt.Fprint(os.Stderr, report.FormatTable(result))
		output, err := report.FormatJSON(result)
		if err != nil {
			return err
		}
		fmt.Println(output)
		return writeMarkdown(cmd, cfg, result)

	default:
		fmt.Print(report.FormatTable(result))
	}

	return nil
}

func writeMarkdown(cmd *cobra.Command, cfg *config.Config, result *metrics.AnalysisResult) error {
	outputDir := cfg.Output.Directory
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

func resolveRepoIDs(db *sqlite.SQLiteStore, cfg *config.Config, repoFlags []string, filters *store.PRFilters) ([]string, error) {
	repos, err := db.ListRepositories()
	if err != nil {
		return nil, err
	}

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

	// Match flags to DB repos
	repoMap := make(map[string]int64)
	for _, r := range repos {
		repoMap[r.FullName] = r.ID
	}

	var ids []int64
	var names []string
	for _, name := range repoFlags {
		if id, ok := repoMap[name]; ok {
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
