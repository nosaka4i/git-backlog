package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nosaka4i/git-backlog/internal/store"
)

// TestChainedOpsDoNotDropFields hammers the CLI with the same kind of
// back-to-back, no-pause invocation pattern that corrupted a real item in
// production (nosaka's d6e02ce, 2026-09-03): many add/title/describe/
// priority/label calls fired one after another with no delay, mirroring a
// shell script chaining several git-backlog commands with `&&`. A describe
// op there landed with every field but clock+description dropped, even
// though its parent commit's tree had all six fields intact — see
// applyOp's title-sanity-check comment in internal/store/item.go for the
// guard this test exercises.
//
// This can't force the underlying transient on demand (believed to be a
// git-level gc/repack window triggered by rapid loose-object creation) —
// it didn't reproduce in isolation even against the real incident's own
// recipe, scaled up. What it does check is the invariant that must hold
// either way: after any chain of ops, every item still has its title and
// track intact, or the op errored instead of silently writing over them.
func TestChainedOpsDoNotDropFields(t *testing.T) {
	chdirTempRepo(t, "alice")

	const numItems = 40
	ids := make([]string, numItems)
	for i := 0; i < numItems; i++ {
		out, err := runCmd(t, newAddCmd(), fmt.Sprintf("item %d", i), "--priority", "p2", "--label", "bug")
		if err != nil {
			t.Fatalf("add %d: %v\n%s", i, err, out)
		}
		id := strings.TrimSpace(strings.TrimPrefix(out, "Added item successfully:"))
		if id == "" {
			t.Fatalf("add %d: couldn't parse id from output: %q", i, out)
		}
		ids[i] = id
	}

	// Three rounds of chained title+describe+priority+label calls per
	// item, back to back, no pause — the highest write volume in the
	// shortest window this in-process harness can drive.
	for round := 0; round < 3; round++ {
		for i, id := range ids {
			// Long, not just the description — a title this size (~250
			// chars) is well past anything a human would type but not
			// unrealistic for an agent, and costs nothing extra to cover.
			newTitle := strings.Repeat(fmt.Sprintf("item %d round %d ", i, round), 12)
			if _, err := runCmd(t, newTitleCmd(), id, newTitle); err != nil {
				t.Fatalf("round %d title %d: %v", round, i, err)
			}
			longDesc := strings.Repeat(fmt.Sprintf("paragraph %d round %d.\n\n", i, round), 20)
			if _, err := runCmd(t, newDescribeCmd(), id, longDesc); err != nil {
				t.Fatalf("round %d describe %d: %v", round, i, err)
			}
			if _, err := runCmd(t, newPriorityCmd(), id, "p1"); err != nil {
				t.Fatalf("round %d priority %d: %v", round, i, err)
			}
			if _, err := runCmd(t, newLabelCmd(), id, "urgent"); err != nil {
				t.Fatalf("round %d label %d: %v", round, i, err)
			}
		}
	}

	all, skipped, err := store.AllItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) > 0 {
		t.Fatalf("%d item(s) failed to load after the chained burst: %v", len(skipped), skipped)
	}
	if len(all) != numItems {
		t.Fatalf("got %d items after the burst, want %d", len(all), numItems)
	}
	for _, it := range all {
		if it.Title == "" {
			t.Fatalf("item %s lost its title", store.ShortID(it.ID))
		}
		if it.Track == "" {
			t.Fatalf("item %s lost its track", store.ShortID(it.ID))
		}
	}
}
