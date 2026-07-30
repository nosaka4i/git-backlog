package store

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

const backlogRefspec = "refs/backlog/*:refs/backlog/*"

// SyncReport summarizes what a Sync call did.
type SyncReport struct {
	Remote        string
	Adopted       []string // items that existed only on the remote
	FastForwarded []string // items whose local ref simply advanced
	Merged        []string // items reconciled from divergent op-logs
}

// Sync pushes and fetches the refs/backlog/* namespace against remote (or
// the sole/"origin" configured remote if remote == ""), reconciling any
// items whose op-log diverged since the last sync. See docs/design/
// git-backlog.md, "Sync & conflict resolution".
func Sync(remote string) (*SyncReport, error) {
	var err error
	if remote == "" {
		remote, err = defaultRemote()
		if err != nil {
			return nil, err
		}
	}

	remoteTrackingPrefix := fmt.Sprintf("refs/remotes/%s/backlog/", remote)
	fetchSpec := fmt.Sprintf("refs/backlog/*:%s*", remoteTrackingPrefix)
	if err := gitx.Fetch(remote, fetchSpec); err != nil {
		return nil, fmt.Errorf("fetching from %s: %w", remote, err)
	}

	ids, err := unionIDs(remoteTrackingPrefix)
	if err != nil {
		return nil, err
	}

	report := &SyncReport{Remote: remote}
	for _, id := range ids {
		localRef := RefFor(id)
		remoteRef := remoteTrackingPrefix + id
		localHash, haveLocal := resolveOrEmpty(localRef)
		remoteHash, haveRemote := resolveOrEmpty(remoteRef)

		switch {
		case !haveRemote:
			// Local-only: nothing to reconcile, sync will push it.
		case !haveLocal:
			if err := gitx.UpdateRef(localRef, remoteHash, ""); err != nil {
				return nil, fmt.Errorf("adopting %s: %w", ShortID(id), err)
			}
			report.Adopted = append(report.Adopted, id)
		case localHash == remoteHash:
			// Already in sync.
		case gitx.IsAncestor(localHash, remoteHash):
			if err := gitx.UpdateRef(localRef, remoteHash, localHash); err != nil {
				return nil, fmt.Errorf("fast-forwarding %s: %w", ShortID(id), err)
			}
			report.FastForwarded = append(report.FastForwarded, id)
		case gitx.IsAncestor(remoteHash, localHash):
			// Local is already ahead; sync will push it.
		default:
			merged, err := mergeItem(localHash, remoteHash)
			if err != nil {
				return nil, fmt.Errorf("reconciling %s: %w", ShortID(id), err)
			}
			if err := gitx.UpdateRef(localRef, merged, localHash); err != nil {
				return nil, fmt.Errorf("recording merge of %s: %w", ShortID(id), err)
			}
			report.Merged = append(report.Merged, id)
		}
	}

	if err := gitx.Push(remote, backlogRefspec); err != nil {
		return nil, fmt.Errorf("pushing to %s: %w", remote, err)
	}
	return report, nil
}

func defaultRemote() (string, error) {
	remotes, err := gitx.Remotes()
	if err != nil {
		return "", err
	}
	if len(remotes) == 0 {
		return "", fmt.Errorf("no git remotes configured")
	}
	for _, r := range remotes {
		if r == "origin" {
			return "origin", nil
		}
	}
	if len(remotes) == 1 {
		return remotes[0], nil
	}
	return "", fmt.Errorf("multiple remotes configured (%s); specify one with --remote", strings.Join(remotes, ", "))
}

