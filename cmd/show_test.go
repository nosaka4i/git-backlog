package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestShowHumanOutput(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetTrack(item.ID, store.TrackCurrent, nil); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"title:       fix flaky test",
		"track:       current",
		"priority:    p1",
		"owner:       alice <alice@example.com>",
		"created:     ",
		"updated:     ",
		"history:",
		"Added item",
		"Moved to current",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestShowUnsetPriorityPrintsUnset(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("write docs", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "priority:    unset") {
		t.Fatalf("output = %q", out)
	}
}

func TestShowJSONIncludesHistory(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetTrack(item.ID, store.TrackCurrent, nil); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newShowCmd(), item.ID, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var detail jsonItemDetail
	if err := json.Unmarshal([]byte(out), &detail); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if detail.ID != item.ID || detail.Track != "current" {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.Tip == detail.ID {
		t.Fatal("tip should have moved past the create commit")
	}
	if len(detail.History) != 2 {
		t.Fatalf("history = %d ops, want 2", len(detail.History))
	}
	// Newest first: the list-change op should come before the create op.
	if len(detail.History[0].Changes) == 0 || detail.History[0].Changes[0].Field != "track" {
		t.Fatalf("expected the list-change op first (newest), got %+v", detail.History[0])
	}
}

func TestShowHistoryIsNewestFirst(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetTitle(item.ID, "fix flaky test, take 2", nil); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	addedIdx := strings.Index(out, "Added item")
	renamedIdx := strings.Index(out, "Renamed item")
	if addedIdx < 0 || renamedIdx < 0 {
		t.Fatalf("missing expected history lines:\n%s", out)
	}
	if !(renamedIdx < addedIdx) {
		t.Fatalf("expected the more recent \"Renamed item\" to appear before \"Added item\":\n%s", out)
	}
}

func TestShowUnknownID(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := runCmd(t, newShowCmd(), "deadbeef"); err == nil {
		t.Fatal("expected an error for an unknown id")
	}
}
