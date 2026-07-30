// Package cmd holds the git-backlog CLI's cobra commands.
package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "git-backlog",
	Short:         "A git-native backlog: what to work on next.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newAllCmd(),
		newShowCmd(),
		newListCmd(),
		newPriorityCmd(),
		newTitleCmd(),
		newSyncCmd(),
	)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// requireInit checks the repo has been set up with `git backlog init`.
func requireInit() error {
	if !gitx.InRepo() {
		return fmt.Errorf("not a git repository")
	}
	if _, ok := gitx.Config("backlog.init"); !ok {
		return fmt.Errorf("backlog not initialized in this repo; run `git backlog init` first")
	}
	return nil
}
