package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var remoteFlag string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Push/fetch the backlog ref namespace, reconciling divergent op-logs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			report, err := store.Sync(remoteFlag)
			if err != nil {
				return err
			}
			fmt.Printf("synced with %s\n", report.Remote)
			if n := len(report.Adopted); n > 0 {
				fmt.Printf("  adopted %d item(s) new from remote\n", n)
			}
			if n := len(report.FastForwarded); n > 0 {
				fmt.Printf("  fast-forwarded %d item(s)\n", n)
			}
			if n := len(report.Merged); n > 0 {
				fmt.Printf("  reconciled %d item(s) with divergent edits\n", n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&remoteFlag, "remote", "", "remote to sync with (default: origin, or the only configured remote)")
	return cmd
}
