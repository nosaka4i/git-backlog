package main

import (
	"fmt"
	"strings"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var listFlag, priorityFlag string
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
			item, err := store.CreateItem(title, list, priority)
			if err != nil {
				return err
			}
			fmt.Printf("%s  %s\n", store.ShortID(item.ID), item.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&listFlag, "list", string(store.ListBacklog), "backlog|current|closed")
	cmd.Flags().StringVar(&priorityFlag, "priority", "", "p0|p1|p2")
	return cmd
}
