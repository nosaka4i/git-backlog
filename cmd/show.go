package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show an item's full state and op-log history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			item, err := store.LoadItem(args[0])
			if err != nil {
				return err
			}
			var syncState *store.ItemSyncState
			if remote, err := store.ResolveRemote(""); err == nil {
				s, err := store.SyncState(item.ID, remote)
				if err != nil {
					return err
				}
				syncState = &s
			}
			if jsonFlag {
				history, err := store.History(item.ID)
				if err != nil {
					return err
				}
				// store.History is oldest-first; reverse so the JSON
				// history array matches the human output's newest-first
				// order (toJSONHistory itself stays order-preserving —
				// see its own test).
				newestFirst := make([]store.OpRecord, len(history))
				for i, op := range history {
					newestFirst[len(history)-1-i] = op
				}
				jitem := toJSONItem(item)
				jitem.Sync = toJSONSyncState(syncState)
				detail := jsonItemDetail{
					jsonItem: jitem,
					Tip:      item.Tip,
					History:  toJSONHistory(newestFirst),
				}
				return printJSON(detail)
			}
			priority := "unset"
			if item.Priority != store.PriorityNone {
				priority = string(item.Priority)
			}
			fmt.Printf("id:          %s\n", store.ShortID(item.ID))
			fmt.Printf("title:       %s\n", item.Title)
			description := "(none)"
			if item.Description != "" {
				description = item.Description
			}
			fmt.Printf("description: %s\n", description)
			fmt.Printf("track:       %s\n", item.Track)
			fmt.Printf("priority:    %s\n", priority)
			comment := "(none)"
			if item.Comment != "" {
				comment = item.Comment
			}
			fmt.Printf("comment:     %s\n", comment)
			fmt.Printf("owner:       %s <%s>\n", item.OwnerName, item.OwnerEmail)
			fmt.Printf("created:     %s\n", item.CreatedAt.Format("2006-01-02 15:04:05 -0700"))
			fmt.Printf("updated:     %s\n", item.UpdatedAt.Format("2006-01-02 15:04:05 -0700"))
			fmt.Printf("sync:        %s\n", syncLine(syncState))
			fmt.Println()
			fmt.Println("history:")
			history, err := store.History(item.ID)
			if err != nil {
				return err
			}
			// store.History is oldest-first; print newest-first here to
			// match the `history` command and `comment show`, rather than
			// the reverse order this used to (inconsistently) print in.
			for i := len(history) - 1; i >= 0; i-- {
				op := history[i]
				fmt.Printf("  %s  %s  %s <%s>\n",
					op.When.Format("2006-01-02 15:04:05 -0700"), op.Commit[:12], op.AuthorName, op.AuthorEmail)
				for _, line := range opActionLines(op) {
					fmt.Printf("      %s\n", line)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	return cmd
}
