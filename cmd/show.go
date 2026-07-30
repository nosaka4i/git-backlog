package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show an item's full state and op-log history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			item, err := store.LoadItem(args[0])
			if err != nil {
				return err
			}
			priority := "unset"
			if item.Priority != store.PriorityNone {
				priority = string(item.Priority)
			}
			fmt.Printf("id:       %s\n", store.ShortID(item.ID))
			fmt.Printf("title:    %s\n", item.Title)
			fmt.Printf("list:     %s\n", item.List)
			fmt.Printf("priority: %s\n", priority)
			fmt.Printf("owner:    %s <%s>\n", item.OwnerName, item.OwnerEmail)
			fmt.Printf("created:  %s\n", item.CreatedAt.Format("2006-01-02 15:04:05 -0700"))
			fmt.Println()
			fmt.Println("history:")
			history, err := store.History(item.ID)
			if err != nil {
				return err
			}
			for _, op := range history {
				fmt.Printf("  %s  %s  %s <%s>\n",
					op.When.Format("2006-01-02 15:04:05 -0700"), op.Commit[:12], op.AuthorName, op.AuthorEmail)
				for _, ch := range op.Changes {
					if ch.Removed {
						fmt.Printf("      %s: (cleared)\n", ch.Field)
					} else {
						fmt.Printf("      %s: %s\n", ch.Field, ch.Value)
					}
				}
			}
			return nil
		},
	}
}
