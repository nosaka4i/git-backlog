package main

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newPriorityCmd() *cobra.Command {
	return &cobra.Command{
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
			item, err := store.SetPriority(args[0], p)
			if err != nil {
				return err
			}
			priority := "unset"
			if item.Priority != store.PriorityNone {
				priority = string(item.Priority)
			}
			fmt.Printf("%s  priority: %s\n", store.ShortID(item.ID), priority)
			return nil
		},
	}
}
