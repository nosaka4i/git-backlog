package cmd

import (
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestDescribeSetsAndClears(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newDescribeCmd(), item.ID, "flakes under -race due to a shared temp dir")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Updated item description successfully: ") {
		t.Fatalf("output = %q", out)
	}
	updated, err := store.LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "flakes under -race due to a shared temp dir" {
		t.Fatalf("Description = %q", updated.Description)
	}

	if _, err := runCmd(t, newDescribeCmd(), item.ID, ""); err != nil {
		t.Fatal(err)
	}
	updated, err = store.LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "" {
		t.Fatalf("Description should be cleared, got %q", updated.Description)
	}
}

func TestShowIncludesDescription(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "description: (none)") {
		t.Fatalf("expected unset description to show (none):\n%s", out)
	}

	if _, err := runCmd(t, newDescribeCmd(), item.ID, "what this item actually is"); err != nil {
		t.Fatal(err)
	}
	out, err = runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "description: what this item actually is") {
		t.Fatalf("expected description text in show output:\n%s", out)
	}
	if !strings.Contains(out, "Updated description") {
		t.Fatalf("expected \"Updated description\" in history:\n%s", out)
	}
}

func TestHistoryShowsClearedDescription(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newDescribeCmd(), item.ID, "temporary text"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newDescribeCmd(), item.ID, ""); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newHistoryCmd(), "--no-pager")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Cleared description") {
		t.Fatalf("expected \"Cleared description\" in history:\n%s", out)
	}
}

func TestDescribeAsAgentAttributesTheOp(t *testing.T) {
	chdirTempRepo(t, "alice")
	if err := gitx.SetConfig("backlog.agent.name", "Claude"); err != nil {
		t.Fatal(err)
	}
	if err := gitx.SetConfig("backlog.agent.email", "claude@example.com"); err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newDescribeCmd(), item.ID, "flakes under -race", "--as-agent"); err != nil {
		t.Fatal(err)
	}

	history, err := store.History(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	last := history[len(history)-1]
	if last.AuthorName != "Claude" || last.AuthorEmail != "claude@example.com" {
		t.Fatalf("last op author = %s <%s>, want Claude <claude@example.com>", last.AuthorName, last.AuthorEmail)
	}
}
