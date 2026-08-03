package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newPriorityCmd() *cobra.Command {
	var asAgent bool
	cmd := &cobra.Command{
		Use:   "priority <id> <p0|p1|p2|none>",
		Short: "Set or clear an item's priority",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			p, err := store.ParsePriority(args[1])
			if err != nil {
				return err
			}
			identity, err := resolveAgentIdentity(asAgent)
			if err != nil {
				return err
			}
			item, err := store.SetPriority(args[0], p, identity)
			if err != nil {
				return err
			}
			fmt.Printf("Updated item priority successfully: %s\n", store.ShortID(item.ID))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asAgent, "as-agent", false, asAgentFlagUsage)
	return cmd
}
