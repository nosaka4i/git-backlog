package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestCommentSetsAndClears(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newCommentCmd(), item.ID, "flaky under -race, not otherwise")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Updated item comment successfully: ") {
		t.Fatalf("output = %q", out)
	}
	updated, err := store.LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Comment != "flaky under -race, not otherwise" {
		t.Fatalf("Comment = %q", updated.Comment)
	}

	if _, err := runCmd(t, newCommentCmd(), item.ID, ""); err != nil {
		t.Fatal(err)
	}
	updated, err = store.LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Comment != "" {
		t.Fatalf("Comment should be cleared, got %q", updated.Comment)
	}
}

func TestShowIncludesComment(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "comment:     (none)") {
		t.Fatalf("expected unset comment to show (none):\n%s", out)
	}

	if _, err := runCmd(t, newCommentCmd(), item.ID, "flaky under -race"); err != nil {
		t.Fatal(err)
	}
	out, err = runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "comment:     flaky under -race") {
		t.Fatalf("expected comment text in show output:\n%s", out)
	}
	if !strings.Contains(out, "Updated comment") {
		t.Fatalf("expected \"Updated comment\" in history:\n%s", out)
	}
}

func TestCommentShowListsNewestFirst(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, newCommentCmd(), "show", item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "no comments" {
		t.Fatalf("expected \"no comments\" for a fresh item, got %q", out)
	}

	if _, err := runCmd(t, newCommentCmd(), item.ID, "flaky under -race"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newCommentCmd(), item.ID, "confirmed, adding retry"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newCommentCmd(), item.ID, ""); err != nil {
		t.Fatal(err)
	}

	out, err = runCmd(t, newCommentCmd(), "show", item.ID)
	if err != nil {
		t.Fatal(err)
	}
	firstIdx := strings.Index(out, "flaky under -race")
	secondIdx := strings.Index(out, "confirmed, adding retry")
	clearedIdx := strings.Index(out, "(cleared)")
	if firstIdx < 0 || secondIdx < 0 || clearedIdx < 0 || !(clearedIdx < secondIdx && secondIdx < firstIdx) {
		t.Fatalf("expected comments newest first:\n%s", out)
	}

	jsonOut, err := runCmd(t, newCommentCmd(), "show", item.ID, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []jsonCommentEntry
	if err := json.Unmarshal([]byte(jsonOut), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, jsonOut)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if entries[1].Text != "confirmed, adding retry" || entries[2].Text != "flaky under -race" {
		t.Fatalf("entries = %+v", entries)
	}
	if !entries[0].Cleared {
		t.Fatalf("entries[0] should be marked cleared: %+v", entries[0])
	}
}

func TestHistoryShowsClearedComment(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetComment(item.ID, "flaky under -race", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetComment(item.ID, "", nil); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newHistoryCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Cleared comment") {
		t.Fatalf("expected \"Cleared comment\":\n%s", out)
	}
}
