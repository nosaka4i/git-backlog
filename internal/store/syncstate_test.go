package store

import (
	"testing"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

// fetchOnly populates repoA's remote-tracking refs without running the
// full Sync reconciliation, so tests can observe an ahead/behind/diverged
// snapshot that a full Sync would otherwise immediately resolve away.
func fetchOnly(t *testing.T, remote string) {
	t.Helper()
	if err := gitx.Fetch(remote, "refs/backlog/*:refs/remotes/"+remote+"/backlog/*"); err != nil {
		t.Fatal(err)
	}
}

func TestSyncStateNotSyncedBeforeFirstSync(t *testing.T) {
	fx := newSyncFixture(t)
	chdir(t, fx.repoA)
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	state, err := SyncState(item.ID, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != SyncNotSynced {
		t.Fatalf("state = %+v, want SyncNotSynced", state)
	}
}

func TestSyncStateUpToDateAfterSync(t *testing.T) {
	fx := newSyncFixture(t)
	chdir(t, fx.repoA)
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}
	state, err := SyncState(item.ID, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != SyncUpToDate {
		t.Fatalf("state = %+v, want SyncUpToDate immediately after Sync (no second fetch needed)", state)
	}
}

func TestSyncStateAheadAfterLocalEditNotYetPushed(t *testing.T) {
	fx := newSyncFixture(t)
	chdir(t, fx.repoA)
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}
	if _, err := SetPriority(item.ID, PriorityP1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := SetTitle(item.ID, "fix flaky test, take 2", nil); err != nil {
		t.Fatal(err)
	}

	state, err := SyncState(item.ID, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != SyncAhead || state.AheadBy != 2 {
		t.Fatalf("state = %+v, want SyncAhead with AheadBy=2", state)
	}
}

func TestSyncStateBehindWhenRemoteHasNewerCommit(t *testing.T) {
	fx := newSyncFixture(t)
	chdir(t, fx.repoA)
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityNone, nil)
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
	if _, err := SetPriority(item.ID, PriorityP2, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}

	// Back in A: fetch (not a full Sync, which would fast-forward and
	// resolve the very state we're trying to observe) to learn about B's
	// push, without advancing A's own local ref.
	chdir(t, fx.repoA)
	fetchOnly(t, "origin")

	state, err := SyncState(item.ID, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != SyncBehind || state.BehindBy != 1 {
		t.Fatalf("state = %+v, want SyncBehind with BehindBy=1", state)
	}
}

func TestSyncStateDivergedWhenBothSidesHaveUnsyncedEdits(t *testing.T) {
	fx := newSyncFixture(t)
	chdir(t, fx.repoA)
	item, err := CreateItem("fix flaky test", TrackBacklog, PriorityNone, nil)
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
	if _, err := SetPriority(item.ID, PriorityP2, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync("origin"); err != nil {
		t.Fatal(err)
	}

	chdir(t, fx.repoA)
	if _, err := SetComment(item.ID, "flaky under -race", nil); err != nil {
		t.Fatal(err)
	}
	fetchOnly(t, "origin")

	state, err := SyncState(item.ID, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != SyncDiverged || state.AheadBy != 1 || state.BehindBy != 1 {
		t.Fatalf("state = %+v, want SyncDiverged with AheadBy=1, BehindBy=1", state)
	}
}

func TestResolveRemoteDefaultsToOrigin(t *testing.T) {
	fx := newSyncFixture(t)
	chdir(t, fx.repoA)
	remote, err := ResolveRemote("")
	if err != nil {
		t.Fatal(err)
	}
	if remote != "origin" {
		t.Fatalf("ResolveRemote(\"\") = %q, want origin", remote)
	}
}

func TestResolveRemoteErrorsWithoutAnyRemote(t *testing.T) {
	chdirTempRepo(t, "alice")
	if _, err := ResolveRemote(""); err == nil {
		t.Fatal("expected an error when no remote is configured")
	}
}
