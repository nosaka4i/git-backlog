package cmd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

var shortIDRe = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

func TestAddDefaultsListBacklogPriorityUnset(t *testing.T) {
	chdirTempRepo(t, "alice")
	out, err := runCmd(t, newAddCmd(), "fix flaky test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Added item successfully: ") {
		t.Fatalf("output = %q", out)
	}
	id := strings.TrimSpace(strings.TrimPrefix(out, "Added item successfully: "))
	if !shortIDRe.MatchString(id) {
		t.Fatalf("output didn't end in a short id: %q", out)
	}

	item, err := store.LoadItem(id)
	if err != nil {
		t.Fatal(err)
	}
	if item.Title != "fix flaky test" || item.Track != store.TrackBacklog || item.Priority != store.PriorityNone {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestAddWithListAndPriorityFlags(t *testing.T) {
	chdirTempRepo(t, "alice")
	out, err := runCmd(t, newAddCmd(), "ship release", "--track", "current", "--priority", "p0")
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(strings.TrimPrefix(out, "Added item successfully: "))
	item, err := store.LoadItem(id)
	if err != nil {
		t.Fatal(err)
	}
	if item.Track != store.TrackCurrent || item.Priority != store.PriorityP0 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestAddWithDescriptionFlag(t *testing.T) {
	chdirTempRepo(t, "alice")
	out, err := runCmd(t, newAddCmd(), "fix flaky test", "--description", "flakes under -race due to a shared temp dir")
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(strings.TrimPrefix(out, "Added item successfully: "))
	item, err := store.LoadItem(id)
	if err != nil {
		t.Fatal(err)
	}
	if item.Description != "flakes under -race due to a shared temp dir" {
		t.Fatalf("Description = %q", item.Description)
	}

	history, err := store.History(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 op-log entries (create, then a description edit), got %d", len(history))
	}
}

func TestAddWithoutDescriptionFlagLeavesItUnset(t *testing.T) {
	chdirTempRepo(t, "alice")
	out, err := runCmd(t, newAddCmd(), "fix flaky test")
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(strings.TrimPrefix(out, "Added item successfully: "))
	item, err := store.LoadItem(id)
	if err != nil {
		t.Fatal(err)
	}
	if item.Description != "" {
		t.Fatalf("expected no description, got %q", item.Description)
	}
	history, err := store.History(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected only the create op when --description is omitted, got %d entries", len(history))
	}
}

func TestAddRejectsEmptyTitle(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := runCmd(t, newAddCmd(), "   "); err == nil {
		t.Fatal("expected an error for an empty title")
	}
}

func TestAddRejectsInvalidListAndPriority(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := runCmd(t, newAddCmd(), "x", "--track", "bogus"); err == nil {
		t.Fatal("expected an error for an invalid --list value")
	}
	if _, err := runCmd(t, newAddCmd(), "x", "--priority", "p9"); err == nil {
		t.Fatal("expected an error for an invalid --priority value")
	}
}

func TestAddRequiresInit(t *testing.T) {
	dir := t.TempDir()
	orig := chdirTo(t, dir)
	defer chdirTo(t, orig)
	runGitInit(t, dir)
	if _, err := runCmd(t, newAddCmd(), "x"); err == nil {
		t.Fatal("expected an error when backlog hasn't been initialized")
	}
}
