// Command git-backlog is a git-native CLI for a small backlog: state is
// stored as real git objects under refs/backlog/*, synced via git
// push/fetch. See docs/design/git-backlog.md.
package main

import (
	"fmt"
	"os"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "git-backlog",
		Short:         "A git-native backlog: what to work on next.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newAllCmd(),
		newShowCmd(),
		newListCmd(),
		newPriorityCmd(),
		newEditCmd(),
		newSyncCmd(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "git-backlog:", err)
		os.Exit(1)
	}
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
