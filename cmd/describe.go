package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newDescribeCmd() *cobra.Command {
	var asAgent bool
	cmd := &cobra.Command{
		Use:   "describe <id> <text>",
		Short: "Set or clear an item's description",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			identity, err := resolveAgentIdentity(asAgent)
			if err != nil {
				return err
			}
			item, err := store.SetDescription(args[0], args[1], identity)
			if err != nil {
				return err
			}
			fmt.Printf("Updated item description successfully: %s\n", store.ShortID(item.ID))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asAgent, "as-agent", false, asAgentFlagUsage)
	return cmd
}
