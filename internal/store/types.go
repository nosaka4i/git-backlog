// Package store implements the git-backlog storage model: each item is an
// append-only op-log of git commits under refs/backlog/<create-commit-hash>,
// with each commit's tree a full snapshot of the item's current field
// values. See docs/design/git-backlog.md.
package store

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nosaka4i/git-backlog/internal/gitx"
)

// Track is the track an item currently belongs to.
type Track string

const (
	TrackBacklog Track = "backlog"
	TrackCurrent Track = "current"
	TrackClosed  Track = "closed"
)

// Rank orders tracks for grouped display: current, backlog, then closed —
// what you're actively doing first, what's queued next, what's done last.
func (b Track) Rank() int {
	switch b {
	case TrackCurrent:
		return 0
	case TrackBacklog:
		return 1
	case TrackClosed:
		return 2
	default:
		return 3
	}
}

// ParseTrack validates a user-supplied track value.
func ParseTrack(s string) (Track, error) {
	switch Track(s) {
	case TrackBacklog, TrackCurrent, TrackClosed:
		return Track(s), nil
	default:
		return "", fmt.Errorf("invalid track %q (want backlog, current, or closed)", s)
	}
}

// Priority is an item's priority tier, or "" if unset.
type Priority string

const (
	PriorityNone Priority = ""
	PriorityP0   Priority = "p0"
	PriorityP1   Priority = "p1"
	PriorityP2   Priority = "p2"
)

// ParsePriority validates a user-supplied priority value. "none" clears it.
func ParsePriority(s string) (Priority, error) {
	switch s {
	case "none":
		return PriorityNone, nil
	case "p0", "p1", "p2":
		return Priority(s), nil
	default:
		return "", fmt.Errorf("invalid priority %q (want p0, p1, p2, or none)", s)
	}
}

// Rank orders priorities for grouped display: p0, p1, p2, then unset.
func (p Priority) Rank() int {
	switch p {
	case PriorityP0:
		return 0
	case PriorityP1:
		return 1
	case PriorityP2:
		return 2
	default:
		return 3
	}
}

// Tree entry field names. "clock" is an internal Lamport counter, not part
// of the user-visible schema.
//
// fieldList/fieldTrack: the track field was originally named "list" on
// disk. Renaming it in place would corrupt history — these are full
// snapshot trees, not diffs, so the very next edit to an item still on
// "list" would look like it cleared "list" and set a brand new "track"
// field out of nowhere. Instead: CreateItem writes fieldTrack for every
// new item; itemFromEntries reads fieldTrack and falls back to fieldList
// for items that haven't been migrated; MigrateTrackField does that
// migration explicitly, as one visible op per item (see item.go), rather
// than silently on next edit. fieldList stays defined permanently so
// already-recorded op-log history (which literally has "list" field
// changes in it, forever, since those commits are immutable) keeps
// rendering correctly.
const (
	fieldTitle       = "title"
	fieldList        = "list"
	fieldTrack       = "track"
	fieldPriority    = "priority"
	fieldComment     = "comment"
	fieldDescription = "description"
	fieldClock       = "clock"
)

// Identity overrides the git author/committer for a single operation,
// instead of using the ambient git config — for recording an edit as made
// by an automation/agent rather than whoever's git identity is configured
// locally. See docs/design/git-backlog.md's "Agent identity" section.
type Identity struct {
	Name  string
	Email string
}

const refNamespace = "refs/backlog/"

// RefFor returns the ref name for an item's create-commit hash.
func RefFor(id string) string {
	return refNamespace + id
}

// IDFromRef extracts the create-commit hash from an item ref name.
func IDFromRef(ref string) string {
	return strings.TrimPrefix(ref, refNamespace)
}

// Item is an item's current, reconstructed state.
type Item struct {
	ID          string // create-commit hash; permanent
	Ref         string
	Tip         string // current op-log commit hash
	Title       string
	Track       Track
	Priority    Priority
	Description string // freeform, optional, permanent (single current value, replace-on-edit like Title); "" means never set
	Comment     string // freeform, optional; "" means never set
	OwnerName   string
	OwnerEmail  string
	CreatedAt   time.Time
	UpdatedAt   time.Time // when the tip op-log commit was made
	Clock       int
}

// ShortID returns git's auto-growing unambiguous short form of the id.
func ShortID(id string) string {
	// Best-effort: fall back to the full id if git can't shorten it for
	// some reason (never expected in practice).
	if short, err := gitx.Run("rev-parse", "--short", id); err == nil {
		return short
	}
	return id
}

func parseClock(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return n
}

// parseGitDate parses the raw "<unix-seconds> <tz-offset>" format git
// stores in commit author/committer lines.
func parseGitDate(raw string) time.Time {
	fields := strings.Fields(raw)
	if len(fields) != 2 {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return time.Time{}
	}
	loc := time.UTC
	if len(fields[1]) == 5 {
		sign := 1
		if fields[1][0] == '-' {
			sign = -1
		}
		hh, errH := strconv.Atoi(fields[1][1:3])
		mm, errM := strconv.Atoi(fields[1][3:5])
		if errH == nil && errM == nil {
			loc = time.FixedZone(fields[1], sign*(hh*3600+mm*60))
		}
	}
	return time.Unix(sec, 0).In(loc)
}
