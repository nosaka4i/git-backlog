package store

import (
	"fmt"
	"sort"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

// CreateItem records a new item's create operation and returns it.
// identity is nil to author it from the ambient git config, or an
// override — see Identity. Because the create commit's author becomes the
// item's permanent owner (no reassignment, ever — see Schema in the
// design doc), identity here doesn't just attribute one op like it does
// for the other Set* functions: it decides who owns the item forever.
func CreateItem(title string, list List, priority Priority, identity *Identity) (*Item, error) {
	entries, err := snapshotEntries(map[string]*string{
		fieldTitle:    &title,
		fieldList:     strPtr(string(list)),
		fieldPriority: priorityPtr(priority),
	}, nil, 0)
	if err != nil {
		return nil, err
	}
	tree, err := gitx.MkTree(entries)
	if err != nil {
		return nil, fmt.Errorf("building create tree: %w", err)
	}
	var commit string
	if identity != nil {
		commit, err = gitx.CommitTreeAs(tree, nil, "add: "+title, identity.Name, identity.Email)
	} else {
		commit, err = gitx.CommitTree(tree, nil, "add: "+title)
	}
	if err != nil {
		return nil, fmt.Errorf("creating item: %w", err)
	}
	if err := gitx.UpdateRef(RefFor(commit), commit, ""); err != nil {
		return nil, fmt.Errorf("recording item ref: %w", err)
	}
	return LoadItem(commit)
}

// ResolveID resolves a (possibly short) id prefix to the full create-commit
// hash of an existing item, erroring if it doesn't name a backlog item.
func ResolveID(idPrefix string) (string, error) {
	full, err := gitx.ResolveCommit(idPrefix)
	if err != nil {
		return "", fmt.Errorf("no item matches %q", idPrefix)
	}
	if !gitx.RefExists(RefFor(full)) {
		return "", fmt.Errorf("%q is not a backlog item id", idPrefix)
	}
	return full, nil
}

// LoadItem reconstructs an item's current state.
func LoadItem(idPrefix string) (*Item, error) {
	id, err := ResolveID(idPrefix)
	if err != nil {
		return nil, err
	}
	ref := RefFor(id)
	tip, err := gitx.Run("rev-parse", ref)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ref, err)
	}
	tipCommit, err := gitx.CatCommit(tip)
	if err != nil {
		return nil, err
	}
	entries, err := gitx.LsTree(tipCommit.Tree)
	if err != nil {
		return nil, err
	}
	createCommit := tipCommit
	if tip != id {
		createCommit, err = gitx.CatCommit(id)
		if err != nil {
			return nil, err
		}
	}
	return itemFromEntries(id, ref, tip, entries, tipCommit, createCommit)
}

