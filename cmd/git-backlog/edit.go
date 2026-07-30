package main

import (
	"fmt"
	"strings"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	var titleFlag string
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit an item's title",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			if !cmd.Flags().Changed("title") {
				return fmt.Errorf("nothing to edit; pass --title")
			}
			title := strings.TrimSpace(titleFlag)
			if title == "" {
				return fmt.Errorf("title must not be empty")
			}
			item, err := store.SetTitle(args[0], title)
			if err != nil {
				return err
			}
			fmt.Printf("%s  %s\n", store.ShortID(item.ID), item.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&titleFlag, "title", "", "new title")
	return cmd
}
