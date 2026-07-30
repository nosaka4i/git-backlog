package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is set at release-build time via
// -ldflags "-X github.com/nosaka4i/git-backlog/cmd.version=vX.Y.Z". Left as
// "dev" for `go build`/`go run`; `go install pkg@vX.Y.Z` fills in the
// module version automatically via runtime/debug instead.
var version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the git-backlog version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(resolveVersion())
			return nil
		},
	}
}

func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
