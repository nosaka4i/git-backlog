package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var trackFlag, priorityFlag, grepFlag string
	var labelFlag []string
	var topFlag int
	var jsonFlag, noPagerFlag bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List backlog items: everything by default, or --top N per track for a capped view",
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
			if trackFlag != "" {
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
			if grepFlag != "" {
				items = filterGrep(items, grepFlag)
			}

			byTrack := make(map[store.Track][]*store.Item)
			for _, it := range items {
				byTrack[it.Track] = append(byTrack[it.Track], it)
			}
			for _, group := range byTrack {
				sort.SliceStable(group, func(i, j int) bool {
					if grepFlag != "" {
						ti, tj := grepTier(group[i], grepFlag), grepTier(group[j], grepFlag)
						if ti != tj {
							return ti < tj
						}
					}
					if group[i].Priority.Rank() != group[j].Priority.Rank() {
						return group[i].Priority.Rank() < group[j].Priority.Rank()
					}
					return group[i].UpdatedAt.After(group[j].UpdatedAt)
				})
			}

			// A cap only kicks in when --top is given: with no --top, list
			// shows every matching item, uncapped, paged -- same as
			// --track/--priority/--label always have. shown holds what
			// actually gets printed/JSON'd; the uncapped byTrack groups
			// stay around for the per-track priority-breakdown counts.
			capped := topFlag > 0
			shownByTrack := byTrack
			if capped {
				shownByTrack = make(map[store.Track][]*store.Item, len(byTrack))
				for b, group := range byTrack {
					if len(group) > topFlag {
						shownByTrack[b] = group[:topFlag]
					} else {
						shownByTrack[b] = group
					}
				}
			}

			showSync := false
			syncStates := make(map[string]*store.ItemSyncState)
			syncRemote, remoteErr := store.ResolveRemote("")
			if remoteErr == nil {
				showSync = true
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

			for _, b := range displayTracks {
				if capped {
					writeTrackDashboard(w, b, byTrack[b], shownByTrack[b], syncStates, topFlag, showSync)
				} else {
					writeTrackFull(w, b, byTrack[b], syncStates, showSync)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&trackFlag, "track", "", "limit to one track: backlog|current|closed")
	cmd.Flags().StringVar(&priorityFlag, "priority", "", "filter to a comma-separated list of p0,p1,p2,none")
	cmd.Flags().StringArrayVar(&labelFlag, "label", nil, "filter to items carrying all of these labels (repeatable)")
	cmd.Flags().StringVar(&grepFlag, "grep", "", "filter to items matching this text in title, description, or comment (case-insensitive); matches rank title first, then description, then comment")
	cmd.Flags().IntVar(&topFlag, "top", 0, "show only the top N items per track (by priority, then most recently updated); omit for the full uncapped list")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&noPagerFlag, "no-pager", false, "don't pipe output through a pager, even on a terminal")
	return cmd
}

// writeTrackDashboard renders one track for a capped (--top N) view:
// "<track> (<N> total[, <breakdown>]):", then an aligned table of up to N
// items. The breakdown is left off when everything already fits (nothing
// to summarize beyond what's already on screen).
func writeTrackDashboard(w io.Writer, b store.Track, full, shown []*store.Item, syncStates map[string]*store.ItemSyncState, top int, showSync bool) {
	total := len(full)
	if total > top {
		fmt.Fprintf(w, "%s (%d total, %s):\n", b, total, priorityCounts(full))
	} else {
		fmt.Fprintf(w, "%s (%d total):\n", b, total)
	}
	fmt.Fprintln(w)
	if len(shown) == 0 {
		fmt.Fprintln(w, "(empty)")
	} else {
		writeItemTable(w, shown, syncStates, showSync)
		if total > top {
			fmt.Fprintf(w, "... and %d more (raise or drop --top to see them all)\n", total-top)
		}
	}
	fmt.Fprintln(w)
}

// writeTrackFull renders one track with every item, no cap -- the default
// view, and what --track has always shown for a single track.
func writeTrackFull(w io.Writer, b store.Track, group []*store.Item, syncStates map[string]*store.ItemSyncState, showSync bool) {
	fmt.Fprintf(w, "%s (%d):\n", b, len(group))
	if len(group) == 0 {
		fmt.Fprintln(w, "(empty)")
	} else {
		writeItemTable(w, group, syncStates, showSync)
	}
	fmt.Fprintln(w)
}

// writeItemTable renders items as an aligned, headered table -- ID,
// PRIORITY, TITLE, LABELS, and (when a remote is configured) SYNC, one row
// per item, ollama-list/docker-ps/kubectl-get style. Columns are sized to
// their widest cell via text/tabwriter, so a value like "(not synced)"
// lands in a fixed column instead of trailing off at a different position
// on every row depending on how long that row's title happened to be.
// Priority rides along as its own column instead of a grouping sub-header,
// so every row has the same shape regardless of how spread out priorities
// are; the sort that put items in this order (priority, then most-recently-
// updated) already clusters same-priority rows together.
func writeItemTable(w io.Writer, items []*store.Item, syncStates map[string]*store.ItemSyncState, showSync bool) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if showSync {
		fmt.Fprintln(tw, "ID\tPRIORITY\tTITLE\tLABELS\tSYNC")
	} else {
		fmt.Fprintln(tw, "ID\tPRIORITY\tTITLE\tLABELS")
	}
	for _, it := range items {
		row := []string{store.ShortID(it.ID), priorityTag(it.Priority), truncate(it.Title, 60), strings.Join(it.Labels, ", ")}
		if showSync {
			row = append(row, strings.TrimSpace(syncMarker(syncStates[it.ID])))
		}
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
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

// grepTier reports where term (case-insensitive substring) matches an
// item, lowest first so a SliceStable sort by tier ranks the most
// relevant field first: 0 = title, 1 = description, 2 = comment, 3 = no
// match at all (filterGrep never lets that last case through, but the
// tier is still well-defined if grepTier is ever called on its own).
func grepTier(it *store.Item, term string) int {
	term = strings.ToLower(term)
	switch {
	case strings.Contains(strings.ToLower(it.Title), term):
		return 0
	case strings.Contains(strings.ToLower(it.Description), term):
		return 1
	case strings.Contains(strings.ToLower(it.Comment), term):
		return 2
	default:
		return 3
	}
}

// filterGrep keeps items whose title, description, or comment contains
// term (case-insensitive substring, not a regex -- kept simple and
// predictable rather than pattern matching like git log --grep).
func filterGrep(items []*store.Item, term string) []*store.Item {
	out := make([]*store.Item, 0, len(items))
	for _, it := range items {
		if grepTier(it, term) < 3 {
			out = append(out, it)
		}
	}
	return out
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
