package cmd

import (
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

func TestInitSetsConfig(t *testing.T) {
	dir := t.TempDir()
	orig := chdirTo(t, dir)
	defer chdirTo(t, orig)
	runGitInit(t, dir)

	out, err := runCmd(t, newInitCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "initialized backlog") {
		t.Fatalf("output = %q", out)
	}
	if v, ok := gitx.Config("backlog.init"); !ok || v != "true" {
		t.Fatalf("backlog.init config = %q, %v", v, ok)
	}
}

func TestInitTwiceIsANoop(t *testing.T) {
	dir := t.TempDir()
	orig := chdirTo(t, dir)
	defer chdirTo(t, orig)
	runGitInit(t, dir)

	if _, err := runCmd(t, newInitCmd()); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, newInitCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "already initialized") {
		t.Fatalf("output = %q", out)
	}
}

func TestInitOutsideGitRepo(t *testing.T) {
	dir := t.TempDir() // not a git repo
	orig := chdirTo(t, dir)
	defer chdirTo(t, orig)

	if _, err := runCmd(t, newInitCmd()); err == nil {
		t.Fatal("expected an error outside a git repository")
	}
}
