package cmd

import (
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestListMovesItem(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newListCmd(), item.ID, "current")
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
	if updated.List != store.ListCurrent {
		t.Fatalf("List = %s, want current", updated.List)
	}
}

func TestListRejectsInvalidValue(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("x", store.ListBacklog, store.PriorityNone)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newListCmd(), item.ID, "sometimeslist"); err == nil {
		t.Fatal("expected an error for an invalid list value")
	}
}

func TestListRejectsUnknownID(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := runCmd(t, newListCmd(), "deadbeef", "current"); err == nil {
		t.Fatal("expected an error for an unknown id")
	}
}
