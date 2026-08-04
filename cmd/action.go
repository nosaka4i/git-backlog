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
	case "list":
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
	lines := make([]string, 0, len(op.Changes))
	for _, ch := range op.Changes {
		lines = append(lines, actionLine(ch.Field, ch.Removed, ch.Value))
	}
	return lines
}

func isCreateOp(op store.OpRecord) bool {
	return strings.HasPrefix(op.Message, "add:")
}
