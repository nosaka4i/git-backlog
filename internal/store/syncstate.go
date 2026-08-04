package store

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

// SyncStatus classifies an item's local ref against the last-known
// remote-tracking ref (refs/remotes/<remote>/backlog/<id>) — computed
// entirely from already-fetched local git state, no network call, the
// same way `git status` reports a branch's ahead/behind from its cached
// upstream tracking ref rather than fetching live. Only as fresh as the
// last `sync`; deliberately not auto-fetched, so read commands like
// `all`/`show` stay side-effect-free.
type SyncStatus string

const (
	SyncUpToDate  SyncStatus = "up_to_date"
	SyncAhead     SyncStatus = "ahead"
	SyncBehind    SyncStatus = "behind"
	SyncDiverged  SyncStatus = "diverged"
	SyncNotSynced SyncStatus = "not_synced" // no remote-tracking ref for this item yet
)

// ItemSyncState is one item's sync status plus how many op-log commits
// it's ahead/behind by (both zero unless Status is Ahead/Behind/Diverged).
type ItemSyncState struct {
	Status   SyncStatus
	AheadBy  int
	BehindBy int
}

// ResolveRemote resolves remote to a concrete remote name, or the
// sole/"origin" configured remote if remote == "" — same resolution Sync
// itself uses. Exported so callers checking many items' SyncState (e.g.
// `all`) can resolve it once up front instead of once per item; an error
// here (no remote configured, or ambiguous with none specified) means
// sync state simply isn't available, not that the caller should fail —
// see docs/design/git-backlog.md's Sync state section.
func ResolveRemote(remote string) (string, error) {
	if remote != "" {
		return remote, nil
	}
	return defaultRemote()
}

// SyncState computes id's sync status against remote (already resolved —
// see ResolveRemote), without fetching.
func SyncState(id, remote string) (ItemSyncState, error) {
	remoteRef := fmt.Sprintf("refs/remotes/%s/backlog/%s", remote, id)
	if !gitx.RefExists(remoteRef) {
		return ItemSyncState{Status: SyncNotSynced}, nil
	}
	localHash, err := gitx.Run("rev-parse", RefFor(id))
	if err != nil {
		return ItemSyncState{}, err
	}
	remoteHash, err := gitx.Run("rev-parse", remoteRef)
	if err != nil {
		return ItemSyncState{}, err
	}
	if localHash == remoteHash {
		return ItemSyncState{Status: SyncUpToDate}, nil
	}
	ahead, behind, err := gitx.AheadBehindCount(localHash, remoteHash)
	if err != nil {
		return ItemSyncState{}, err
	}
	switch {
	case behind == 0:
		return ItemSyncState{Status: SyncAhead, AheadBy: ahead}, nil
	case ahead == 0:
		return ItemSyncState{Status: SyncBehind, BehindBy: behind}, nil
	default:
		return ItemSyncState{Status: SyncDiverged, AheadBy: ahead, BehindBy: behind}, nil
	}
}
