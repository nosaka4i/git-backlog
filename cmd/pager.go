package cmd

import (
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

// pagerWriter returns where to print output, and a cleanup func that must
// be called once all writing is done (even on error). It mirrors git's own
// auto-paging behavior (see core.pager in git-config(1)): output is only
// piped through a pager when stdout is a terminal, and the pager command
// itself follows git's own precedence — $GIT_PAGER, then core.pager, then
// $PAGER, then "less" — so a user's existing git pager setup just works.
// noPager (a --no-pager flag) always skips paging, same as `git --no-pager`.
func pagerWriter(noPager bool) (io.Writer, func()) {
	if noPager || !isTerminal(os.Stdout) {
		return os.Stdout, func() {}
	}
	pagerCmd := pagerCommand()
	if pagerCmd == "" {
		return os.Stdout, func() {}
	}
	return startPager(pagerCmd)
}

// startPager unconditionally spawns pagerCmd (via "sh -c", so pager values
// with arguments/pipes work, matching git), wiring its stdout/stderr to the
// process's own and returning a writer for its stdin plus a cleanup func
// that closes the pipe and waits for it to exit. Split out from
// pagerWriter so the actual piping mechanics are testable without needing
// a real terminal on stdout.
func startPager(pagerCmd string) (io.Writer, func()) {
	c := exec.Command("sh", "-c", pagerCmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = pagerEnv(pagerCmd, os.Environ())
	stdin, err := c.StdinPipe()
	if err != nil {
		return os.Stdout, func() {}
	}
	if err := c.Start(); err != nil {
		return os.Stdout, func() {}
	}
	return stdin, func() {
		stdin.Close()
		_ = c.Wait()
	}
}

// pagerEnv appends less's comfort defaults when pagerCmd is exactly "less"
// and the user hasn't already configured $LESS: auto-exit if content fits
// on one screen (F), keep raw escape codes (R), skip the alternate-screen
// clear on startup (X) — matching git's own default LESS behavior.
func pagerEnv(pagerCmd string, base []string) []string {
	if pagerCmd != "less" {
		return base
	}
	for _, kv := range base {
		if strings.HasPrefix(kv, "LESS=") {
			return base
		}
	}
	return append(base, "LESS=FRX")
}

// pagerCommand resolves the pager to use, following git's own precedence.
// An explicitly-empty value (env var or config set to "") disables paging,
// distinct from the value being unset entirely.
func pagerCommand() string {
	if v, ok := os.LookupEnv("GIT_PAGER"); ok {
		return v
	}
	if v, ok := gitx.Config("core.pager"); ok {
		return v
	}
	if v, ok := os.LookupEnv("PAGER"); ok {
		return v
	}
	return "less"
}

func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
