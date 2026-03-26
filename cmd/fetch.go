package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	gh "github.com/Arsenalist/prx/internal/provider/github"
	"github.com/Arsenalist/prx/internal/store"
	"github.com/Arsenalist/prx/internal/sync"
	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch PR data from GitHub and store locally",
	RunE:  runFetch,
}

func init() {
	fetchCmd.Flags().StringSlice("team", nil, "fetch for a specific team's repos")
	fetchCmd.Flags().StringSlice("repo", nil, "fetch for specific repo(s) (owner/repo)")
	fetchCmd.Flags().String("instance", "", "limit to repos from this instance")
	fetchCmd.Flags().Bool("full", false, "re-fetch all data (ignore incremental state)")
	fetchCmd.Flags().Bool("dry-run", false, "show what would be fetched without making API calls")
	fetchCmd.Flags().String("since", "", "only fetch PRs updated after this date")
	fetchCmd.Flags().String("states", "", "PR states: open,closed,all")
	rootCmd.AddCommand(fetchCmd)
}

func runFetch(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	settings, err := loadSettings(db)
	if err != nil {
		return err
	}

	full, _ := cmd.Flags().GetBool("full")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	repoFlags, _ := cmd.Flags().GetStringSlice("repo")
	teamFlags, _ := cmd.Flags().GetStringSlice("team")

	// Resolve which repos to fetch
	type repoRef struct {
		instanceName string
		repoFullName string
	}
	var repos []repoRef

	if len(teamFlags) > 0 {
		// Expand teams from DB
		for _, teamName := range teamFlags {
			team, err := db.GetTeamByName(teamName)
			if err != nil {
				return err
			}
			if team == nil {
				fmt.Fprintf(os.Stderr, "Warning: team %q not found, skipping\n", teamName)
				continue
			}
			teamRepos, err := db.GetTeamRepos(team.ID)
			if err != nil {
				return err
			}
			for _, r := range teamRepos {
				// Find instance name for this repo
				instances, _ := db.ListInstances()
				for _, inst := range instances {
					if inst.ID == r.InstanceID {
						repos = append(repos, repoRef{instanceName: inst.Name, repoFullName: r.FullName})
						break
					}
				}
			}
		}
	} else if len(repoFlags) > 0 {
		// Find repos in DB by full name
		allRepos, _ := db.ListRepositories()
		instances, _ := db.ListInstances()
		instMap := make(map[int64]string)
		for _, inst := range instances {
			instMap[inst.ID] = inst.Name
		}
		repoSet := make(map[string]bool)
		for _, r := range repoFlags {
			repoSet[r] = true
		}
		for _, r := range allRepos {
			if repoSet[r.FullName] {
				repos = append(repos, repoRef{instanceName: instMap[r.InstanceID], repoFullName: r.FullName})
			}
		}
		// If repo not found in DB, default to first instance
		for _, r := range repoFlags {
			found := false
			for _, ref := range repos {
				if ref.repoFullName == r {
					found = true
					break
				}
			}
			if !found && len(instances) > 0 {
				repos = append(repos, repoRef{instanceName: instances[0].Name, repoFullName: r})
			}
		}
	} else {
		// All repos in DB
		allRepos, _ := db.ListRepositories()
		instances, _ := db.ListInstances()
		instMap := make(map[int64]string)
		for _, inst := range instances {
			instMap[inst.ID] = inst.Name
		}
		for _, r := range allRepos {
			repos = append(repos, repoRef{instanceName: instMap[r.InstanceID], repoFullName: r.FullName})
		}
	}

	if len(repos) == 0 {
		return fmt.Errorf("no repos to fetch (use `prx repo add` to add repos first)")
	}

	testPatterns := sync.CompileTestPatterns(settings.TestPatterns)

	if !quiet {
		mode := "incremental"
		if full {
			mode = "full"
		}
		if dryRun {
			mode = "dry-run"
		}
		fmt.Fprintf(os.Stderr, "Fetching %d repo(s) [%s]\n", len(repos), mode)
	}

	for i, ref := range repos {
		inst, err := db.GetInstanceByName(ref.instanceName)
		if err != nil || inst == nil {
			fmt.Fprintf(os.Stderr, "Warning: instance %q not found, skipping %s\n", ref.instanceName, ref.repoFullName)
			continue
		}

		// Resolve token: configured env var → common env vars → git config
		token := resolveToken(inst.TokenEnv)

		// Create provider
		client := gh.NewClient(inst.BaseURL, token, inst.TLSSkipVerify, settings.Fetch.MaxRetries)

		// Ensure instance and repo exist in DB
		instID, err := db.UpsertInstance(store.InstanceRecord{
			Name: inst.Name, Type: inst.Type, BaseURL: inst.BaseURL,
			TokenEnv: inst.TokenEnv, TLSSkipVerify: inst.TLSSkipVerify,
		})
		if err != nil {
			return fmt.Errorf("upserting instance: %w", err)
		}

		parts := strings.SplitN(ref.repoFullName, "/", 2)
		repoID, err := db.UpsertRepository(store.RepositoryRecord{
			InstanceID: instID, Owner: parts[0], Name: parts[1], FullName: ref.repoFullName,
		})
		if err != nil {
			return fmt.Errorf("upserting repository: %w", err)
		}

		// Check rate limit
		if !quiet {
			rl, err := client.GetRateLimit()
			if err == nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "  API rate limit: %d/%d remaining (resets %s)\n", rl.Remaining, rl.Limit, rl.ResetAt)
				}
				if rl.Remaining < settings.Fetch.RateLimitBuffer {
					fmt.Fprintf(os.Stderr, "  Warning: only %d API requests remaining (resets at %s)\n", rl.Remaining, rl.ResetAt)
				}
			}
		}

		if !quiet {
			fmt.Fprintf(os.Stderr, "\n(%d/%d) %s\n", i+1, len(repos), ref.repoFullName)
		}

		// Run sync
		engine := sync.NewEngine(client, db)
		opts := sync.Options{
			Full:         full,
			DryRun:       dryRun,
			States:       settings.Fetch.States,
			PerPage:      settings.Fetch.PerPage,
			TestPatterns: testPatterns,
			Verbose:      verbose,
			Log: func(format string, args ...interface{}) {
				if !quiet {
					fmt.Fprintf(os.Stderr, format+"\n", args...)
				}
			},
		}

		result, err := engine.SyncRepo(instID, repoID, parts[0], parts[1], opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			continue
		}

		if !quiet {
			parts := []string{}
			if result.New > 0 {
				parts = append(parts, fmt.Sprintf("%d new", result.New))
			}
			if result.Updated > 0 {
				parts = append(parts, fmt.Sprintf("%d updated", result.Updated))
			}
			if result.Skipped > 0 {
				parts = append(parts, fmt.Sprintf("%d skipped", result.Skipped))
			}
			if result.Errors > 0 {
				parts = append(parts, fmt.Sprintf("%d errors", result.Errors))
			}
			summary := strings.Join(parts, ", ")
			if summary == "" {
				summary = "nothing to do"
			}
			fmt.Fprintf(os.Stderr, "  Done: %s (%s)\n", summary, result.Duration.Round(time.Millisecond))
		}
	}

	if !quiet {
		fmt.Fprintln(os.Stderr)
	}

	return nil
}

// resolveToken tries multiple sources to find a GitHub token:
// 1. The configured env var (e.g. GITHUB_TOKEN)
// 2. Common env vars: GH_TOKEN, GITHUB_TOKEN, GITHUB_API_TOKEN, GHE_TOKEN
// 3. git config --global rbc-ctx.oauth-token (set by enterprise tooling)
func resolveToken(configuredEnv string) string {
	if v := os.Getenv(configuredEnv); v != "" {
		return v
	}
	for _, env := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITHUB_API_TOKEN", "GHE_TOKEN"} {
		if env == configuredEnv {
			continue // already checked
		}
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	out, err := exec.Command("git", "config", "--global", "rbc-ctx.oauth-token").Output()
	if err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v
		}
	}
	return ""
}