// AllItems loads every tracked item. Order is unspecified; callers sort for
// display.
func AllItems() ([]*Item, error) {
	refs, err := gitx.ForEachRef(refNamespace)
	if err != nil {
		return nil, err
	}
	items := make([]*Item, 0, len(refs))
	for _, r := range refs {
		id := IDFromRef(r.Ref)
		tipCommit, err := gitx.CatCommit(r.Hash)
		if err != nil {
			return nil, err
		}
		entries, err := gitx.LsTree(tipCommit.Tree)
		if err != nil {
			return nil, err
		}
		createCommit := tipCommit
		if r.Hash != id {
			createCommit, err = gitx.CatCommit(id)
			if err != nil {
				return nil, err
			}
		}
		item, err := itemFromEntries(id, r.Ref, r.Hash, entries, tipCommit, createCommit)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// SetList records a list-change operation. identity is nil to use the
// ambient git config, or an override (e.g. for an agent acting on
// someone's behalf) — see Identity.
func SetList(idPrefix string, list List, identity *Identity) (*Item, error) {
	return applyOp(idPrefix, map[string]*string{fieldList: strPtr(string(list))}, "list: "+string(list), identity)
}

// SetPriority records a priority-change operation. priority == PriorityNone
// clears it. identity is nil to use the ambient git config, or an override
// — see Identity.
func SetPriority(idPrefix string, priority Priority, identity *Identity) (*Item, error) {
	msg := "priority: none"
	if priority != PriorityNone {
		msg = "priority: " + string(priority)
	}
	return applyOp(idPrefix, map[string]*string{fieldPriority: priorityPtr(priority)}, msg, identity)
}

// SetTitle records a title-edit operation. identity is nil to use the
// ambient git config, or an override — see Identity.
func SetTitle(idPrefix string, title string, identity *Identity) (*Item, error) {
	return applyOp(idPrefix, map[string]*string{fieldTitle: &title}, "edit: "+title, identity)
}

// SetComment records a comment-edit operation. The new text fully replaces
// any previous comment, same as SetTitle — the prior text remains visible
// via the item's op-log history. identity is nil to use the ambient git
// config, or an override — see Identity.
func SetComment(idPrefix string, comment string, identity *Identity) (*Item, error) {
	return applyOp(idPrefix, map[string]*string{fieldComment: &comment}, "comment: "+comment, identity)
}

// applyOp writes a new op-log commit on top of idPrefix's current tip,
// changing the given fields (nil value removes the field) and returns the
// item's new state. identity is nil to author the commit from the ambient
// git config, or a non-nil override to author/commit it as that identity
// instead (e.g. an automation/agent acting on someone's behalf).
func applyOp(idPrefix string, changes map[string]*string, message string, identity *Identity) (*Item, error) {
	id, err := ResolveID(idPrefix)
	if err != nil {
		return nil, err
	}
	ref := RefFor(id)
	tip, err := gitx.Run("rev-parse", ref)
	if err != nil {
		return nil, err
	}
	tipCommit, err := gitx.CatCommit(tip)
	if err != nil {
		return nil, err
	}
	current, err := gitx.LsTree(tipCommit.Tree)
	if err != nil {
		return nil, err
	}
	currentClockRaw, err := fieldValue(current, fieldClock)
	if err != nil {
		return nil, err
	}
	currentClock := parseClock(currentClockRaw)
	entries, err := snapshotEntries(changes, current, currentClock+1)
	if err != nil {
		return nil, err
	}
	tree, err := gitx.MkTree(entries)
	if err != nil {
		return nil, fmt.Errorf("building op tree: %w", err)
	}
	var commit string
	if identity != nil {
		commit, err = gitx.CommitTreeAs(tree, []string{tip}, message, identity.Name, identity.Email)
	} else {
		commit, err = gitx.CommitTree(tree, []string{tip}, message)
	}
	if err != nil {
		return nil, fmt.Errorf("recording operation: %w", err)
	}
	if err := gitx.UpdateRef(ref, commit, tip); err != nil {
		return nil, fmt.Errorf("advancing %s (concurrent local edit?): %w", ref, err)
	}
	return LoadItem(id)
}

// snapshotEntries builds the full tree entry list for a new op: start from
// base (the previous snapshot, or nil for a create op), apply changes (nil
// value = remove that field), and set the clock entry. Fields not in
// changes keep their existing blob hash unchanged, so unmodified fields
// reuse the already-existing blob (ordinary git tree-diffing efficiency).
func snapshotEntries(changes map[string]*string, base []gitx.TreeEntry, clock int) ([]gitx.TreeEntry, error) {
	m := make(map[string]string, len(base)+len(changes)+1)
	for _, e := range base {
		m[e.Name] = e.Hash
	}
	for name, val := range changes {
		if val == nil || *val == "" {
			delete(m, name)
			continue
		}
		hash, err := gitx.HashBlob(*val)
		if err != nil {
			return nil, fmt.Errorf("writing %s blob: %w", name, err)
		}
		m[name] = hash
	}
	clockHash, err := gitx.HashBlob(fmt.Sprintf("%d", clock))
	if err != nil {
		return nil, fmt.Errorf("writing clock blob: %w", err)
	}
	m[fieldClock] = clockHash

	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]gitx.TreeEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, gitx.TreeEntry{Name: name, Hash: m[name]})
	}
	return entries, nil
}

func itemFromEntries(id, ref, tip string, entries []gitx.TreeEntry, tipCommit, createCommit *gitx.CommitInfo) (*Item, error) {
	title, err := fieldValue(entries, fieldTitle)
	if err != nil {
		return nil, fmt.Errorf("item %s: %w", id, err)
	}
	if title == "" {
		return nil, fmt.Errorf("item %s has no title (corrupt?)", id)
	}
	priorityRaw, err := fieldValue(entries, fieldPriority)
	if err != nil {
		return nil, fmt.Errorf("item %s: %w", id, err)
	}
	listRaw, err := fieldValue(entries, fieldList)
	if err != nil {
		return nil, fmt.Errorf("item %s: %w", id, err)
	}
	comment, err := fieldValue(entries, fieldComment)
	if err != nil {
		return nil, fmt.Errorf("item %s: %w", id, err)
	}
	clockRaw, err := fieldValue(entries, fieldClock)
	if err != nil {
		return nil, fmt.Errorf("item %s: %w", id, err)
	}
	return &Item{
		ID:         id,
		Ref:        ref,
		Tip:        tip,
		Title:      title,
		List:       List(listRaw),
		Priority:   Priority(priorityRaw),
		Comment:    comment,
		OwnerName:  createCommit.AuthorName,
		OwnerEmail: createCommit.AuthorEmail,
		CreatedAt:  parseGitDate(createCommit.AuthorDate),
		UpdatedAt:  parseGitDate(tipCommit.AuthorDate),
		Clock:      parseClock(clockRaw),
	}, nil
}

// fieldValue looks up a named tree entry's blob content. Returns ("", nil)
// when the field is genuinely absent (no entry with that name) — distinct
// from ("", err) when the entry exists but its blob failed to read, which
// used to be silently collapsed into the same empty-string result. That
// masked real transient git errors (e.g. a concurrent gc/repack window)
// as if the field had simply never been set, surfacing as a misleading
// "has no title (corrupt?)" for an item that was actually fine — confirmed
// 2026-07-31 by reading the same blob directly with `git cat-file -p`
// moments later and getting valid content back.
func fieldValue(entries []gitx.TreeEntry, name string) (string, error) {
	for _, e := range entries {
		if e.Name == name {
			blob, err := gitx.CatBlob(e.Hash)
			if err != nil {
				return "", fmt.Errorf("reading %s blob %s: %w", name, e.Hash, err)
			}
			return blob, nil
		}
	}
	return "", nil
}

func strPtr(s string) *string { return &s }

func priorityPtr(p Priority) *string {
	s := string(p)
	return &s
}
