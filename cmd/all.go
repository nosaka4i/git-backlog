package cmd

import (
	"fmt"
	"sort"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newAllCmd() *cobra.Command {
	var listFlag, priorityFlag string
	cmd := &cobra.Command{
		Use:   "all",
		Short: "List all backlog items, grouped by list then priority",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			items, err := store.AllItems()
			if err != nil {
				return err
			}
			displayLists := []store.List{store.ListBacklog, store.ListCurrent, store.ListClosed}
			if listFlag != "" {
				l, err := store.ParseList(listFlag)
				if err != nil {
					return err
				}
				items = filterList(items, l)
				displayLists = []store.List{l}
			}
			if priorityFlag != "" {
				p, err := store.ParsePriority(priorityFlag)
				if err != nil {
					return err
				}
				items = filterPriority(items, p)
			}
			sort.SliceStable(items, func(i, j int) bool {
				if items[i].List.Rank() != items[j].List.Rank() {
					return items[i].List.Rank() < items[j].List.Rank()
				}
				if items[i].Priority.Rank() != items[j].Priority.Rank() {
					return items[i].Priority.Rank() < items[j].Priority.Rank()
				}
				return items[i].CreatedAt.Before(items[j].CreatedAt)
			})
			byList := make(map[store.List][]*store.Item)
			for _, it := range items {
				byList[it.List] = append(byList[it.List], it)
			}
			for _, l := range displayLists {
				fmt.Println(string(l) + ":")
				group := byList[l]
				if len(group) == 0 {
					fmt.Println("  (empty)")
					continue
				}
				lastPriority := -1
				for _, it := range group {
					if it.Priority.Rank() != lastPriority {
						lastPriority = it.Priority.Rank()
						fmt.Println("  " + groupLabel(it.Priority))
					}
					fmt.Printf("    %s  %s\n", store.ShortID(it.ID), truncate(it.Title, 60))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&listFlag, "list", "", "filter by backlog|current|closed")
	cmd.Flags().StringVar(&priorityFlag, "priority", "", "filter by p0|p1|p2|none")
	return cmd
}

func groupLabel(p store.Priority) string {
	if p == store.PriorityNone {
		return "unprioritized:"
	}
	return string(p) + ":"
}

func filterList(items []*store.Item, l store.List) []*store.Item {
	out := make([]*store.Item, 0, len(items))
	for _, it := range items {
		if it.List == l {
			out = append(out, it)
		}
	}
	return out
}

func filterPriority(items []*store.Item, p store.Priority) []*store.Item {
	out := make([]*store.Item, 0, len(items))
	for _, it := range items {
		if it.Priority == p {
			out = append(out, it)
		}
	}
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
