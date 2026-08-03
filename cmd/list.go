package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var asAgent bool
	cmd := &cobra.Command{
		Use:   "list <id> <backlog|current|closed>",
		Short: "Move an item to a different list",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			l, err := store.ParseList(args[1])
			if err != nil {
				return err
			}
			identity, err := resolveAgentIdentity(asAgent)
			if err != nil {
				return err
			}
			item, err := store.SetList(args[0], l, identity)
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
