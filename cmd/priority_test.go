package cmd

import (
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestPrioritySetsAndClears(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newPriorityCmd(), item.ID, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Updated item priority successfully: ") {
		t.Fatalf("output = %q", out)
	}
	updated, err := store.LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Priority != store.PriorityP1 {
		t.Fatalf("Priority = %s, want p1", updated.Priority)
	}

	if _, err := runCmd(t, newPriorityCmd(), item.ID, "none"); err != nil {
		t.Fatal(err)
	}
	cleared, err := store.LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Priority != store.PriorityNone {
		t.Fatalf("Priority = %s, want cleared", cleared.Priority)
	}
}

func TestPriorityRejectsInvalidValue(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("x", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newPriorityCmd(), item.ID, "p9"); err == nil {
		t.Fatal("expected an error for an invalid priority value")
	}
}
