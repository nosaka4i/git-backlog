package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

// These guard a real regression (found 2026-09-03: a peer session hit
// "has no title (corrupt?)" on every item in a 108-item backlog simply by
// running from a nested subdirectory instead of the repo root). Root
// cause: gitx.LsTree ran plain `git ls-tree <tree>`, which by default
// scopes its output to whatever path matches the caller's cwd *within
// that tree* unless --full-tree is passed. Item trees are flat field
// blobs (title, track, priority, ...) unrelated to the repo's real
// directory layout, so from most subdirectories that scoping matched
// nothing and silently emptied every field, including title. See
// gitx.LsTree's --full-tree comment for the fix.
//
// Three angles, because "no corrupt error" alone isn't enough evidence:
//   - the direct repro (list from one nested dir)
//   - several read commands at several depths, root included as control
//   - the sneakier variant: a subdirectory name that happens to match a
//     field name, where pre-fix scoping would've matched ONE entry
//     instead of zero — no error printed at all, just silent data loss
//   - every write command run from a subdirectory, checked field-by-field
//     after each step, since a write could succeed and print success
//     while still dropping sibling fields with nothing to show for it

func assertNoCorruption(t *testing.T, out string) {
	t.Helper()
	if strings.Contains(out, "corrupt") {
		t.Fatalf("output reported corruption:\n%s", out)
	}
}

// TestListWorksFromASubdirectory is the direct repro from the original
// report: list from a nested subdirectory must show the same items as
// from the root, not "corrupt."
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
	assertNoCorruption(t, out)
	if !strings.Contains(out, "fix flaky test") {
		t.Fatalf("list didn't show the item when run from a subdirectory:\n%s", out)
	}
}

