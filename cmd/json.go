package cmd

import (
	"encoding/json"
	"os"
	"time"

	"github.com/nosaka4i/git-backlog/internal/store"
)

// jsonOwner is an item's fixed create-commit author, as JSON.
type jsonOwner struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// jsonItem is an item's current state, as JSON. Priority is omitted
// entirely when unset, rather than emitted as "" or "none".
type jsonItem struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Track       string         `json:"track"`
	Priority    string         `json:"priority,omitempty"`
	Comment     string         `json:"comment,omitempty"`
	Owner       jsonOwner      `json:"owner"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Sync        *jsonSyncState `json:"sync,omitempty"`
}

func toJSONItem(it *store.Item) jsonItem {
	return jsonItem{
		ID:          it.ID,
		Title:       it.Title,
		Description: it.Description,
		Track:       string(it.Track),
		Priority:    string(it.Priority),
		Comment:     it.Comment,
		Owner:       jsonOwner{Name: it.OwnerName, Email: it.OwnerEmail},
		CreatedAt:   it.CreatedAt,
		UpdatedAt:   it.UpdatedAt,
	}
}

// jsonSyncState is an item's sync status, as JSON. nil (omitted from the
// object entirely) means no remote was configured/resolvable — distinct
// from status "not_synced", which means a remote exists but this item has
// no remote-tracking ref yet.
type jsonSyncState struct {
	Status   string `json:"status"`
	AheadBy  int    `json:"ahead_by,omitempty"`
	BehindBy int    `json:"behind_by,omitempty"`
}

func toJSONSyncState(s *store.ItemSyncState) *jsonSyncState {
	if s == nil {
		return nil
	}
	return &jsonSyncState{Status: string(s.Status), AheadBy: s.AheadBy, BehindBy: s.BehindBy}
}

// jsonChange is one field touched by an op-log entry.
type jsonChange struct {
	Field   string `json:"field"`
	Removed bool   `json:"removed,omitempty"`
	Value   string `json:"value,omitempty"`
}

// jsonOp is one op-log entry.
type jsonOp struct {
	Commit  string       `json:"commit"`
	Clock   int          `json:"clock"`
	Author  jsonOwner    `json:"author"`
	When    time.Time    `json:"when"`
	Changes []jsonChange `json:"changes"`
}

// jsonItemDetail is an item's full state plus its op-log history, as
// printed by `show --json`.
type jsonItemDetail struct {
	jsonItem
	Tip     string   `json:"tip"`
	History []jsonOp `json:"history"`
}

func toJSONHistory(history []store.OpRecord) []jsonOp {
	ops := make([]jsonOp, 0, len(history))
	for _, op := range history {
		ops = append(ops, jsonOp{
			Commit:  op.Commit,
			Clock:   op.Clock,
			Author:  jsonOwner{Name: op.AuthorName, Email: op.AuthorEmail},
			When:    op.When,
			Changes: toJSONChanges(op.Changes),
		})
	}
	return ops
}

// jsonHistoryEntry is one entry in `history --json`: a single op-log
// commit, tagged with which item it belongs to (an item's *current*
// title, not necessarily its title at the time of this op).
type jsonHistoryEntry struct {
	When    time.Time    `json:"when"`
	Commit  string       `json:"commit"`
	ItemID  string       `json:"item_id"`
	Title   string       `json:"title"`
	Author  jsonOwner    `json:"author"`
	Changes []jsonChange `json:"changes"`
}

// jsonCommentEntry is one comment-field edit in an item's op-log, as
// printed by `comment show --json`.
type jsonCommentEntry struct {
	When    time.Time `json:"when"`
	Commit  string    `json:"commit"`
	Author  jsonOwner `json:"author"`
	Text    string    `json:"text,omitempty"`
	Cleared bool      `json:"cleared,omitempty"`
}

func toJSONChanges(changes []store.FieldChange) []jsonChange {
	out := make([]jsonChange, 0, len(changes))
	for _, ch := range changes {
		out = append(out, jsonChange{Field: ch.Field, Removed: ch.Removed, Value: ch.Value})
	}
	return out
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
