package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestLabelAddRemoveAndShow(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runCmd(t, newLabelCmd(), item.ID, "sprint-xyz", "backend"); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "labels:      backend, sprint-xyz") {
		t.Fatalf("show missing sorted labels line:\n%s", out)
	}

	if _, err := runCmd(t, newLabelCmd(), "--remove", item.ID, "backend"); err != nil {
		t.Fatal(err)
	}
	out, err = runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Scope the check to the current-state labels line: the op-log history
	// below it still legitimately mentions "backend" (that removal is a
	// recorded, immutable op).
	if !strings.Contains(out, "labels:      sprint-xyz\n") || strings.Contains(out, "labels:      backend") {
		t.Fatalf("show's labels line should read only sprint-xyz after removal:\n%s", out)
	}
}

func TestListLabelFilter(t *testing.T) {
	chdirTempRepo(t, "alice")
	a, err := store.CreateItem("in sprint", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddLabels(a.ID, []string{"sprint-xyz"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("not in sprint", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newListCmd(), "--label", "sprint-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "in sprint") || strings.Contains(out, "not in sprint") {
		t.Fatalf("--label filter didn't narrow to the labeled item:\n%s", out)
	}
	// The label is shown inline in the list view.
	if !strings.Contains(out, "[sprint-xyz]") {
		t.Fatalf("list should show labels inline:\n%s", out)
	}
}

func TestLabelRoster(t *testing.T) {
	chdirTempRepo(t, "alice")
	a, err := store.CreateItem("one", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.CreateItem("two", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddLabels(a.ID, []string{"sprint-xyz", "backend"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddLabels(b.ID, []string{"sprint-xyz"}, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newLabelCmd(), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var roster []jsonLabelCount
	if err := json.Unmarshal([]byte(out), &roster); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(roster) != 2 {
		t.Fatalf("roster = %+v, want 2 labels", roster)
	}
	// Most-used first: sprint-xyz (2) before backend (1).
	if roster[0].Label != "sprint-xyz" || roster[0].Count != 2 {
		t.Fatalf("roster[0] = %+v, want sprint-xyz x2", roster[0])
	}
	if roster[1].Label != "backend" || roster[1].Count != 1 {
		t.Fatalf("roster[1] = %+v, want backend x1", roster[1])
	}
}
