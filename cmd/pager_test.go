package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

func TestPagerCommandDefaultsToLess(t *testing.T) {
	chdirTempRepo(t, "alice")
	t.Setenv("GIT_PAGER", "")
	os.Unsetenv("GIT_PAGER")
	os.Unsetenv("PAGER")
	if got := pagerCommand(); got != "less" {
		t.Fatalf("pagerCommand() = %q, want %q", got, "less")
	}
}

func TestPagerCommandPrecedence(t *testing.T) {
	chdirTempRepo(t, "alice")
	os.Unsetenv("GIT_PAGER")
	os.Unsetenv("PAGER")

	t.Setenv("PAGER", "pager-from-env")
	if got := pagerCommand(); got != "pager-from-env" {
		t.Fatalf("pagerCommand() = %q, want PAGER env value", got)
	}

	if err := gitx.SetConfig("core.pager", "pager-from-config"); err != nil {
		t.Fatal(err)
	}
	if got := pagerCommand(); got != "pager-from-config" {
		t.Fatalf("pagerCommand() = %q, want core.pager to beat $PAGER", got)
	}

	t.Setenv("GIT_PAGER", "pager-from-git-pager")
	if got := pagerCommand(); got != "pager-from-git-pager" {
		t.Fatalf("pagerCommand() = %q, want $GIT_PAGER to beat core.pager", got)
	}
}

func TestPagerCommandExplicitlyEmptyDisables(t *testing.T) {
	chdirTempRepo(t, "alice")
	os.Unsetenv("PAGER")
	t.Setenv("GIT_PAGER", "")
	if got := pagerCommand(); got != "" {
		t.Fatalf("pagerCommand() = %q, want empty string ($GIT_PAGER=\"\" disables paging)", got)
	}
}

func TestIsTerminalFalseForPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if isTerminal(w) {
		t.Fatal("expected a pipe to not report as a terminal")
	}
}

func TestPagerWriterSkipsPagingWhenNotATerminal(t *testing.T) {
	// os.Stdout is a pipe under `go test`, never a terminal, so paging
	// should always be skipped regardless of noPager.
	w, done := pagerWriter(false)
	defer done()
	if w != os.Stdout {
		t.Fatal("expected pagerWriter to return os.Stdout directly when stdout isn't a terminal")
	}
}

// TestStartPagerPipesOutputThroughSubprocess exercises the actual paging
// mechanism: startPager spawns a real subprocess ("cat", standing in for
// the user's pager) and whatever's written to the returned writer must
// come out the other end on stdout once done() has run.
func TestStartPagerPipesOutputThroughSubprocess(t *testing.T) {
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

	pw, done := startPager("cat")
	fmt.Fprintln(pw, "line one")
	fmt.Fprintln(pw, "line two")
	done()

	os.Stdout = orig
	w.Close()
	got := <-outCh
	want := "line one\nline two\n"
	if got != want {
		t.Fatalf("output through the pager subprocess = %q, want %q", got, want)
	}
}

func TestPagerEnvSetsLessDefaultsOnlyForLess(t *testing.T) {
	base := []string{"HOME=/home/alice", "PATH=/usr/bin"}

	got := pagerEnv("less", base)
	if !containsPrefix(got, "LESS=FRX") {
		t.Fatalf("expected LESS=FRX to be injected for the bare \"less\" pager, got %v", got)
	}

	got = pagerEnv("less -R", base)
	if containsPrefix(got, "LESS=") {
		t.Fatalf("expected no LESS injection for a pager command other than exactly \"less\", got %v", got)
	}

	withLess := append(append([]string{}, base...), "LESS=X")
	got = pagerEnv("less", withLess)
	if !containsExact(got, "LESS=X") || containsPrefix(withLess[:len(base)], "LESS=") {
		t.Fatalf("expected an existing $LESS to be left untouched, got %v", got)
	}
}

func containsPrefix(env []string, prefix string) bool {
	for _, kv := range env {
		if len(kv) >= len(prefix) && kv[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func containsExact(env []string, want string) bool {
	for _, kv := range env {
		if kv == want {
			return true
		}
	}
	return false
}

func TestHistoryNoPagerFlagWorks(t *testing.T) {
	chdirTempRepo(t, "alice")
	out, err := runCmd(t, newHistoryCmd(), "--no-pager")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no history entries yet, got:\n%s", out)
	}
}
