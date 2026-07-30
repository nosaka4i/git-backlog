// Command git-backlog is a git-native CLI for a small backlog: state is
// stored as real git objects under refs/backlog/*, synced via git
// push/fetch. See docs/design/git-backlog.md.
package main

import (
	"fmt"
	"os"

	"github.com/nosaka4i/git-backlog/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "git-backlog:", err)
		os.Exit(1)
	}
}
