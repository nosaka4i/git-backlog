package cmd

import (
	"fmt"
	"sort"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newAllCmd() *cobra.Command {
	var listFlag, priorityFlag string
	var jsonFlag bool
	var closedLimit int
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
			explicitList := listFlag != ""
			if explicitList {
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

			byList := make(map[store.List][]*store.Item)
			for _, it := range items {
				byList[it.List] = append(byList[it.List], it)
			}

			// Closed items accumulate forever in a successful backlog; cap
			// them to the N most recently touched unless the caller asked
			// specifically for the closed list (--list closed), in which
			// case they get everything.
			omitted := 0
			if !explicitList && closedLimit > 0 {
				if group := byList[store.ListClosed]; len(group) > closedLimit {
					sort.SliceStable(group, func(i, j int) bool {
						return group[i].UpdatedAt.After(group[j].UpdatedAt)
					})
					omitted = len(group) - closedLimit
					byList[store.ListClosed] = group[:closedLimit]
				}
			}

			for _, group := range byList {
				sort.SliceStable(group, func(i, j int) bool {
					if group[i].Priority.Rank() != group[j].Priority.Rank() {
						return group[i].Priority.Rank() < group[j].Priority.Rank()
					}
					return group[i].CreatedAt.Before(group[j].CreatedAt)
				})
			}

			if jsonFlag {
				out := make([]jsonItem, 0, len(items))
				for _, l := range displayLists {
					for _, it := range byList[l] {
						out = append(out, toJSONItem(it))
					}
				}
				return printJSON(out)
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
				if l == store.ListClosed && omitted > 0 {
					fmt.Printf("  ... and %d more (use --closed-limit 0 to show all, or --list closed)\n", omitted)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&listFlag, "list", "", "filter by backlog|current|closed")
	cmd.Flags().StringVar(&priorityFlag, "priority", "", "filter by p0|p1|p2|none")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.Flags().IntVar(&closedLimit, "closed-limit", 10,
		"cap closed items to the N most recently updated (0 = unlimited); ignored when --list is set")
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
