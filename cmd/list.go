package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

// dashboardCap is how many items each track shows in the default (no
// --track) list view. Chosen so the whole dashboard -- sync line, last-24h
// line, and all three tracks -- reliably fits inside a 24-row terminal
// regardless of how items are spread across priorities: a fixed 3 items
// per track, no priority sub-headers, keeps each track's line count
// predictable instead of ballooning with however many priority groups its
// top items happen to span. Asking for a track by name (--track) bypasses
// this entirely and shows everything, paged, same as it always has.
const dashboardCap = 3

func newListCmd() *cobra.Command {
	var trackFlag, priorityFlag string
	var labelFlag []string
	var jsonFlag, noPagerFlag bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List backlog items: a capped dashboard by default, or a full track with --track",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			allItems, skipped, err := store.AllItems()
			if err != nil {
				return err
			}
			warnSkippedItems(skipped)
			items := allItems
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
			priorities, err := parsePriorityFilter(priorityFlag)
			if err != nil {
				return err
			}
			if len(priorities) > 0 {
				items = filterPriorities(items, priorities)
			}
			if len(labelFlag) > 0 {
				items = filterLabels(items, labelFlag)
			}

			byTrack := make(map[store.Track][]*store.Item)
			for _, it := range items {
				byTrack[it.Track] = append(byTrack[it.Track], it)
			}
			for _, group := range byTrack {
				sort.SliceStable(group, func(i, j int) bool {
					if group[i].Priority.Rank() != group[j].Priority.Rank() {
						return group[i].Priority.Rank() < group[j].Priority.Rank()
					}
					return group[i].UpdatedAt.After(group[j].UpdatedAt)
				})
			}
			totalByTrack := make(map[store.Track]int, len(byTrack))
			for b, group := range byTrack {
				totalByTrack[b] = len(group)
			}

			// The dashboard cap only kicks in when the caller didn't name a
			// track explicitly -- same rule `closed` alone used to follow
			// (via the now-retired --closed-limit), extended to every
			// track. shown holds what actually gets printed/JSON'd; the
			// uncapped byTrack groups stay around for the per-track
			// priority-breakdown counts.
			shownByTrack := byTrack
			if !explicitTrack {
				shownByTrack = make(map[store.Track][]*store.Item, len(byTrack))
				for b, group := range byTrack {
					if len(group) > dashboardCap {
						shownByTrack[b] = group[:dashboardCap]
					} else {
						shownByTrack[b] = group
					}
				}
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
					for _, it := range shownByTrack[b] {
						jitem := toJSONItem(it)
						jitem.Sync = toJSONSyncState(syncStates[it.ID])
						out = append(out, jitem)
					}
				}
				return printJSON(out)
			}

			w, done := pagerWriter(noPagerFlag)
			defer done()

			if remoteErr == nil {
				fmt.Fprintf(w, "sync: %s\n", syncSummaryLine(syncRemote, syncStates))
			}
			added, closed, err := last24h(allItems)
			if err != nil {
				return err
			}
			fmt.Fprintln(w, last24hLine(added, closed))
			fmt.Fprintln(w)

			if explicitTrack {
				writeTrackFull(w, displayTracks[0], byTrack[displayTracks[0]], syncStates)
			} else {
				for _, b := range displayTracks {
					writeTrackDashboard(w, b, byTrack[b], shownByTrack[b], syncStates)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&trackFlag, "track", "", "show one track in full, uncapped, paged: backlog|current|closed")
	cmd.Flags().StringVar(&priorityFlag, "priority", "", "filter to a comma-separated list of p0,p1,p2,none")
	cmd.Flags().StringArrayVar(&labelFlag, "label", nil, "filter to items carrying all of these labels (repeatable)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&noPagerFlag, "no-pager", false, "don't pipe output through a pager, even on a terminal")
	return cmd
}

// writeTrackDashboard renders one track for the default (no --track) view:
// "<track> (<N> total[, <breakdown>]):", then up to dashboardCap items with
// priority inlined per line instead of grouped under sub-headers, so a
// track's line count never depends on how spread out its priorities are.
// The breakdown is left off when everything already fits (nothing to
// summarize beyond what's already on screen).
func writeTrackDashboard(w io.Writer, b store.Track, full, shown []*store.Item, syncStates map[string]*store.ItemSyncState) {
	total := len(full)
	if total > dashboardCap {
		fmt.Fprintf(w, "%s (%d total, %s):\n", b, total, priorityCounts(full))
	} else {
		fmt.Fprintf(w, "%s (%d total):\n", b, total)
	}
	fmt.Fprintln(w)
	if len(shown) == 0 {
		fmt.Fprintln(w, "  (empty)")
	} else {
		for _, it := range shown {
			fmt.Fprintf(w, "  %s %s  %s%s%s\n", priorityTag(it.Priority), store.ShortID(it.ID),
				truncate(it.Title, 60), labelSuffix(it.Labels), syncMarker(syncStates[it.ID]))
		}
		if total > dashboardCap {
			fmt.Fprintf(w, "  ... and %d more (use --track %s to see them all)\n", total-dashboardCap, b)
		}
	}
	fmt.Fprintln(w)
}

// writeTrackFull renders one track exactly as `list` has always rendered
// it: every item, grouped under a priority sub-header, no cap. This is
// what an explicit --track opts into -- a deliberate, narrow request gets
// the real thing, same as --track closed always has.
func writeTrackFull(w io.Writer, b store.Track, group []*store.Item, syncStates map[string]*store.ItemSyncState) {
	fmt.Fprintf(w, "%s (%d):\n", b, len(group))
	if len(group) == 0 {
		fmt.Fprintln(w, "  (empty)")
		return
	}
	lastPriority := -1
	for _, it := range group {
		if it.Priority.Rank() != lastPriority {
			lastPriority = it.Priority.Rank()
			fmt.Fprintln(w, "  "+groupLabel(it.Priority))
		}
		fmt.Fprintf(w, "    %s  %s%s%s\n", store.ShortID(it.ID), truncate(it.Title, 60),
			labelSuffix(it.Labels), syncMarker(syncStates[it.ID]))
	}
}

// priorityTag renders an item's priority as the dashboard's leading
// column: "p0"/"p1"/"p2", or "none" for PriorityNone (which is "" on the
// wire and would otherwise print as nothing).
func priorityTag(p store.Priority) string {
	if p == store.PriorityNone {
		return "none"
	}
	return string(p)
}

// priorityCounts renders a track's full (uncapped) priority breakdown,
// npm-audit style: count then label, comma-separated, every tier shown
// even at zero so a reader never has to wonder whether p0 was omitted or
// just empty.
func priorityCounts(items []*store.Item) string {
	var p0, p1, p2, none int
	for _, it := range items {
		switch it.Priority {
		case store.PriorityP0:
			p0++
		case store.PriorityP1:
			p1++
		case store.PriorityP2:
			p2++
		default:
			none++
		}
	}
	return fmt.Sprintf("%d p0, %d p1, %d p2, %d none", p0, p1, p2, none)
}

// last24h counts, across every item's op-log regardless of any filter the
// current `list` call applied, how many were created and how many were
// moved to closed in the trailing 24 hours -- a rolling window, not a
// calendar day, since a calendar day is meaningless across time zones.
// Renames/priority/label/comment edits are deliberately not counted here:
// unlike "created" or "closed", they have no unambiguous meaning in
// aggregate (10 priority changes could mean anything).
func last24h(items []*store.Item) (added, closed int, err error) {
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, it := range items {
		history, err := store.History(it.ID)
		if err != nil {
			return 0, 0, err
		}
		for _, op := range history {
			if op.When.Before(cutoff) {
				continue
			}
			if isCreateOp(op) {
				added++
				continue
			}
			for _, ch := range op.Changes {
				if (ch.Field == "track" || ch.Field == "list") && !ch.Removed && ch.Value == string(store.TrackClosed) {
					closed++
				}
			}
		}
	}
	return added, closed, nil
}

func last24hLine(added, closed int) string {
	if added == 0 && closed == 0 {
		return "last 24h: none"
	}
	return fmt.Sprintf("last 24h: %d new, %d closed", added, closed)
}

// parsePriorityFilter splits a comma-separated --priority value into its
// individual priorities ("p0,p1" -> [p0, p1]), validating each the same
// way a single-valued --priority always has. "" (flag not given) returns
// nil, nil -- no filter.
func parsePriorityFilter(raw string) ([]store.Priority, error) {
	if raw == "" {
		return nil, nil
	}
	var out []store.Priority
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		p, err := store.ParsePriority(tok)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// filterPriorities keeps items whose priority matches any one of the
// wanted values (OR semantics -- "p0,p1" means "p0 or p1", unlike
// --label's AND semantics across multiple labels).
func filterPriorities(items []*store.Item, priorities []store.Priority) []*store.Item {
	want := make(map[store.Priority]struct{}, len(priorities))
	for _, p := range priorities {
		want[p] = struct{}{}
	}
	out := make([]*store.Item, 0, len(items))
	for _, it := range items {
		if _, ok := want[it.Priority]; ok {
			out = append(out, it)
		}
	}
	return out
}

// filterLabels keeps only items carrying every one of the wanted labels
// (AND semantics, matching how `gh issue list --label` narrows).
func filterLabels(items []*store.Item, want []string) []*store.Item {
	out := make([]*store.Item, 0, len(items))
	for _, it := range items {
		has := make(map[string]struct{}, len(it.Labels))
		for _, l := range it.Labels {
			has[l] = struct{}{}
		}
		ok := true
		for _, w := range want {
			if _, found := has[strings.TrimSpace(w)]; !found {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, it)
		}
	}
	return out
}

// labelSuffix renders an item's labels as a compact inline tag list for the
// list view, or "" when it has none.
func labelSuffix(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return "  [" + strings.Join(labels, ", ") + "]"
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

// warnSkippedItems prints one line to stderr per item AllItems() couldn't
// load, so a corrupt item is still visible instead of just silently missing
// from the count — shared by list/history/label, every AllItems() caller.
// Goes to stderr specifically so it never lands inside --json/paged output.
func warnSkippedItems(skipped []store.ItemLoadError) {
	for _, s := range skipped {
		fmt.Fprintf(os.Stderr, "warning: skipped item %s (failed to load): %s\n", store.ShortID(s.ID), s.Err)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
