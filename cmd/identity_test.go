package cmd

import (
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/nosaka4i/git-backlog/internal/store"
)

func TestResolveAgentIdentityFalseIsAmbient(t *testing.T) {
	chdirTempRepo(t, "alice")
	identity, err := resolveAgentIdentity(false)
	if err != nil {
		t.Fatal(err)
	}
	if identity != nil {
		t.Fatalf("expected nil identity when --as-agent isn't set, got %+v", identity)
	}
}

func TestResolveAgentIdentityErrorsWithoutConfig(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := resolveAgentIdentity(true); err == nil {
		t.Fatal("expected an error when backlog.agent.* isn't configured")
	}

	if err := gitx.SetConfig("backlog.agent.name", "Claude"); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveAgentIdentity(true); err == nil {
		t.Fatal("expected an error when backlog.agent.email is still missing")
	}
}

func TestResolveAgentIdentityReadsConfig(t *testing.T) {
	chdirTempRepo(t, "alice")
	if err := gitx.SetConfig("backlog.agent.name", "Claude"); err != nil {
		t.Fatal(err)
	}
	if err := gitx.SetConfig("backlog.agent.email", "claude@example.com"); err != nil {
		t.Fatal(err)
	}
	identity, err := resolveAgentIdentity(true)
	if err != nil {
		t.Fatal(err)
	}
	if identity == nil || identity.Name != "Claude" || identity.Email != "claude@example.com" {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestCommentAsAgentAttributesTheOp(t *testing.T) {
	chdirTempRepo(t, "alice")
	if err := gitx.SetConfig("backlog.agent.name", "Claude"); err != nil {
		t.Fatal(err)
	}
	if err := gitx.SetConfig("backlog.agent.email", "claude@example.com"); err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newCommentCmd(), item.ID, "looks flaky under -race", "--as-agent"); err != nil {
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

	// A plain (non-agent) comment still attributes to the ambient identity.
	if _, err := runCmd(t, newCommentCmd(), item.ID, "confirmed by alice"); err != nil {
		t.Fatal(err)
	}
	history, err = store.History(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	last = history[len(history)-1]
	if last.AuthorName != "alice" {
		t.Fatalf("expected the ambient identity without --as-agent, got %s", last.AuthorName)
	}
}

func TestAddAsAgentSetsPermanentOwner(t *testing.T) {
	chdirTempRepo(t, "alice")
	if err := gitx.SetConfig("backlog.agent.name", "Claude"); err != nil {
		t.Fatal(err)
	}
	if err := gitx.SetConfig("backlog.agent.email", "claude@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newAddCmd(), "agent-created item", "--as-agent"); err != nil {
		t.Fatal(err)
	}
	items, err := store.AllItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].OwnerName != "Claude" || items[0].OwnerEmail != "claude@example.com" {
		t.Fatalf("owner = %s <%s>, want Claude <claude@example.com>", items[0].OwnerName, items[0].OwnerEmail)
	}

	// A plain (non-agent) add still owns to the ambient identity.
	if _, err := runCmd(t, newAddCmd(), "human-created item"); err != nil {
		t.Fatal(err)
	}
	items, err = store.AllItems()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, it := range items {
		if it.Title == "human-created item" {
			found = true
			if it.OwnerName != "alice" {
				t.Fatalf("expected the ambient identity without --as-agent, got %s", it.OwnerName)
			}
		}
	}
	if !found {
		t.Fatal("human-created item not found")
	}
}

func TestCommentAsAgentErrorsWithoutConfig(t *testing.T) {
	chdirTempRepo(t, "alice")
	item, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runCmd(t, newCommentCmd(), item.ID, "text", "--as-agent")
	if err == nil {
		t.Fatal("expected an error when --as-agent is used without backlog.agent.* configured")
	}
	if !strings.Contains(err.Error(), "backlog.agent.name") {
		t.Fatalf("error should mention backlog.agent.name: %v", err)
	}
}
