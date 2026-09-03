package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

// TestListWorksFromASubdirectory guards against a real regression (found
// 2026-09-03: a peer session hit "has no title (corrupt?)" on every item
// in a 108-item backlog simply by running from a nested subdirectory
// instead of the repo root). Root cause: gitx.LsTree ran plain
// `git ls-tree <tree>`, which by default scopes its output to whatever
// path matches the caller's cwd *within that tree* — item trees are flat
// field blobs (title, track, ...) unrelated to the repo's real directory
// layout, so from any subdirectory that scoping matched nothing and
// silently emptied every field, including title. See gitx.LsTree's
// --full-tree comment for the fix.
func TestListWorksFromASubdirectory(t *testing.T) {
	root := chdirTempRepo(t, "alice")
	if _, err := store.CreateItem("fix flaky test", store.TrackBacklog, store.PriorityNone, nil); err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(root, "some", "nested", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	orig := chdirTo(t, sub)
	defer chdirTo(t, orig)

	out, err := runCmd(t, newListCmd())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "corrupt") {
		t.Fatalf("list reported corruption when run from a subdirectory:\n%s", out)
	}
	if !strings.Contains(out, "fix flaky test") {
		t.Fatalf("list didn't show the item when run from a subdirectory:\n%s", out)
	}
}
