package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestShowSyncLineNoRemoteConfigured(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sync:        no remote configured") {
		t.Fatalf("output missing sync line:\n%s", out)
	}
}

func TestAllHasNoSummaryOrMarkersWithoutARemote(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newAllCmd())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "up to date with") || strings.Contains(out, "ahead") || strings.Contains(out, "behind") {
		t.Fatalf("expected no sync summary without a remote:\n%s", out)
	}
}

// syncedRepoPair sets up a bare remote plus two clones, both git-backlog
// initialized, for exercising all/show's sync-state rendering against a
// real remote.
type syncedRepoPair struct {
	repoA, repoB string
}

func newSyncedRepoPair(t *testing.T) syncedRepoPair {
	t.Helper()
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
	return syncedRepoPair{repoA: repoA, repoB: repoB}
}

func TestAllShowsUpToDateSummaryAfterSync(t *testing.T) {
	fx := newSyncedRepoPair(t)
	orig := chdirTo(t, fx.repoA)
	defer chdirTo(t, orig)
	if err := gitx.SetConfig("backlog.init", "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newSyncCmd()); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newAllCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "up to date with origin\n\n") {
		t.Fatalf("output = %q, want it to start with the up-to-date summary", out)
	}
}

func TestAllMarksAheadItemAndJSONIncludesSyncStatus(t *testing.T) {
	fx := newSyncedRepoPair(t)
	orig := chdirTo(t, fx.repoA)
	defer chdirTo(t, orig)
	if err := gitx.SetConfig("backlog.init", "true"); err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newSyncCmd()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPriority(item.ID, store.PriorityP1, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newAllCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1 ahead") {
		t.Fatalf("expected the summary to mention 1 ahead:\n%s", out)
	}
	if !strings.Contains(out, "fix flaky test  ↑1") {
		t.Fatalf("expected the item's line to have the ↑1 marker:\n%s", out)
	}

	jsonOut, err := runCmd(t, newAllCmd(), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var items []jsonItem
	if err := json.Unmarshal([]byte(jsonOut), &items); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, jsonOut)
	}
	if len(items) != 1 || items[0].Sync == nil || items[0].Sync.Status != "ahead" || items[0].Sync.AheadBy != 1 {
		t.Fatalf("items = %+v", items)
	}

	showOut, err := runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(showOut, "sync:        ahead by 1 (not yet pushed)") {
		t.Fatalf("show output missing sync line:\n%s", showOut)
	}
}

func TestAllMarksDivergedItem(t *testing.T) {
	fx := newSyncedRepoPair(t)

	origA := chdirTo(t, fx.repoA)
	if err := gitx.SetConfig("backlog.init", "true"); err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newSyncCmd()); err != nil {
		t.Fatal(err)
	}

	chdirTo(t, fx.repoB)
	if err := gitx.SetConfig("backlog.init", "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newSyncCmd()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPriority(item.ID, store.PriorityP2, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newSyncCmd()); err != nil {
		t.Fatal(err)
	}

	chdirTo(t, fx.repoA)
	if _, err := store.SetComment(item.ID, "flaky under -race", nil); err != nil {
		t.Fatal(err)
	}
	if err := gitx.Fetch("origin", "refs/backlog/*:refs/remotes/origin/backlog/*"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newAllCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1 diverged") {
		t.Fatalf("expected the summary to mention 1 diverged:\n%s", out)
	}
	if !strings.Contains(out, "fix flaky test  ⇕") {
		t.Fatalf("expected the item's line to have the diverged marker:\n%s", out)
	}
	chdirTo(t, origA)
}
