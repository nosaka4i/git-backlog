package store

import (
	"fmt"
	"time"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

// FieldChange is one field touched by an operation.
type FieldChange struct {
	Field string
	// Removed is true when the operation cleared the field (e.g.
	// "priority none"); Value is meaningless in that case.
	Removed bool
	Value   string
}

// OpRecord is one entry in an item's op-log, oldest history first.
type OpRecord struct {
	Commit      string
	Clock       int
	AuthorName  string
	AuthorEmail string
	When        time.Time
	Message     string
	Changes     []FieldChange
}

// History returns an item's full op-log, oldest first.
func History(idPrefix string) ([]OpRecord, error) {
	id, err := ResolveID(idPrefix)
	if err != nil {
		return nil, err
	}
	tip, err := gitx.Run("rev-parse", RefFor(id))
	if err != nil {
		return nil, err
	}
	commits, err := gitx.RevListReverse(tip)
	if err != nil {
		return nil, err
	}
	records := make([]OpRecord, 0, len(commits))
	for _, c := range commits {
		ci, err := gitx.CatCommit(c)
		if err != nil {
			return nil, err
		}
		entries, err := gitx.LsTree(ci.Tree)
		if err != nil {
			return nil, err
		}
		var changes []FieldChange
		if len(ci.Parents) == 0 {
			for _, e := range entries {
				if e.Name == fieldClock {
					continue
				}
				val, err := gitx.CatBlob(e.Hash)
				if err != nil {
					return nil, fmt.Errorf("reading %s blob %s: %w", e.Name, e.Hash, err)
				}
				changes = append(changes, FieldChange{Field: e.Name, Value: val})
			}
		} else {
			diffs, err := gitx.DiffTree(ci.Parents[0], c)
			if err != nil {
				return nil, err
			}
			for _, d := range diffs {
				if d.Name == fieldClock {
					continue
				}
				if d.Status == 'D' {
					changes = append(changes, FieldChange{Field: d.Name, Removed: true})
					continue
				}
				val, err := gitx.CatBlob(d.NewHash)
				if err != nil {
					return nil, fmt.Errorf("reading %s blob %s: %w", d.Name, d.NewHash, err)
				}
				changes = append(changes, FieldChange{Field: d.Name, Value: val})
			}
		}
		clockRaw, err := fieldValue(entries, fieldClock)
		if err != nil {
			return nil, err
		}
		records = append(records, OpRecord{
			Commit:      c,
			Clock:       parseClock(clockRaw),
			AuthorName:  ci.AuthorName,
			AuthorEmail: ci.AuthorEmail,
			When:        parseGitDate(ci.AuthorDate),
			Message:     firstLine(c),
			Changes:     changes,
		})
	}
	return records, nil
}

func firstLine(commit string) string {
	msg, err := gitx.Run("show", "-s", "--format=%s", commit)
	if err != nil {
		return ""
	}
	return msg
}
