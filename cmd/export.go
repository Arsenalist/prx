package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/Arsenalist/prx/internal/store"
	"github.com/Arsenalist/prx/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export PR data as JSON or CSV",
	RunE:  runExport,
}

func init() {
	exportCmd.Flags().StringSlice("repo", nil, "export specific repo(s)")
	exportCmd.Flags().StringSlice("team", nil, "export team's repos")
	exportCmd.Flags().String("start", "", "filter by start date")
	exportCmd.Flags().String("end", "", "filter by end date")
	exportCmd.Flags().String("preset", "", "date preset")
	exportCmd.Flags().String("output", "", "output file (default: stdout)")
	exportCmd.Flags().String("export-format", "json", "export format: json, csv")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
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

	startDate, endDate := resolveDateRange(cmd, cfg)
	repoFlags, _ := cmd.Flags().GetStringSlice("repo")
	teamFlags, _ := cmd.Flags().GetStringSlice("team")

	startStr := startDate.Format("2006-01-02")
	endStr := endDate.Format("2006-01-02")

	filters := store.PRFilters{
		StartDate: startStr,
		EndDate:   endStr,
	}

	if len(teamFlags) > 0 {
		teamRefs := resolveTeamRepos(cfg, teamFlags)
		for _, ref := range teamRefs {
			repoFlags = append(repoFlags, ref.repo)
		}
	}

	if _, err := resolveRepoIDs(db, cfg, repoFlags, &filters); err != nil {
		return err
	}

	prs, err := db.ListPullRequests(filters)
	if err != nil {
		return fmt.Errorf("listing PRs: %w", err)
	}

	// Determine output writer
	out := os.Stdout
	if outFile, _ := cmd.Flags().GetString("output"); outFile != "" {
		f, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	exportFormat, _ := cmd.Flags().GetString("export-format")
	switch exportFormat {
	case "csv":
		return exportCSV(out, prs)
	default:
		return exportJSON(out, prs)
	}
}

func exportJSON(out *os.File, prs []store.PullRequestRecord) error {
	data, err := json.MarshalIndent(prs, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, string(data))
	return nil
}

func exportCSV(out *os.File, prs []store.PullRequestRecord) error {
	w := csv.NewWriter(out)
	defer w.Flush()

	header := []string{"number", "title", "author", "state", "created_at", "merged_at", "additions", "deletions", "changed_files"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, pr := range prs {
		mergedAt := ""
		if pr.MergedAt != nil {
			mergedAt = *pr.MergedAt
		}
		row := []string{
			strconv.Itoa(pr.Number),
			pr.Title,
			pr.Author,
			pr.State,
			pr.CreatedAt,
			mergedAt,
			strconv.Itoa(pr.Additions),
			strconv.Itoa(pr.Deletions),
			strconv.Itoa(pr.ChangedFiles),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