func unionIDs(remoteTrackingPrefix string) ([]string, error) {
	localRefs, err := gitx.ForEachRef(refNamespace)
	if err != nil {
		return nil, err
	}
	remoteRefs, err := gitx.ForEachRef(remoteTrackingPrefix)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(localRefs)+len(remoteRefs))
	for _, r := range localRefs {
		set[IDFromRef(r.Ref)] = true
	}
	for _, r := range remoteRefs {
		set[strings.TrimPrefix(r.Ref, remoteTrackingPrefix)] = true
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func resolveOrEmpty(ref string) (string, bool) {
	if !gitx.RefExists(ref) {
		return "", false
	}
	hash, err := gitx.Run("rev-parse", ref)
	if err != nil {
		return "", false
	}
	return hash, true
}

// fieldTouch is one operation's change to a single field, used to pick a
// per-field winner when reconciling two divergent op-logs.
type fieldTouch struct {
	clock   int
	commit  string
	hash    string
	removed bool
}

// mergeItem reconciles two divergent op-log tips of the same item into a
// new merge commit. Per field, the winning value is whichever operation
// (on either side, since the shared base) that actually touched that field
// has the highest Lamport clock value, tiebroken by commit hash. This
// matches docs/design/git-backlog.md's "Sync & conflict resolution":
// operations on different fields both survive; operations on the same
// field resolve deterministically without wall-clock timestamps.
func mergeItem(local, remote string) (string, error) {
	base, err := gitx.MergeBase(local, remote)
	if err != nil {
		return "", fmt.Errorf("no common ancestor: %w", err)
	}
	localCommits, err := gitx.RevListExclude(local, base)
	if err != nil {
		return "", err
	}
	remoteCommits, err := gitx.RevListExclude(remote, base)
	if err != nil {
		return "", err
	}

	baseCommit, err := gitx.CatCommit(base)
	if err != nil {
		return "", err
	}
	baseEntries, err := gitx.LsTree(baseCommit.Tree)
	if err != nil {
		return "", err
	}
	merged := make(map[string]string, len(baseEntries))
	for _, e := range baseEntries {
		if e.Name == fieldClock {
			continue
		}
		merged[e.Name] = e.Hash
	}
	maxClock := parseClock(fieldValue(baseEntries, fieldClock))

	touches := make(map[string][]fieldTouch)
	for _, commits := range [][]string{localCommits, remoteCommits} {
		for _, c := range commits {
			ci, err := gitx.CatCommit(c)
			if err != nil {
				return "", err
			}
			if len(ci.Parents) == 0 {
				return "", fmt.Errorf("commit %s has no parent but is not the base", c[:12])
			}
			entries, err := gitx.LsTree(ci.Tree)
			if err != nil {
				return "", err
			}
			clock := parseClock(fieldValue(entries, fieldClock))
			if clock > maxClock {
				maxClock = clock
			}
			diffs, err := gitx.DiffTree(ci.Parents[0], c)
			if err != nil {
				return "", err
			}
			for _, d := range diffs {
				if d.Name == fieldClock {
					continue
				}
				touches[d.Name] = append(touches[d.Name], fieldTouch{
					clock:   clock,
					commit:  c,
					hash:    d.NewHash,
					removed: d.Status == 'D',
				})
			}
		}
	}

	for field, ts := range touches {
		best := ts[0]
		for _, t := range ts[1:] {
			if t.clock > best.clock || (t.clock == best.clock && t.commit > best.commit) {
				best = t
			}
		}
		if best.removed {
			delete(merged, field)
		} else {
			merged[field] = best.hash
		}
	}

	names := make([]string, 0, len(merged)+1)
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]gitx.TreeEntry, 0, len(names)+1)
	for _, name := range names {
		entries = append(entries, gitx.TreeEntry{Name: name, Hash: merged[name]})
	}
	clockHash, err := gitx.HashBlob(fmt.Sprintf("%d", maxClock+1))
	if err != nil {
		return "", err
	}
	entries = append(entries, gitx.TreeEntry{Name: fieldClock, Hash: clockHash})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	tree, err := gitx.MkTree(entries)
	if err != nil {
		return "", fmt.Errorf("building merge tree: %w", err)
	}
	message := fmt.Sprintf("merge: reconcile %d operation(s)", len(localCommits)+len(remoteCommits))
	return gitx.CommitTree(tree, []string{local, remote}, message)
}
