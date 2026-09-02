package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/nosaka4i/git-backlog/internal/store"
)

// TestListShowsAlignedColumnHeaders confirms each track renders as a
// headered table (ID/PRIORITY/TITLE/LABELS, ollama-list/docker-ps style)
// with columns actually aligned via tabwriter, not just a header line
// slapped on top of differently-indented rows.
func TestListShowsAlignedColumnHeaders(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("short", store.TrackBacklog, store.PriorityP1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("a much longer title than the other one", store.TrackBacklog, store.PriorityP2, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out, "\n")
	var header string
	var rows []string
	for i, line := range lines {
		if strings.HasPrefix(line, "ID") {
			header = line
			rows = lines[i+1 : i+3]
			break
		}
	}
	if header == "" {
		t.Fatalf("expected an ID/PRIORITY/TITLE/LABELS header row:\n%s", out)
	}
	if !strings.Contains(header, "PRIORITY") || !strings.Contains(header, "TITLE") || !strings.Contains(header, "LABELS") {
		t.Fatalf("header missing an expected column:\n%q", header)
	}
	priorityCol := strings.Index(header, "PRIORITY")
	for _, row := range rows {
		if len(row) < priorityCol || !strings.HasPrefix(strings.TrimSpace(row[priorityCol:]), "p") {
			t.Fatalf("expected PRIORITY column at index %d to align with the header:\nheader: %q\nrow:    %q", priorityCol, header, row)
		}
	}
}

func TestListNoSyncColumnWithoutARemote(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("solo item", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "SYNC") {
		t.Fatalf("did not expect a SYNC column with no remote configured:\n%s", out)
	}
}

func TestListGroupsByTrackThenPriority(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("backlog p1 item", store.TrackBacklog, store.PriorityP1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("backlog unprioritized item", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("current p0 item", store.TrackCurrent, store.PriorityP0, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd())
	if err != nil {
		t.Fatal(err)
	}

	backlogIdx := strings.Index(out, "backlog (")
	currentIdx := strings.Index(out, "current (")
	closedIdx := strings.Index(out, "closed (")
	if backlogIdx < 0 || currentIdx < 0 || closedIdx < 0 {
		t.Fatalf("missing a list header:\n%s", out)
	}
	if !(currentIdx < backlogIdx && backlogIdx < closedIdx) {
		t.Fatalf("list headers out of order (want current, backlog, closed):\n%s", out)
	}
	if !strings.Contains(out, "current (1):") || !strings.Contains(out, "backlog (2):") || !strings.Contains(out, "closed (0):") {
		t.Fatalf("list headers missing expected counts:\n%s", out)
	}
	if !strings.Contains(out, "backlog p1 item") || !strings.Contains(out, "current p0 item") {
		t.Fatalf("missing expected items:\n%s", out)
	}
	// closed has no items at all -> shown but empty.
	closedSection := out[closedIdx:]
	if !strings.Contains(closedSection, "(empty)") {
		t.Fatalf("expected closed section to say (empty):\n%s", closedSection)
	}
}

// TestListSkipsCorruptItemInsteadOfAborting confirms `list` shows every
// good item instead of hard-erroring the moment ANY one item is broken —
// confirmed live 2026-08-30, one corrupt item used to take the whole
// command down and print nothing at all.
func TestListSkipsCorruptItemInsteadOfAborting(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("fine item", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}
	bad, err := store.CreateItem("about to be corrupted", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := gitx.MkTree(nil)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := gitx.CommitTree(tree, []string{bad.ID}, "corrupt: no fields at all")
	if err != nil {
		t.Fatal(err)
	}
	if err := gitx.UpdateRef(store.RefFor(bad.ID), commit, bad.ID); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd())
	if err != nil {
		t.Fatalf("list should not hard-error on one corrupt item: %v", err)
	}
	if !strings.Contains(out, "fine item") {
		t.Fatalf("expected the good item to still show up:\n%s", out)
	}
	if !strings.Contains(out, "warning: skipped item "+store.ShortID(bad.ID)) {
		t.Fatalf("expected a warning naming the skipped item:\n%s", out)
	}
}

func TestListLast24hCountsNewAndClosed(t *testing.T) {
	chdirTempRepo(t, "alice")
	kept, err := store.CreateItem("stays open", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	toClose, err := store.CreateItem("gets closed", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetTrack(toClose.ID, store.TrackClosed, nil); err != nil {
		t.Fatal(err)
	}
	_ = kept

	out, err := runCmd(t, newListCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "last 24h: 2 new, 1 closed") {
		t.Fatalf("expected a last-24h line counting both creates and the one close:\n%s", out)
	}
}

func TestListLast24hNoneWhenNothingHappened(t *testing.T) {
	chdirTempRepo(t, "alice")

	out, err := runCmd(t, newListCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "last 24h: none") {
		t.Fatalf("expected 'last 24h: none' on an empty backlog:\n%s", out)
	}
}

func TestListSortsMostRecentlyUpdatedFirstWithinGroup(t *testing.T) {
	chdirTempRepo(t, "alice")
	first, err := store.CreateItem("created first", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(gitTimestampResolution)
	if _, err := store.CreateItem("created second", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(gitTimestampResolution)
	// Touch the first (older) item so it's now the most recently updated,
	// even though it was created first.
	if _, err := store.SetTitle(first.ID, "created first, updated last", nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd())
	if err != nil {
		t.Fatal(err)
	}
	firstIdx := strings.Index(out, "created first, updated last")
	secondIdx := strings.Index(out, "created second")
	if firstIdx < 0 || secondIdx < 0 {
		t.Fatalf("missing expected items:\n%s", out)
	}
	if !(firstIdx < secondIdx) {
		t.Fatalf("expected the more-recently-updated item first despite being created earlier:\n%s", out)
	}
}

func TestListTrackFilterShowsOnlyThatTrack(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("in backlog", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("in current", store.TrackCurrent, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd(), "--track", "current")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "backlog (") || strings.Contains(out, "closed (") {
		t.Fatalf("expected only the current section:\n%s", out)
	}
	if !strings.Contains(out, "in current") || strings.Contains(out, "in backlog") {
		t.Fatalf("wrong items shown:\n%s", out)
	}
}

func TestListPriorityFilter(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("p0 item", store.TrackBacklog, store.PriorityP0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("p1 item", store.TrackBacklog, store.PriorityP1, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd(), "--priority", "p0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "p0 item") || strings.Contains(out, "p1 item") {
		t.Fatalf("priority filter didn't apply:\n%s", out)
	}
}

func TestListPriorityFilterCommaSeparatedIsOR(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("p0 item", store.TrackBacklog, store.PriorityP0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("p1 item", store.TrackBacklog, store.PriorityP1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("p2 item", store.TrackBacklog, store.PriorityP2, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd(), "--priority", "p0,p1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "p0 item") || !strings.Contains(out, "p1 item") {
		t.Fatalf("expected both p0 and p1 items with an OR filter:\n%s", out)
	}
	if strings.Contains(out, "p2 item") {
		t.Fatalf("did not expect p2 item with --priority p0,p1:\n%s", out)
	}
}

func TestListInvalidFiltersError(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := runCmd(t, newListCmd(), "--track", "bogus"); err == nil {
		t.Fatal("expected an error for an invalid --list value")
	}
	if _, err := runCmd(t, newListCmd(), "--priority", "p9"); err == nil {
		t.Fatal("expected an error for an invalid --priority value")
	}
}

func createClosedItems(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		item, err := store.CreateItem(fmt.Sprintf("closed item %d", i), store.TrackClosed, store.PriorityNone, nil)
		if err != nil {
			t.Fatal(err)
		}
		_ = item
	}
}

// TestListShowsEveryItemByDefault is the direct regression check for the
// bug --top exists to fix: a plain `list` with no flags used to cap each
// track at a fixed 3 items, so a just-added item with an unremarkable
// priority could be invisible on the very next `list` call. Confirmed
// live (2026-09-01) — reported as "I don't see the most recently added
// ones."
func TestListShowsEveryItemByDefault(t *testing.T) {
	chdirTempRepo(t, "alice")
	createClosedItems(t, 15)

	out, err := runCmd(t, newListCmd())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "closed item ") != 15 {
		t.Fatalf("expected all 15 closed items with no --top, got %d:\n%s", strings.Count(out, "closed item "), out)
	}
	if strings.Contains(out, "more (raise or drop --top") {
		t.Fatalf("did not expect a capped-view note with no --top:\n%s", out)
	}
}

func TestListTopCapsPerTrack(t *testing.T) {
	chdirTempRepo(t, "alice")
	createClosedItems(t, 15)

	out, err := runCmd(t, newListCmd(), "--top", "3")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "closed item ") != 3 {
		t.Fatalf("expected 3 closed items shown with --top 3, got %d:\n%s", strings.Count(out, "closed item "), out)
	}
	if !strings.Contains(out, "closed (15 total, 0 p0, 0 p1, 0 p2, 15 none):") {
		t.Fatalf("expected the header to show the true total and priority breakdown:\n%s", out)
	}
	if !strings.Contains(out, "... and 12 more (raise or drop --top to see them all)") {
		t.Fatalf("expected an omitted-count note pointing at --top:\n%s", out)
	}
}

func TestListTopSkipsBreakdownWhenTrackFits(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("only item", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd(), "--top", "3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "backlog (1 total):") {
		t.Fatalf("expected a plain total with no breakdown for a track at/under --top:\n%s", out)
	}
	if strings.Contains(out, "backlog (1 total, ") {
		t.Fatalf("did not expect a priority breakdown for a track at/under --top:\n%s", out)
	}
}

func TestListTrackAloneIsStillUncapped(t *testing.T) {
	chdirTempRepo(t, "alice")
	createClosedItems(t, 15)

	out, err := runCmd(t, newListCmd(), "--track", "closed")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "closed item ") != 15 {
		t.Fatalf("expected all 15 closed items with --track and no --top, got %d", strings.Count(out, "closed item "))
	}
}

func TestListTrackWithTopCapsJustThatTrack(t *testing.T) {
	chdirTempRepo(t, "alice")
	createClosedItems(t, 15)

	out, err := runCmd(t, newListCmd(), "--track", "closed", "--top", "5")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "closed item ") != 5 {
		t.Fatalf("expected --track closed --top 5 to cap at 5, got %d:\n%s", strings.Count(out, "closed item "), out)
	}
}

func TestListGrepFiltersAcrossTitleDescriptionComment(t *testing.T) {
	chdirTempRepo(t, "alice")
	titleMatch, err := store.CreateItem("fix the flaky race test", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = titleMatch
	descMatch, err := store.CreateItem("unrelated title", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetDescription(descMatch.ID, "this test is flaky under -race", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("no match at all", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd(), "--grep", "flaky")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "fix the flaky race test") {
		t.Fatalf("expected the title match to show up:\n%s", out)
	}
	if !strings.Contains(out, "unrelated title") {
		t.Fatalf("expected the description match to show up:\n%s", out)
	}
	if strings.Contains(out, "no match at all") {
		t.Fatalf("did not expect the non-matching item:\n%s", out)
	}
}

func TestListGrepIsCaseInsensitive(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("Fix The Flaky Race Test", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd(), "--grep", "flaky race")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Fix The Flaky Race Test") {
		t.Fatalf("expected a case-insensitive match:\n%s", out)
	}
}

// TestListGrepRanksTitleThenDescriptionThenComment confirms search
// results order by which field matched, most relevant first, regardless
// of the items' relative priority/recency.
func TestListGrepRanksTitleThenDescriptionThenComment(t *testing.T) {
	chdirTempRepo(t, "alice")
	commentMatch, err := store.CreateItem("z item", store.TrackBacklog, store.PriorityP0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetComment(commentMatch.ID, "noted: widget behavior", nil); err != nil {
		t.Fatal(err)
	}
	descMatch, err := store.CreateItem("a item", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetDescription(descMatch.ID, "the widget needs work", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("widget rollout", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd(), "--grep", "widget")
	if err != nil {
		t.Fatal(err)
	}
	titleIdx := strings.Index(out, "widget rollout")
	descIdx := strings.Index(out, "a item")
	commentIdx := strings.Index(out, "z item")
	if titleIdx < 0 || descIdx < 0 || commentIdx < 0 {
		t.Fatalf("missing expected items:\n%s", out)
	}
	if !(titleIdx < descIdx && descIdx < commentIdx) {
		t.Fatalf("expected title match, then description match, then comment match:\n%s", out)
	}
}

func TestListJSONOutput(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("unprioritized", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("prioritized", store.TrackBacklog, store.PriorityP2, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd(), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var items []jsonItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	for _, it := range items {
		if it.Title == "unprioritized" && it.Priority != "" {
			t.Fatalf("expected empty priority for unprioritized item, got %q", it.Priority)
		}
	}
}

func TestListJSONRespectsTop(t *testing.T) {
	chdirTempRepo(t, "alice")
	createClosedItems(t, 15)

	out, err := runCmd(t, newListCmd(), "--json", "--top", "3")
	if err != nil {
		t.Fatal(err)
	}
	var items []jsonItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3 (capped)", len(items))
	}
}
