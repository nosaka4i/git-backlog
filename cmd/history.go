package cmd

import (
	"fmt"
	"sort"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newHistoryCmd() *cobra.Command {
	var listFlag, priorityFlag string
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show a chronological activity trail across every item",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			items, err := store.AllItems()
			if err != nil {
				return err
			}
			if listFlag != "" {
				l, err := store.ParseList(listFlag)
				if err != nil {
					return err
				}
				items = filterList(items, l)
			}
			if priorityFlag != "" {
				p, err := store.ParsePriority(priorityFlag)
				if err != nil {
					return err
				}
				items = filterPriority(items, p)
			}

			entries, err := gatherHistory(items)
			if err != nil {
				return err
			}
			sort.SliceStable(entries, func(i, j int) bool {
				return entries[i].op.When.After(entries[j].op.When)
			})

			if jsonFlag {
				out := make([]jsonHistoryEntry, 0, len(entries))
				for _, e := range entries {
					out = append(out, jsonHistoryEntry{
						When:    e.op.When,
						Commit:  e.op.Commit,
						ItemID:  e.item.ID,
						Title:   e.item.Title,
						Author:  jsonOwner{Name: e.op.AuthorName, Email: e.op.AuthorEmail},
						Changes: toJSONChanges(e.op.Changes),
					})
				}
				return printJSON(out)
			}

			for _, e := range entries {
				fmt.Printf("%s  %s  %s <%s>\n",
					e.op.When.Format("2006-01-02 15:04:05 -0700"), e.op.Commit[:12], e.op.AuthorName, e.op.AuthorEmail)
				for _, line := range opActionLines(e.op) {
					fmt.Printf("    Title: %s (%s)\n", truncate(e.item.Title, 60), line)
				}
				fmt.Println()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&listFlag, "list", "", "filter to items currently in backlog|current|closed")
	cmd.Flags().StringVar(&priorityFlag, "priority", "", "filter to items currently at p0|p1|p2|none")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	return cmd
}

type historyEntry struct {
	item *store.Item
	op   store.OpRecord
}

func gatherHistory(items []*store.Item) ([]historyEntry, error) {
	var entries []historyEntry
	for _, it := range items {
		history, err := store.History(it.ID)
		if err != nil {
			return nil, err
		}
		for _, op := range history {
			entries = append(entries, historyEntry{item: it, op: op})
		}
	}
	return entries, nil
}
