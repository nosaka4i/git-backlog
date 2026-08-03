package cmd

import (
	"fmt"
	"strings"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var listFlag, priorityFlag string
	var asAgent bool
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Create a new backlog item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			title := strings.TrimSpace(args[0])
			if title == "" {
				return fmt.Errorf("title must not be empty")
			}
			list, err := store.ParseList(listFlag)
			if err != nil {
				return err
			}
			var priority store.Priority
			if priorityFlag != "" {
				priority, err = store.ParsePriority(priorityFlag)
				if err != nil {
					return err
				}
			}
			identity, err := resolveAgentIdentity(asAgent)
			if err != nil {
				return err
			}
			item, err := store.CreateItem(title, list, priority, identity)
			if err != nil {
				return err
			}
			fmt.Printf("Added item successfully: %s\n", store.ShortID(item.ID))
			return nil
		},
	}
	cmd.Flags().StringVar(&listFlag, "list", string(store.ListBacklog), "backlog|current|closed")
	cmd.Flags().StringVar(&priorityFlag, "priority", "", "p0|p1|p2")
	cmd.Flags().BoolVar(&asAgent, "as-agent", false,
		"create this item owned by the agent identity from backlog.agent.name/backlog.agent.email, instead of the ambient git identity — owner is permanent, see docs/design/git-backlog.md")
	return cmd
}
