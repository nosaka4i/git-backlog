package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Start tracking backlog items in this repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !gitx.InRepo() {
				return fmt.Errorf("not a git repository")
			}
			if _, ok := gitx.Config("backlog.init"); ok {
				fmt.Println("backlog already initialized")
				return nil
			}
			if err := gitx.SetConfig("backlog.init", "true"); err != nil {
				return err
			}
			fmt.Println("initialized backlog (refs/backlog/*)")
			return nil
		},
	}
}
