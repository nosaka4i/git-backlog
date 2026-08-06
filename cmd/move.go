package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newMoveCmd() *cobra.Command {
	var asAgent bool
	cmd := &cobra.Command{
		Use:   "move <id> <backlog|current|closed>",
		Short: "Move an item to a different track",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			l, err := store.ParseTrack(args[1])
			if err != nil {
				return err
			}
			identity, err := resolveAgentIdentity(asAgent)
			if err != nil {
				return err
			}
			item, err := store.SetTrack(args[0], l, identity)
			if err != nil {
				return err
			}
			fmt.Printf("Moved item successfully: %s\n", store.ShortID(item.ID))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asAgent, "as-agent", false, asAgentFlagUsage)
	return cmd
}
