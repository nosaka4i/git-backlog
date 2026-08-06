package cmd

import (
	"strings"

	"github.com/nosaka4i/git-backlog/internal/store"
)

// actionLine renders one field change as a short, human verb phrase.
// Shared by `show <id>`'s per-item history and the global `history`
// command, so both read the same way.
func actionLine(field string, removed bool, value string) string {
	switch field {
	case "list", "track":
		// "list" is the field's legacy on-disk name — still literally
		// present in the immutable history of any op recorded before the
		// track rename (see fieldList's doc comment in internal/store/
		// types.go). Both render the same way.
		return "Moved to " + value
	case "priority":
		if removed {
			return "Cleared priority"
		}
		return "Updated priority to " + value
	case "title":
		return "Renamed item"
	case "comment":
		if removed {
			return "Cleared comment"
		}
		return "Updated comment"
	case "description":
		if removed {
			return "Cleared description"
		}
		return "Updated description"
	default:
		return field + ": " + value
	}
}

// opActionLines renders every field an operation touched as one line each.
// A create op sets several fields at once but is one atomic action, so it
// collapses to a single "Added item" line instead of one line per field.
func opActionLines(op store.OpRecord) []string {
	if isCreateOp(op) {
		return []string{"Added item"}
	}
	if isMigrateTrackOp(op) {
		return []string{"Migrated the track field's storage format (no value change)"}
	}
	lines := make([]string, 0, len(op.Changes))
	for _, ch := range op.Changes {
		lines = append(lines, actionLine(ch.Field, ch.Removed, ch.Value))
	}
	return lines
}

func isCreateOp(op store.OpRecord) bool {
	return strings.HasPrefix(op.Message, "add:")
}

// isMigrateTrackOp identifies MigrateTrackField's one-time op (see
// internal/store/item.go): it always removes "list" and adds "track" with
// the same value in a single commit, which would otherwise render as a
// confusing "Moved to <value>" (from the "list" removal, value blank) plus
// a raw "track: <value>" line — collapsing it to one clear line is more
// honest about what actually happened (a storage-format change, not a real
// move).
func isMigrateTrackOp(op store.OpRecord) bool {
	return strings.HasPrefix(op.Message, "migrate:")
}
