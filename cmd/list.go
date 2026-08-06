package cmd

import (
	"fmt"
	"sort"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var trackFlag, priorityFlag string
	var jsonFlag bool
	var closedLimit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all backlog items, grouped by track then priority",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			items, err := store.AllItems()
			if err != nil {
				return err
			}
			displayTracks := []store.Track{store.TrackCurrent, store.TrackBacklog, store.TrackClosed}
			explicitTrack := trackFlag != ""
			if explicitTrack {
				b, err := store.ParseTrack(trackFlag)
				if err != nil {
					return err
				}
				items = filterTrack(items, b)
				displayTracks = []store.Track{b}
			}
			if priorityFlag != "" {
				p, err := store.ParsePriority(priorityFlag)
				if err != nil {
					return err
				}
				items = filterPriority(items, p)
			}

			byTrack := make(map[store.Track][]*store.Item)
			for _, it := range items {
				byTrack[it.Track] = append(byTrack[it.Track], it)
			}
			totalByTrack := make(map[store.Track]int, len(byTrack))
			for b, group := range byTrack {
				totalByTrack[b] = len(group)
			}

			// Closed items accumulate forever in a successful backlog; cap
			// them to the N most recently touched unless the caller asked
			// specifically for the closed track (--track closed), in
			// which case they get everything.
			omitted := 0
			if !explicitTrack && closedLimit > 0 {
				if group := byTrack[store.TrackClosed]; len(group) > closedLimit {
					sort.SliceStable(group, func(i, j int) bool {
						return group[i].UpdatedAt.After(group[j].UpdatedAt)
					})
					omitted = len(group) - closedLimit
					byTrack[store.TrackClosed] = group[:closedLimit]
				}
			}

			for _, group := range byTrack {
				sort.SliceStable(group, func(i, j int) bool {
					if group[i].Priority.Rank() != group[j].Priority.Rank() {
						return group[i].Priority.Rank() < group[j].Priority.Rank()
					}
					return group[i].UpdatedAt.After(group[j].UpdatedAt)
				})
			}

			syncStates := make(map[string]*store.ItemSyncState)
			syncRemote, remoteErr := store.ResolveRemote("")
			if remoteErr == nil {
				for _, it := range items {
					s, err := store.SyncState(it.ID, syncRemote)
					if err != nil {
						return err
					}
					syncStates[it.ID] = &s
				}
			}

			if jsonFlag {
				out := make([]jsonItem, 0, len(items))
				for _, b := range displayTracks {
					for _, it := range byTrack[b] {
						jitem := toJSONItem(it)
						jitem.Sync = toJSONSyncState(syncStates[it.ID])
						out = append(out, jitem)
					}
				}
				return printJSON(out)
			}

			if remoteErr == nil {
				fmt.Println(syncSummaryLine(syncRemote, syncStates))
				fmt.Println()
			}

			for _, b := range displayTracks {
				fmt.Printf("%s (%d):\n", b, totalByTrack[b])
				group := byTrack[b]
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
					fmt.Printf("    %s  %s%s\n", store.ShortID(it.ID), truncate(it.Title, 60), syncMarker(syncStates[it.ID]))
				}
				if b == store.TrackClosed && omitted > 0 {
					fmt.Printf("  ... and %d more (use --closed-limit 0 to show all, or --track closed)\n", omitted)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&trackFlag, "track", "", "filter by backlog|current|closed")
	cmd.Flags().StringVar(&priorityFlag, "priority", "", "filter by p0|p1|p2|none")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.Flags().IntVar(&closedLimit, "closed-limit", 10,
		"cap closed items to the N most recently updated (0 = unlimited); ignored when --track is set")
	return cmd
}

func groupLabel(p store.Priority) string {
	if p == store.PriorityNone {
		return "unprioritized:"
	}
	return string(p) + ":"
}

func filterTrack(items []*store.Item, b store.Track) []*store.Item {
	out := make([]*store.Item, 0, len(items))
	for _, it := range items {
		if it.Track == b {
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
