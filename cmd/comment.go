package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newCommentCmd() *cobra.Command {
	var asAgent bool
	cmd := &cobra.Command{
		Use:   "comment <id> <text>",
		Short: "Set or clear an item's comment",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			identity, err := resolveAgentIdentity(asAgent)
			if err != nil {
				return err
			}
			item, err := store.SetComment(args[0], args[1], identity)
			if err != nil {
				return err
			}
			fmt.Printf("Updated item comment successfully: %s\n", store.ShortID(item.ID))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asAgent, "as-agent", false, asAgentFlagUsage)
	cmd.AddCommand(newCommentShowCmd())
	return cmd
}

func newCommentShowCmd() *cobra.Command {
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show an item's comments, newest first",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			item, err := store.LoadItem(args[0])
			if err != nil {
				return err
			}
			history, err := store.History(item.ID)
			if err != nil {
				return err
			}
			entries := commentEntries(history)

			if jsonFlag {
				out := make([]jsonCommentEntry, 0, len(entries))
				for _, e := range entries {
					out = append(out, jsonCommentEntry{
						When:    e.op.When,
						Commit:  e.op.Commit,
						Author:  jsonOwner{Name: e.op.AuthorName, Email: e.op.AuthorEmail},
						Text:    e.change.Value,
						Cleared: e.change.Removed,
					})
				}
				return printJSON(out)
			}

			if len(entries) == 0 {
				fmt.Println("no comments")
				return nil
			}
			for _, e := range entries {
				fmt.Printf("  %s  %s  %s <%s>\n",
					e.op.When.Format("2006-01-02 15:04:05 -0700"), e.op.Commit[:12], e.op.AuthorName, e.op.AuthorEmail)
				if e.change.Removed {
					fmt.Println("      (cleared)")
					continue
				}
				fmt.Printf("      %s\n", e.change.Value)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	return cmd
}

type commentEntry struct {
	op     store.OpRecord
	change store.FieldChange
}

// commentEntries pulls just the comment-field changes out of an item's
// op-log, newest first (matching `history`'s convention, rather than
// `show`'s oldest-first per-item op-log).
func commentEntries(history []store.OpRecord) []commentEntry {
	var entries []commentEntry
	for i := len(history) - 1; i >= 0; i-- {
		op := history[i]
		for _, ch := range op.Changes {
			if ch.Field != "comment" {
				continue
			}
			entries = append(entries, commentEntry{op: op, change: ch})
		}
	}
	return entries
}
