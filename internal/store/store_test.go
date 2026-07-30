package store

import (
	"testing"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

func TestParseList(t *testing.T) {
	for _, ok := range []string{"backlog", "current", "closed"} {
		if _, err := ParseList(ok); err != nil {
			t.Errorf("ParseList(%q) unexpected error: %v", ok, err)
		}
	}
	if _, err := ParseList("bogus"); err == nil {
		t.Error("ParseList(\"bogus\") expected error, got nil")
	}
}

func TestParsePriority(t *testing.T) {
	for _, ok := range []string{"p0", "p1", "p2"} {
		p, err := ParsePriority(ok)
		if err != nil {
			t.Errorf("ParsePriority(%q) unexpected error: %v", ok, err)
		}
		if string(p) != ok {
			t.Errorf("ParsePriority(%q) = %q", ok, p)
		}
	}
	p, err := ParsePriority("none")
	if err != nil || p != PriorityNone {
		t.Errorf("ParsePriority(\"none\") = %q, %v", p, err)
	}
	if _, err := ParsePriority("p9"); err == nil {
		t.Error("ParsePriority(\"p9\") expected error, got nil")
	}
}

func TestPriorityRank(t *testing.T) {
	if PriorityP0.Rank() >= PriorityP1.Rank() {
		t.Error("p0 should rank before p1")
	}
	if PriorityP1.Rank() >= PriorityP2.Rank() {
		t.Error("p1 should rank before p2")
	}
	if PriorityP2.Rank() >= PriorityNone.Rank() {
		t.Error("p2 should rank before unset")
	}
}

func TestRefForIDFromRef(t *testing.T) {
	id := "abc123"
	ref := RefFor(id)
	if ref != "refs/backlog/abc123" {
		t.Fatalf("RefFor = %s", ref)
	}
	if got := IDFromRef(ref); got != id {
		t.Fatalf("IDFromRef = %s, want %s", got, id)
	}
}

func TestCreateAndLoadItem(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("fix flaky test", ListBacklog, PriorityP1)
	if err != nil {
		t.Fatal(err)
	}
	if item.Title != "fix flaky test" || item.List != ListBacklog || item.Priority != PriorityP1 {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.ID != item.Tip {
		t.Fatalf("a fresh item's tip should equal its id: id=%s tip=%s", item.ID, item.Tip)
	}
	if item.OwnerName != "alice" || item.OwnerEmail != "alice@example.com" {
		t.Fatalf("unexpected owner: %s <%s>", item.OwnerName, item.OwnerEmail)
	}
	if item.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}

	loaded, err := LoadItem(item.ID[:10])
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != item.ID || loaded.Title != item.Title {
		t.Fatalf("LoadItem by prefix mismatch: %+v", loaded)
	}
}

func TestCreateItemDefaultsPriorityUnset(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("write docs", ListBacklog, PriorityNone)
	if err != nil {
		t.Fatal(err)
	}
	if item.Priority != PriorityNone {
		t.Fatalf("expected unset priority, got %q", item.Priority)
	}
}

func TestResolveIDErrors(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := ResolveID("deadbeef"); err == nil {
		t.Fatal("expected error resolving a nonexistent id")
	}
	// A real object that isn't a commit at all (not even a candidate item).
	blob, err := gitx.HashBlob("not an item")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveID(blob[:10]); err == nil {
		t.Fatal("expected error resolving a non-commit object")
	}
}

func TestSetListSetPriorityEdit(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("fix flaky test", ListBacklog, PriorityP1)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := SetList(item.ID, ListCurrent)
	if err != nil {
		t.Fatal(err)
	}
	if updated.List != ListCurrent {
		t.Fatalf("List = %s, want current", updated.List)
	}
	if updated.Priority != PriorityP1 {
		t.Fatalf("priority should survive a list change unchanged, got %s", updated.Priority)
	}
	if updated.Tip == updated.ID {
		t.Fatal("tip should have moved past the create commit after an update")
	}

	updated, err = SetPriority(item.ID, PriorityNone)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Priority != PriorityNone {
		t.Fatalf("priority = %q, want cleared", updated.Priority)
	}

	updated, err = SetTitle(item.ID, "fix the flaky auth test")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "fix the flaky auth test" {
		t.Fatalf("title = %q", updated.Title)
	}

	// id and owner never change across the op-log.
	if updated.ID != item.ID || updated.OwnerEmail != item.OwnerEmail {
		t.Fatalf("id/owner must stay fixed: got id=%s owner=%s", updated.ID, updated.OwnerEmail)
	}
}

func TestAllItems(t *testing.T) {
	chdirTempRepo(t, "alice")
	titles := []string{"a", "b", "c"}
	for _, ttl := range titles {
		if _, err := CreateItem(ttl, ListBacklog, PriorityNone); err != nil {
			t.Fatal(err)
		}
	}
	items, err := AllItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != len(titles) {
		t.Fatalf("AllItems returned %d items, want %d", len(items), len(titles))
	}
}

func TestHistoryTracksEachOperation(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("fix flaky test", ListBacklog, PriorityNone)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetPriority(item.ID, PriorityP2); err != nil {
		t.Fatal(err)
	}
	if _, err := SetList(item.ID, ListClosed); err != nil {
		t.Fatal(err)
	}

	history, err := History(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("History returned %d ops, want 3", len(history))
	}
	if history[0].Clock != 0 || history[1].Clock != 1 || history[2].Clock != 2 {
		t.Fatalf("expected strictly increasing clocks, got %d, %d, %d",
			history[0].Clock, history[1].Clock, history[2].Clock)
	}
	// The create op sets every field given to it (priority was left unset,
	// so it's simply absent, not a 3rd field); later ops touch only what
	// changed.
	if len(history[0].Changes) != 2 {
		t.Fatalf("create op changes = %+v, want 2 fields (title, list)", history[0].Changes)
	}
	if len(history[1].Changes) != 1 || history[1].Changes[0].Field != fieldPriority {
		t.Fatalf("priority op changes = %+v", history[1].Changes)
	}
	if len(history[2].Changes) != 1 || history[2].Changes[0].Field != fieldList {
		t.Fatalf("list op changes = %+v", history[2].Changes)
	}
}

func TestHistoryRecordsClearedField(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("fix flaky test", ListBacklog, PriorityP1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetPriority(item.ID, PriorityNone); err != nil {
		t.Fatal(err)
	}
	history, err := History(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	last := history[len(history)-1]
	if len(last.Changes) != 1 || !last.Changes[0].Removed || last.Changes[0].Field != fieldPriority {
		t.Fatalf("expected a removed priority change, got %+v", last.Changes)
	}
}
