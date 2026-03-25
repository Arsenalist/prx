package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Arsenalist/prx/internal/config"
	gh "github.com/Arsenalist/prx/internal/provider/github"
	"github.com/Arsenalist/prx/internal/store"
	"github.com/Arsenalist/prx/internal/store/sqlite"
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
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	full, _ := cmd.Flags().GetBool("full")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	repoFlags, _ := cmd.Flags().GetStringSlice("repo")
	teamFlags, _ := cmd.Flags().GetStringSlice("team")

	// Open database
	db := sqlite.New(cfg.Storage.SQLite.Path)
	if err := db.Open(); err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		return fmt.Errorf("migrating database: %w", err)
	}

	// Resolve which repos to fetch
	var repos []repoRef
	if len(teamFlags) > 0 {
		repos = resolveTeamRepos(cfg, teamFlags)
	} else {
		repos = resolveRepos(cfg, repoFlags)
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repos to fetch (check config or --repo flag)")
	}

	testPatterns := sync.CompileTestPatterns(cfg.TestPatterns)

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
		inst, ok := cfg.Instances[ref.instance]
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: instance %q not found, skipping %s\n", ref.instance, ref.repo)
			continue
		}

		// Resolve token
		token := os.Getenv(inst.Token.Env)
		if token == "" {
			return fmt.Errorf("token environment variable %s is not set (needed for instance %q)", inst.Token.Env, ref.instance)
		}

		// Create provider
		client := gh.NewClient(inst.BaseURL, token, inst.TLSSkipVerify, cfg.Fetch.MaxRetries)

		// Ensure instance and repo exist in DB
		instID, err := db.UpsertInstance(store.InstanceRecord{
			Name: ref.instance, Type: inst.Type, BaseURL: inst.BaseURL,
		})
		if err != nil {
			return fmt.Errorf("upserting instance: %w", err)
		}

		parts := strings.SplitN(ref.repo, "/", 2)
		repoID, err := db.UpsertRepository(store.RepositoryRecord{
			InstanceID: instID, Owner: parts[0], Name: parts[1], FullName: ref.repo,
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
				if rl.Remaining < cfg.Fetch.RateLimitBuffer {
					fmt.Fprintf(os.Stderr, "  Warning: only %d API requests remaining (resets at %s)\n", rl.Remaining, rl.ResetAt)
				}
			}
		}

		if !quiet {
			fmt.Fprintf(os.Stderr, "\n(%d/%d) %s\n", i+1, len(repos), ref.repo)
		}

		// Run sync
		engine := sync.NewEngine(client, db)
		opts := sync.Options{
			Full:         full,
			DryRun:       dryRun,
			States:       cfg.Fetch.States,
			PerPage:      cfg.Fetch.PerPage,
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

type repoRef struct {
	instance string
	repo     string
}

func resolveRepos(cfg *config.Config, repoFlags []string) []repoRef {
	// If --repo flags specified, find their instances
	if len(repoFlags) > 0 {
		var refs []repoRef
		for _, r := range repoFlags {
			// Find which instance this repo belongs to
			found := false
			for _, cfgRepo := range cfg.Repos {
				if cfgRepo.Repo == r {
					refs = append(refs, repoRef{instance: cfgRepo.Instance, repo: r})
					found = true
					break
				}
			}
			if !found {
				// Check teams
				for _, team := range cfg.Teams {
					for _, tr := range team.Repos {
						if tr.Repo == r {
							refs = append(refs, repoRef{instance: tr.Instance, repo: r})
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			if !found {
				// Default to first instance
				for instName := range cfg.Instances {
					refs = append(refs, repoRef{instance: instName, repo: r})
					break
				}
			}
		}
		return refs
	}

	// All repos from config
	var refs []repoRef
	for _, r := range cfg.Repos {
		refs = append(refs, repoRef{instance: r.Instance, repo: r.Repo})
	}
	for _, team := range cfg.Teams {
		for _, r := range team.Repos {
			refs = append(refs, repoRef{instance: r.Instance, repo: r.Repo})
		}
	}
	return refs
}
