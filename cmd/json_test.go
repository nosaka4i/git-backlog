package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestToJSONItemOmitsUnsetPriority(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(toJSONItem(item))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"priority"`) {
		t.Fatalf("expected priority omitted from JSON, got %s", b)
	}
	if strings.Contains(string(b), `"comment"`) {
		t.Fatalf("expected unset comment omitted from JSON, got %s", b)
	}

	item2, err := store.CreateItem("ship release", store.ListCurrent, store.PriorityP0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetComment(item2.ID, "needs sign-off from bob", nil); err != nil {
		t.Fatal(err)
	}
	item2, err = store.LoadItem(item2.ID)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := json.Marshal(toJSONItem(item2))
	if err != nil {
		t.Fatal(err)
	}
	var decoded jsonItem
	if err := json.Unmarshal(b2, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != item2.ID || decoded.Title != item2.Title || decoded.List != "current" || decoded.Priority != "p0" {
		t.Fatalf("decoded = %+v", decoded)
	}
	if decoded.Comment != "needs sign-off from bob" {
		t.Fatalf("decoded.Comment = %q", decoded.Comment)
	}
	if decoded.Owner.Name != "alice" || decoded.Owner.Email != "alice@example.com" {
		t.Fatalf("owner = %+v", decoded.Owner)
	}
}

func TestToJSONHistoryMarksRemovedFields(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPriority(item.ID, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}
	history, err := store.History(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	ops := toJSONHistory(history)
	if len(ops) != 2 {
		t.Fatalf("got %d ops, want 2", len(ops))
	}
	last := ops[len(ops)-1]
	if len(last.Changes) != 1 || !last.Changes[0].Removed || last.Changes[0].Field != "priority" {
		t.Fatalf("last op changes = %+v", last.Changes)
	}
	if ops[0].Clock != 0 || ops[1].Clock != 1 {
		t.Fatalf("clocks = %d, %d", ops[0].Clock, ops[1].Clock)
	}
}
