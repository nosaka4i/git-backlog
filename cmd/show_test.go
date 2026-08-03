package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestShowHumanOutput(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetList(item.ID, store.ListCurrent, nil); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"title:    fix flaky test",
		"list:     current",
		"priority: p1",
		"owner:    alice <alice@example.com>",
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
	item, err := store.CreateItem("write docs", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newShowCmd(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "priority: unset") {
		t.Fatalf("output = %q", out)
	}
}

func TestShowJSONIncludesHistory(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetList(item.ID, store.ListCurrent, nil); err != nil {
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
	if detail.ID != item.ID || detail.List != "current" {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.Tip == detail.ID {
		t.Fatal("tip should have moved past the create commit")
	}
	if len(detail.History) != 2 {
		t.Fatalf("history = %d ops, want 2", len(detail.History))
	}
}

func TestShowUnknownID(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := runCmd(t, newShowCmd(), "deadbeef"); err == nil {
		t.Fatal("expected an error for an unknown id")
	}
}