// TestReadCommandsWorkFromNestedSubdirectories checks list/show/history/
// label-roster/comment-show at several nesting depths, including the repo
// root itself (depth 0) as the control case that must keep passing too.
func TestReadCommandsWorkFromNestedSubdirectories(t *testing.T) {
	depths := []struct {
		name string
		segs []string
	}{
		{"repo root", nil},
		{"one level deep", []string{"pkg"}},
		{"several levels deep", []string{"packages", "server", "src", "handlers"}},
	}
	for _, depth := range depths {
		t.Run(depth.name, func(t *testing.T) {
			root := chdirTempRepo(t, "alice")
			item, err := store.CreateItem("investigate slow queries", store.TrackBacklog, store.PriorityP1, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.AddLabels(item.ID, []string{"perf"}, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := store.SetComment(item.ID, "started digging", nil); err != nil {
				t.Fatal(err)
			}

			if len(depth.segs) > 0 {
				dir := filepath.Join(append([]string{root}, depth.segs...)...)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				orig := chdirTo(t, dir)
				defer chdirTo(t, orig)
			}

			cases := []struct {
				name string
				cmd  *cobra.Command
				args []string
			}{
				{"list", newListCmd(), nil},
				{"show", newShowCmd(), []string{item.ID}},
				{"history", newHistoryCmd(), nil},
				{"label roster", newLabelCmd(), nil},
				{"comment show", newCommentShowCmd(), []string{item.ID}},
			}
			for _, c := range cases {
				out, err := runCmd(t, c.cmd, c.args...)
				if err != nil {
					t.Fatalf("%s: %v\n%s", c.name, err, out)
				}
				assertNoCorruption(t, out)
			}
		})
	}
}

// TestSubdirectoryNamedLikeAFieldDoesNotPartiallyMatch guards the
// sneakiest variant of this bug: before the --full-tree fix, if the
// invoking cwd's relative path happened to literally equal a field name
// (e.g. "title"), git ls-tree's scoping would find exactly ONE matching
// entry instead of zero — no "corrupt" error at all, just every other
// field silently read back as absent. A write from there would merge its
// change onto that single-field base and drop everything else with
// nothing printed to warn about it.
func TestSubdirectoryNamedLikeAFieldDoesNotPartiallyMatch(t *testing.T) {
	root := chdirTempRepo(t, "alice")
	item, err := store.CreateItem("keep every field", store.TrackBacklog, store.PriorityP1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddLabels(item.ID, []string{"regression"}, nil); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(root, "title") // deliberately matches a field name
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	orig := chdirTo(t, dir)
	defer chdirTo(t, orig)

	if out, err := runCmd(t, newCommentCmd(), item.ID, "updated from a decoy subdirectory"); err != nil {
		t.Fatalf("comment: %v\n%s", err, out)
	}

	got, err := store.LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "keep every field" {
		t.Fatalf("title lost: %+v", got)
	}
	if got.Track != store.TrackBacklog {
		t.Fatalf("track lost: %+v", got)
	}
	if got.Priority != store.PriorityP1 {
		t.Fatalf("priority lost: %+v", got)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "regression" {
		t.Fatalf("labels lost: %+v", got)
	}
	if got.Comment != "updated from a decoy subdirectory" {
		t.Fatalf("comment wasn't applied: %+v", got)
	}
}

// TestWriteCommandsPreserveAllFieldsFromASubdirectory runs every
// field-changing command from a nested subdirectory in sequence and
// checks the item's full state after each step — a write could succeed
// and print success while still silently dropping sibling fields, which a
// "did it error" check alone wouldn't catch.
func TestWriteCommandsPreserveAllFieldsFromASubdirectory(t *testing.T) {
	root := chdirTempRepo(t, "alice")
	item, err := store.CreateItem("original title", store.TrackBacklog, store.PriorityP2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddLabels(item.ID, []string{"keep-me"}, nil); err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(root, "packages", "server")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	orig := chdirTo(t, sub)
	defer chdirTo(t, orig)

	steps := []struct {
		name string
		run  func() (string, error)
	}{
		{"title", func() (string, error) { return runCmd(t, newTitleCmd(), item.ID, "renamed from subdir") }},
		{"describe", func() (string, error) { return runCmd(t, newDescribeCmd(), item.ID, "described from subdir") }},
		{"priority", func() (string, error) { return runCmd(t, newPriorityCmd(), item.ID, "p1") }},
		{"comment", func() (string, error) { return runCmd(t, newCommentCmd(), item.ID, "commented from subdir") }},
		{"move", func() (string, error) { return runCmd(t, newMoveCmd(), item.ID, "current") }},
		{"label add", func() (string, error) { return runCmd(t, newLabelCmd(), item.ID, "added-from-subdir") }},
	}

	for _, step := range steps {
		if out, err := step.run(); err != nil {
			t.Fatalf("%s: %v\n%s", step.name, err, out)
		}
		got, err := store.LoadItem(item.ID)
		if err != nil {
			t.Fatalf("after %s: reload: %v", step.name, err)
		}
		if got.Title == "" {
			t.Fatalf("after %s: title dropped", step.name)
		}
		if got.Track == "" {
			t.Fatalf("after %s: track dropped", step.name)
		}
		hasKeepMe := false
		for _, l := range got.Labels {
			if l == "keep-me" {
				hasKeepMe = true
			}
		}
		if !hasKeepMe {
			t.Fatalf("after %s: label %q was dropped: %+v", step.name, "keep-me", got.Labels)
		}
	}

	final, err := store.LoadItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Title != "renamed from subdir" {
		t.Fatalf("final title = %q", final.Title)
	}
	if final.Description != "described from subdir" {
		t.Fatalf("final description = %q", final.Description)
	}
	if final.Priority != store.PriorityP1 {
		t.Fatalf("final priority = %q", final.Priority)
	}
	if final.Comment != "commented from subdir" {
		t.Fatalf("final comment = %q", final.Comment)
	}
	if final.Track != store.TrackCurrent {
		t.Fatalf("final track = %q", final.Track)
	}
	labelSet := make(map[string]bool, len(final.Labels))
	for _, l := range final.Labels {
		labelSet[l] = true
	}
	if !labelSet["keep-me"] || !labelSet["added-from-subdir"] {
		t.Fatalf("final labels = %+v", final.Labels)
	}
}
