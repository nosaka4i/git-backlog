package cmd

import (
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestTitleRenames(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newTitleCmd(), item.ID, "fix the flaky auth test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Renamed item successfully: ") {
		t.Fatalf("output = %q", out)
	}
	updated, err := store.LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "fix the flaky auth test" {
		t.Fatalf("Title = %q", updated.Title)
	}
}

func TestTitleRejectsEmpty(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("x", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newTitleCmd(), item.ID, "   "); err == nil {
		t.Fatal("expected an error for an empty title")
	}
}
