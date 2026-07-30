package store

import (
	"os"
	"os/exec"
	"testing"
)

// chdirTempRepo creates a fresh git repo and chdirs the test process into
// it, restoring the original cwd on cleanup. Every store function operates
// on the process cwd (same as plain git), so tests need a real repo
// underfoot.
func chdirTempRepo(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.name", name)
	run("config", "user.email", name+"@example.com")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return dir
}
