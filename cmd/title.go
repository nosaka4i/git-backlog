package cmd

import (
	"fmt"
	"strings"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newTitleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "title <id> <new title>",
		Short: "Rename an item",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			title := strings.TrimSpace(args[1])
			if title == "" {
				return fmt.Errorf("title must not be empty")
			}
			item, err := store.SetTitle(args[0], title)
			if err != nil {
				return err
			}
			fmt.Printf("Renamed item successfully: %s\n", store.ShortID(item.ID))
			return nil
		},
	}
}
