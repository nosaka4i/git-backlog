package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestAllGroupsByListThenPriority(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("backlog p1 item", store.ListBacklog, store.PriorityP1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("backlog unprioritized item", store.ListBacklog, store.PriorityNone); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("current p0 item", store.ListCurrent, store.PriorityP0); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newAllCmd())
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

func TestAllListFilterShowsOnlyThatList(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("in backlog", store.ListBacklog, store.PriorityNone); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("in current", store.ListCurrent, store.PriorityNone); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newAllCmd(), "--list", "current")
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

func TestAllPriorityFilter(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("p0 item", store.ListBacklog, store.PriorityP0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("p1 item", store.ListBacklog, store.PriorityP1); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newAllCmd(), "--priority", "p0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "p0 item") || strings.Contains(out, "p1 item") {
		t.Fatalf("priority filter didn't apply:\n%s", out)
	}
}

func TestAllInvalidFiltersError(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := runCmd(t, newAllCmd(), "--list", "bogus"); err == nil {
		t.Fatal("expected an error for an invalid --list value")
	}
	if _, err := runCmd(t, newAllCmd(), "--priority", "p9"); err == nil {
		t.Fatal("expected an error for an invalid --priority value")
	}
}

func createClosedItems(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		item, err := store.CreateItem(fmt.Sprintf("closed item %d", i), store.ListClosed, store.PriorityNone)
		if err != nil {
			t.Fatal(err)
		}
		_ = item
	}
}

func TestAllCapsClosedByDefault(t *testing.T) {
	chdirTempRepo(t, "alice")
	createClosedItems(t, 15)

	out, err := runCmd(t, newAllCmd())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "closed item ") != 10 {
		t.Fatalf("expected 10 closed items shown by default, got %d:\n%s", strings.Count(out, "closed item "), out)
	}
	if !strings.Contains(out, "closed (15):") {
		t.Fatalf("expected the header count to show the true total (15), not the capped count:\n%s", out)
	}
	if !strings.Contains(out, "... and 5 more") {
		t.Fatalf("expected an omitted-count note:\n%s", out)
	}
}

func TestAllClosedLimitZeroIsUnlimited(t *testing.T) {
	chdirTempRepo(t, "alice")
	createClosedItems(t, 15)

	out, err := runCmd(t, newAllCmd(), "--closed-limit", "0")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "closed item ") != 15 {
		t.Fatalf("expected all 15 closed items, got %d", strings.Count(out, "closed item "))
	}
	if strings.Contains(out, "more") {
		t.Fatalf("did not expect an omitted-count note:\n%s", out)
	}
}

func TestAllListClosedBypassesCap(t *testing.T) {
	chdirTempRepo(t, "alice")
	createClosedItems(t, 15)

	out, err := runCmd(t, newAllCmd(), "--list", "closed")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "closed item ") != 15 {
		t.Fatalf("expected all 15 closed items when --list closed is explicit, got %d", strings.Count(out, "closed item "))
	}
}

func TestAllJSONOutput(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("unprioritized", store.ListBacklog, store.PriorityNone); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("prioritized", store.ListBacklog, store.PriorityP2); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newAllCmd(), "--json")
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

func TestAllJSONRespectsClosedCap(t *testing.T) {
	chdirTempRepo(t, "alice")
	createClosedItems(t, 15)

	out, err := runCmd(t, newAllCmd(), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var items []jsonItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(items) != 10 {
		t.Fatalf("got %d items, want 10 (capped)", len(items))
	}
}
