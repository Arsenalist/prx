package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var backfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Populate new columns from existing raw_data JSON",
	Long:  `Parses raw_data JSON stored in pull_requests and timeline_events to populate extracted fields (merged_by, comment_count, review_comment_count, review_state).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, cleanup, err := openDB()
		if err != nil {
			return err
		}
		defer cleanup()

		updated, err := db.BackfillExtractedFields()
		if err != nil {
			return fmt.Errorf("backfill failed: %w", err)
		}

		fmt.Printf("Backfill complete: %d rows updated\n", updated)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(backfillCmd)
}
