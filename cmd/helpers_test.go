package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/spf13/cobra"
)

// chdirTempRepo creates a fresh, backlog-initialized git repo and chdirs
// the test process into it, restoring the original cwd on cleanup. Every
// command operates on the process cwd (same as plain git), so tests need a
// real repo underfoot.
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

	if err := gitx.SetConfig("backlog.init", "true"); err != nil {
		t.Fatal(err)
	}
	return dir
}

// chdirTo chdirs the test process to dir and returns the previous cwd, for
// tests that need a repo without the full backlog-initialized setup that
// chdirTempRepo provides (e.g. testing the requireInit guard itself).
func chdirTo(t *testing.T, dir string) string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return orig
}

// runGitInit creates a bare `git init` repo with no backlog setup at all.
func runGitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

// runCmd executes a freshly constructed cobra command with args, capturing
// stdout. Commands print via fmt.Print* directly to os.Stdout rather than
// cmd.OutOrStdout(), so stdout itself has to be redirected.
func runCmd(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(args)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	runErr := cmd.Execute()

	w.Close()
	os.Stdout = orig
	out := <-outCh
	return out, runErr
}
