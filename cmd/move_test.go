package cmd

import (
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestMoveMovesItem(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newMoveCmd(), item.ID, "current")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Moved item successfully: ") {
		t.Fatalf("output = %q", out)
	}
	updated, err := store.LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Track != store.TrackCurrent {
		t.Fatalf("List = %s, want current", updated.Track)
	}
}

func TestMoveRejectsInvalidValue(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("x", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newMoveCmd(), item.ID, "sometimeslist"); err == nil {
		t.Fatal("expected an error for an invalid list value")
	}
}

func TestMoveRejectsUnknownID(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := runCmd(t, newMoveCmd(), "deadbeef", "current"); err == nil {
		t.Fatal("expected an error for an unknown id")
	}
}
