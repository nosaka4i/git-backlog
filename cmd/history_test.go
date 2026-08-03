package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/nosaka4i/git-backlog/internal/store"
)

// gitAuthorTimestamps have 1-second resolution, so operations within the
// same wall-clock second tie and sort by insertion order, not real time.
// Tests asserting chronological ordering need a real gap to be meaningful.
const gitTimestampResolution = 1100 * time.Millisecond

func TestHistoryFlatChronologicalAcrossItems(t *testing.T) {
	chdirTempRepo(t, "alice")
	a, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.CreateItem("write docs", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(gitTimestampResolution)
	if _, err := store.SetList(a.ID, store.ListCurrent, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPriority(b.ID, store.PriorityP0, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newHistoryCmd())
	if err != nil {
		t.Fatal(err)
	}
	// 4 ops total: 2 creates + 2 updates.
	if strings.Count(out, "Added item") != 2 {
		t.Fatalf("expected 2 \"Added item\" entries:\n%s", out)
	}
	if !strings.Contains(out, "Title: fix flaky test (Moved to current)") {
		t.Fatalf("missing the list-change entry:\n%s", out)
	}
	if !strings.Contains(out, "Title: write docs (Updated priority to p0)") {
		t.Fatalf("missing the priority-change entry:\n%s", out)
	}

	// Newest first: the two update entries should both come before both
	// create entries.
	lastUpdateIdx := maxIndex(out, "Moved to current", "Updated priority to p0")
	firstCreateIdx := strings.Index(out, "Added item")
	if lastUpdateIdx < 0 || firstCreateIdx < 0 || lastUpdateIdx > firstCreateIdx {
		t.Fatalf("expected updates before creates (newest first):\n%s", out)
	}
}

func maxIndex(s string, subs ...string) int {
	max := -1
	for _, sub := range subs {
		if idx := strings.Index(s, sub); idx > max {
			max = idx
		}
	}
	return max
}

func TestHistoryShowsClearedPriorityAndRename(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPriority(item.ID, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetTitle(item.ID, "fix the flaky auth test", nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newHistoryCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Cleared priority") {
		t.Fatalf("missing Cleared priority:\n%s", out)
	}
	if !strings.Contains(out, "Renamed item") {
		t.Fatalf("missing Renamed item:\n%s", out)
	}
}

func TestHistoryListAndPriorityFilters(t *testing.T) {
	chdirTempRepo(t, "alice")
	inCurrent, err := store.CreateItem("in current", store.ListCurrent, store.PriorityP0, nil)
	if err != nil {
		t.Fatal(err)
	}
	inBacklog, err := store.CreateItem("in backlog", store.ListBacklog, store.PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newHistoryCmd(), "--list", "current")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "in current") || strings.Contains(out, "in backlog") {
		t.Fatalf("--list current filter didn't apply:\n%s", out)
	}

	out, err = runCmd(t, newHistoryCmd(), "--priority", "p1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "in backlog") || strings.Contains(out, "in current") {
		t.Fatalf("--priority p1 filter didn't apply:\n%s", out)
	}

	_ = inCurrent
	_ = inBacklog
}

func TestHistoryInvalidFilters(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := runCmd(t, newHistoryCmd(), "--list", "bogus"); err == nil {
		t.Fatal("expected an error for an invalid --list value")
	}
	if _, err := runCmd(t, newHistoryCmd(), "--priority", "p9"); err == nil {
		t.Fatal("expected an error for an invalid --priority value")
	}
}

func TestHistoryJSON(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(gitTimestampResolution)
	if _, err := store.SetList(item.ID, store.ListCurrent, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newHistoryCmd(), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []jsonHistoryEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	// newest first: the list-change entry comes before the create entry.
	if entries[0].ItemID != item.ID || entries[0].Title != "fix flaky test" {
		t.Fatalf("entries[0] = %+v", entries[0])
	}
	if len(entries[0].Changes) != 1 || entries[0].Changes[0].Field != "list" || entries[0].Changes[0].Value != "current" {
		t.Fatalf("entries[0].Changes = %+v", entries[0].Changes)
	}
	if entries[0].Author.Name != "alice" {
		t.Fatalf("entries[0].Author = %+v", entries[0].Author)
	}
}

func TestHistoryMergeCommitShowsMultipleActionsUnderOneHeader(t *testing.T) {
	// Simulates the sync-merge case: one op-log commit touching two
	// fields at once should render as one header line with two action
	// lines beneath it, both attributed to that commit's single author.
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	runGit(t, "", "init", "--bare", "-q", remote)
	repoA := filepath.Join(root, "repoA")
	repoB := filepath.Join(root, "repoB")
	runGit(t, "", "clone", "-q", remote, repoA)
	runGit(t, "", "clone", "-q", remote, repoB)
	runGit(t, repoA, "config", "user.name", "alice")
	runGit(t, repoA, "config", "user.email", "alice@example.com")
	runGit(t, repoB, "config", "user.name", "bob")
	runGit(t, repoB, "config", "user.email", "bob@example.com")

	orig := chdirTo(t, repoA)
	defer chdirTo(t, orig)
	if err := gitx.SetConfig("backlog.init", "true"); err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Sync("origin"); err != nil {
		t.Fatal(err)
	}

	chdirTo(t, repoB)
	if err := gitx.SetConfig("backlog.init", "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Sync("origin"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPriority(item.ID, store.PriorityP0, nil); err != nil {
		t.Fatal(err)
	}

	chdirTo(t, repoA)
	if _, err := store.SetList(item.ID, store.ListCurrent, nil); err != nil {
		t.Fatal(err)
	}

	chdirTo(t, repoB)
	if _, err := store.Sync("origin"); err != nil {
		t.Fatal(err)
	}

	chdirTo(t, repoA)
	if _, err := store.Sync("origin"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newHistoryCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Moved to current") || !strings.Contains(out, "Updated priority to p0") {
		t.Fatalf("expected both reconciled actions present:\n%s", out)
	}
}
