package cmd

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/nosaka4i/git-backlog/internal/store"
)

// The Lamport-clock merge algorithm itself is already covered thoroughly in
// internal/store; these tests just check the cmd layer's wiring (error
// surfacing, remote selection, report formatting).

func TestSyncErrorsWithNoRemote(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := runCmd(t, newSyncCmd()); err == nil {
		t.Fatal("expected an error when no remote is configured")
	}
}

func TestSyncReportsAdoption(t *testing.T) {
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

	orig := chdirTo(t, repoA)
	defer chdirTo(t, orig)
	if err := gitx.SetConfig("backlog.init", "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateItem("fix flaky test", store.ListBacklog, store.PriorityNone); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, newSyncCmd()); err != nil {
		t.Fatal(err)
	}

	chdirTo(t, repoB)
	if err := gitx.SetConfig("backlog.init", "true"); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newSyncCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "synced with origin") || !strings.Contains(out, "adopted 1 item") {
		t.Fatalf("output = %q", out)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v (dir=%s): %v\n%s", args, dir, err, out)
	}
}
