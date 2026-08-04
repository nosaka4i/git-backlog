package cmd

import (
	"fmt"
	"strings"

	"github.com/nosaka4i/git-backlog/internal/store"
)

// syncLine renders one item's sync state for `show`'s "sync:" line. state
// is nil when no remote could be resolved (see store.ResolveRemote) —
// distinct from store.SyncNotSynced, which means a remote exists but this
// item has no remote-tracking ref yet (never synced).
func syncLine(state *store.ItemSyncState) string {
	if state == nil {
		return "no remote configured"
	}
	switch state.Status {
	case store.SyncUpToDate:
		return "up to date"
	case store.SyncAhead:
		return fmt.Sprintf("ahead by %d (not yet pushed)", state.AheadBy)
	case store.SyncBehind:
		return fmt.Sprintf("behind by %d (not yet pulled)", state.BehindBy)
	case store.SyncDiverged:
		return fmt.Sprintf("diverged (%d ahead, %d behind)", state.AheadBy, state.BehindBy)
	case store.SyncNotSynced:
		return "not yet synced"
	default:
		return string(state.Status)
	}
}

// syncMarker renders one item's sync state as a short suffix for `all`'s
// compact list view — "" when up to date or no remote configured, so the
// common case (an actively-synced backlog) doesn't add visual noise to
// every line.
func syncMarker(state *store.ItemSyncState) string {
	if state == nil {
		return ""
	}
	switch state.Status {
	case store.SyncAhead:
		return fmt.Sprintf("  ↑%d", state.AheadBy)
	case store.SyncBehind:
		return fmt.Sprintf("  ↓%d", state.BehindBy)
	case store.SyncDiverged:
		return "  ⇕"
	case store.SyncNotSynced:
		return "  (not synced)"
	default:
		return ""
	}
}

// syncSummaryLine renders `all`'s top-of-output aggregate, mirroring `git
// status`'s own summary sentence — only mentions categories that are
// actually non-zero, and falls back to a plain "up to date" statement
// when nothing needs attention, rather than a noisy all-zeros line.
func syncSummaryLine(remote string, states map[string]*store.ItemSyncState) string {
	var ahead, behind, diverged, notSynced int
	for _, s := range states {
		switch s.Status {
		case store.SyncAhead:
			ahead++
		case store.SyncBehind:
			behind++
		case store.SyncDiverged:
			diverged++
		case store.SyncNotSynced:
			notSynced++
		}
	}
	var parts []string
	if ahead > 0 {
		parts = append(parts, fmt.Sprintf("%d ahead", ahead))
	}
	if behind > 0 {
		parts = append(parts, fmt.Sprintf("%d behind", behind))
	}
	if diverged > 0 {
		parts = append(parts, fmt.Sprintf("%d diverged", diverged))
	}
	if notSynced > 0 {
		parts = append(parts, fmt.Sprintf("%d not yet synced", notSynced))
	}
	if len(parts) == 0 {
		return "up to date with " + remote
	}
	return strings.Join(parts, ", ")
}
