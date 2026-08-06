package store

import (
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

func TestParseList(t *testing.T) {
	for _, ok := range []string{"backlog", "current", "closed"} {
		if _, err := ParseTrack(ok); err != nil {
			t.Errorf("ParseTrack(%q) unexpected error: %v", ok, err)
		}
	}
	if _, err := ParseTrack("bogus"); err == nil {
		t.Error("ParseTrack(\"bogus\") expected error, got nil")
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
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if item.Title != "fix flaky test" || item.Track != TrackBacklog || item.Priority != PriorityP1 {
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
	item, err := CreateItem("write docs", TrackBacklog, PriorityNone, nil)
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
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := SetTrack(item.ID, TrackCurrent, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Track != TrackCurrent {
		t.Fatalf("Track = %s, want current", updated.Track)
	}
	if updated.Priority != PriorityP1 {
		t.Fatalf("priority should survive a list change unchanged, got %s", updated.Priority)
	}
	if updated.Tip == updated.ID {
		t.Fatal("tip should have moved past the create commit after an update")
	}

	updated, err = SetPriority(item.ID, PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Priority != PriorityNone {
		t.Fatalf("priority = %q, want cleared", updated.Priority)
	}

	updated, err = SetTitle(item.ID, "fix the flaky auth test", nil)
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

func TestSetComment(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if item.Comment != "" {
		t.Fatalf("comment should be unset on create, got %q", item.Comment)
	}

	updated, err := SetComment(item.ID, "flaky under -race, not otherwise", nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Comment != "flaky under -race, not otherwise" {
		t.Fatalf("comment = %q", updated.Comment)
	}

	updated, err = SetComment(item.ID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Comment != "" {
		t.Fatalf("comment should be cleared, got %q", updated.Comment)
	}
}

func TestSetCommentAsAgentIdentity(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	agent := &Identity{Name: "Claude", Email: "claude@example.com"}
	updated, err := SetComment(item.ID, "flaky under -race", agent)
	if err != nil {
		t.Fatal(err)
	}
	// The item's owner is fixed at creation and unaffected by an
	// identity override on a later op.
	if updated.OwnerName != "alice" || updated.OwnerEmail != "alice@example.com" {
		t.Fatalf("owner should stay fixed at creation: got %s <%s>", updated.OwnerName, updated.OwnerEmail)
	}

	history, err := History(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	last := history[len(history)-1]
	if last.AuthorName != "Claude" || last.AuthorEmail != "claude@example.com" {
		t.Fatalf("last op author = %s <%s>, want the agent identity", last.AuthorName, last.AuthorEmail)
	}
}

func TestSetDescription(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if item.Description != "" {
		t.Fatalf("description should be unset on create, got %q", item.Description)
	}

	updated, err := SetDescription(item.ID, "flakes under -race due to a shared temp dir", nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "flakes under -race due to a shared temp dir" {
		t.Fatalf("description = %q", updated.Description)
	}

	updated, err = SetDescription(item.ID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "" {
		t.Fatalf("description should be cleared, got %q", updated.Description)
	}
}

func TestSetDescriptionAsAgentIdentity(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	agent := &Identity{Name: "Claude", Email: "claude@example.com"}
	updated, err := SetDescription(item.ID, "flakes under -race", agent)
	if err != nil {
		t.Fatal(err)
	}
	if updated.OwnerName != "alice" || updated.OwnerEmail != "alice@example.com" {
		t.Fatalf("owner should stay fixed at creation: got %s <%s>", updated.OwnerName, updated.OwnerEmail)
	}

	history, err := History(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	last := history[len(history)-1]
	if last.AuthorName != "Claude" || last.AuthorEmail != "claude@example.com" {
		t.Fatalf("last op author = %s <%s>, want the agent identity", last.AuthorName, last.AuthorEmail)
	}
}

func TestAllItems(t *testing.T) {
	chdirTempRepo(t, "alice")
	titles := []string{"a", "b", "c"}
	for _, ttl := range titles {
		if _, err := CreateItem(ttl, TrackBacklog, PriorityNone, nil); err != nil {
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
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetPriority(item.ID, PriorityP2, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := SetTrack(item.ID, TrackClosed, nil); err != nil {
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
		t.Fatalf("create op changes = %+v, want 2 fields (title, track)", history[0].Changes)
	}
	if len(history[1].Changes) != 1 || history[1].Changes[0].Field != fieldPriority {
		t.Fatalf("priority op changes = %+v", history[1].Changes)
	}
	if len(history[2].Changes) != 1 || history[2].Changes[0].Field != fieldTrack {
		t.Fatalf("track op changes = %+v", history[2].Changes)
	}
}

func TestHistoryRecordsClearedField(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetPriority(item.ID, PriorityNone, nil); err != nil {
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

// createLegacyItem builds an item the way CreateItem used to, before the
// track rename: a "list" tree entry instead of "track". Used to simulate a
// pre-migration item for TestMigrateTrackField* below.
func createLegacyItem(t *testing.T, title string, track Track, priority Priority) *Item {
	t.Helper()
	entries, err := snapshotEntries(map[string]*string{
		fieldTitle:    &title,
		fieldList:     strPtr(string(track)),
		fieldPriority: priorityPtr(priority),
	}, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := gitx.MkTree(entries)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := gitx.CommitTree(tree, nil, "add: "+title)
	if err != nil {
		t.Fatal(err)
	}
	if err := gitx.UpdateRef(RefFor(commit), commit, ""); err != nil {
		t.Fatal(err)
	}
	item, err := LoadItem(commit)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func TestMigrateTrackFieldConvertsLegacyItem(t *testing.T) {
	chdirTempRepo(t, "alice")
	legacy := createLegacyItem(t, "fix flaky test", TrackCurrent, PriorityP1)
	if legacy.Track != TrackCurrent {
		t.Fatalf("legacy item track = %q, want current (via fallback to the list entry)", legacy.Track)
	}

	migrated, err := MigrateTrackField(legacy.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if migrated.Track != TrackCurrent {
		t.Fatalf("migrated item track = %q, want current (value must survive the migration)", migrated.Track)
	}
	if migrated.ID != legacy.ID {
		t.Fatalf("migration must not change the item's id: got %s, want %s", migrated.ID, legacy.ID)
	}

	history, err := History(migrated.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 ops (create, migrate), got %d", len(history))
	}
	last := history[len(history)-1]
	if !strings.HasPrefix(last.Message, "migrate:") {
		t.Fatalf("last op message = %q, want a migrate: prefix", last.Message)
	}
	var sawListRemoved, sawTrackAdded bool
	for _, ch := range last.Changes {
		switch {
		case ch.Field == fieldList && ch.Removed:
			sawListRemoved = true
		case ch.Field == fieldTrack && ch.Value == string(TrackCurrent):
			sawTrackAdded = true
		}
	}
	if !sawListRemoved || !sawTrackAdded {
		t.Fatalf("migrate op changes = %+v, want list removed + track added", last.Changes)
	}
}

func TestMigrateTrackFieldIsNoOpOnAlreadyMigratedItem(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := CreateItem("write docs", TrackBacklog, PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := MigrateTrackField(item.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if migrated.Tip != item.Tip {
		t.Fatalf("expected no new commit for an already-migrated item: tip changed from %s to %s", item.Tip, migrated.Tip)
	}
	history, err := History(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected no new op recorded, got %d ops", len(history))
	}
}
