package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// syncFixture is a bare remote plus two clones, each with its own git
// identity, wired for `sync` tests. Callers chdir(t, repoA) / chdir(t,
// repoB) to switch which clone store calls operate against.
type syncFixture struct {
	repoA, repoB string
}

func newSyncFixture(t *testing.T) syncFixture {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	runGit(t, "", "init", "--bare", "-q", remote)

	repoA := filepath.Join(root, "repoA")
	repoB := filepath.Join(root, "repoB")
	runGit(t, "", "clone", "-q", remote, repoA)
	runGit(t, "", "clone", "-q", remote, repoB)
	runGit(t, repoA, "config", "user.name", "alice")
	runGit(t, repoA, "config", "user.email", "alice@example.com")
	runGit(t, repoB, "config", "user.name", "bob")
	runGit(t, repoB, "config", "user.email", "bob@example.com")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	return syncFixture{repoA: repoA, repoB: repoB}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v (dir=%s): %v\n%s", args, dir, err, out)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func TestSyncAdoptsItemsFromRemote(t *testing.T) {
	fx := newSyncFixture(t)

	chdir(t, fx.repoA)
	item, err := CreateItem("fix flaky test", ListBacklog, PriorityP1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}

	chdir(t, fx.repoB)
	report, err := Sync("origin")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Adopted) != 1 || report.Adopted[0] != item.ID {
		t.Fatalf("Adopted = %v, want [%s]", report.Adopted, item.ID)
	}
	loaded, err := LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Title != item.Title || loaded.Priority != item.Priority {
		t.Fatalf("adopted item mismatch: %+v", loaded)
	}
}

func TestSyncFastForwards(t *testing.T) {
	fx := newSyncFixture(t)

	chdir(t, fx.repoA)
	item, err := CreateItem("fix flaky test", ListBacklog, PriorityNone)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}

	chdir(t, fx.repoB)
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}

	chdir(t, fx.repoA)
	if _, err := SetPriority(item.ID, PriorityP0); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}

	chdir(t, fx.repoB)
	report, err := Sync("origin")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.FastForwarded) != 1 || report.FastForwarded[0] != item.ID {
		t.Fatalf("FastForwarded = %v, want [%s]", report.FastForwarded, item.ID)
	}
	loaded, err := LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Priority != PriorityP0 {
		t.Fatalf("priority = %q, want p0", loaded.Priority)
	}
}

// TestSyncMergesConcurrentEditsToDifferentFields is the scenario from
// docs/design/git-backlog.md's "Sync & conflict resolution": two clones
// each edit a different field on the same item while offline. Both edits
// must survive the merge.
func TestSyncMergesConcurrentEditsToDifferentFields(t *testing.T) {
	fx := newSyncFixture(t)

	chdir(t, fx.repoA)
	item, err := CreateItem("fix flaky test", ListBacklog, PriorityP1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}

	chdir(t, fx.repoB)
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}
	if _, err := SetPriority(item.ID, PriorityP0); err != nil {
		t.Fatal(err)
	}

	chdir(t, fx.repoA)
	if _, err := SetList(item.ID, ListCurrent); err != nil {
		t.Fatal(err)
	}

	// B pushes its priority change first.
	chdir(t, fx.repoB)
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}

	// A's push now conflicts (non-fast-forward) and must reconcile.
	chdir(t, fx.repoA)
	report, err := Sync("origin")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Merged) != 1 || report.Merged[0] != item.ID {
		t.Fatalf("Merged = %v, want [%s]", report.Merged, item.ID)
	}
	final, err := LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.List != ListCurrent {
		t.Errorf("List = %s, want current (A's edit should survive)", final.List)
	}
	if final.Priority != PriorityP0 {
		t.Errorf("Priority = %s, want p0 (B's edit should survive)", final.Priority)
	}

	// B fast-forwards to the same merge result.
	chdir(t, fx.repoB)
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}
	afterB, err := LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterB.Tip != final.Tip {
		t.Fatalf("B's tip = %s, want to converge on A's merge tip %s", afterB.Tip, final.Tip)
	}
}

// TestSyncSameFieldConflictConvergesDeterministically covers the harder
// case: both clones edit the *same* field concurrently. There's no way to
// keep both, but every clone must land on the identical resolution.
func TestSyncSameFieldConflictConvergesDeterministically(t *testing.T) {
	fx := newSyncFixture(t)

	chdir(t, fx.repoA)
	item, err := CreateItem("fix flaky test", ListBacklog, PriorityNone)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}
	chdir(t, fx.repoB)
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}

	chdir(t, fx.repoA)
	if _, err := SetPriority(item.ID, PriorityP1); err != nil {
		t.Fatal(err)
	}
	chdir(t, fx.repoB)
	if _, err := SetPriority(item.ID, PriorityP2); err != nil {
		t.Fatal(err)
	}

	// B pushes first, A must reconcile the same-field conflict.
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}
	chdir(t, fx.repoA)
	report, err := Sync("origin")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Merged) != 1 {
		t.Fatalf("Merged = %v, want exactly one reconciled item", report.Merged)
	}
	resolvedA, err := LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedA.Priority != PriorityP1 && resolvedA.Priority != PriorityP2 {
		t.Fatalf("priority = %q, want p1 or p2 (nothing else)", resolvedA.Priority)
	}

	chdir(t, fx.repoB)
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}
	resolvedB, err := LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedB.Priority != resolvedA.Priority {
		t.Fatalf("clones diverged on the same-field conflict: A=%s B=%s", resolvedA.Priority, resolvedB.Priority)
	}
	if resolvedB.Tip != resolvedA.Tip {
		t.Fatalf("clones converged on different priorities but different tips: A=%s B=%s", resolvedA.Tip, resolvedB.Tip)
	}
}
