# git-backlog

A lightweight and opinionated backlog CLI tool that leverages git's distributed source management mechanics.  Ideal for small teams of
developers, product managers, or even AI coding agents.  State is stored as real git objects under `refs/backlog/*` and synced via
plain `git push`/`fetch`.  Keep track of items, set priority, shift them across your backlog, current, or closed lists.

See [`docs/design/git-backlog.md`](docs/design/git-backlog.md) for the full
design: schema, storage model, and how `sync` reconciles concurrent edits
from different clones.

## Install

```
go install github.com/nosaka4i/git-backlog@latest
```

Requires `$GOPATH/bin` (or `$GOBIN`) on your `PATH` — git finds
`git-backlog` there and exposes it as `git backlog`.

## Quickstart

```
git backlog init
git backlog add "fix flaky test" --priority p1
git backlog add "write docs"
git backlog list
git backlog move <id> current
git backlog priority <id> p0
git backlog show <id>
git backlog sync
```

## Commands

| Command | Effect |
|---|---|
| `init` | Start tracking backlog items in the current repo |
| `add "<title>" [--track backlog\|current\|closed] [--priority p0\|p1\|p2] [--description "<text>"] [--label <name>]... [--as-agent]` | Create an item (defaults to `--track backlog`, unset priority) |
| `list [--track <value>] [--priority <value>] [--label <value>]... [--no-pager] [--json]` | With no `--track`: a capped dashboard (3 items/track, priority-first). With `--track`: that one track in full, grouped by priority, uncapped, paged |
| `show <id> [--json]` | Full item state plus its complete op-log history (newest first), including sync status |
| `history [--track <value>] [--priority <value>] [--label <value>]... [--json] [--no-pager]` | Chronological activity trail across every item, newest first |
| `move <id> <backlog\|current\|closed> [--as-agent]` | Move an item to a different track (closing an item is just `move <id> closed`) |
| `priority <id> <p0\|p1\|p2\|none> [--as-agent]` | Set or clear priority |
| `title <id> "<new title>" [--as-agent]` | Rename an item |
| `describe <id> "<text>" [--as-agent]` | Set an item's description (empty string clears it) |
| `comment <id> "<text>" [--as-agent]` | Set an item's comment (empty string clears it) |
| `comment show <id> [--json]` | Show an item's comments, newest first |
| `label <id> <name>... [--remove] [--as-agent]` | Attach labels to an item, or `--remove` to detach them (flat tags for grouping, e.g. a sprint) |
| `label [--json]` | With no args, list every label in use with how many items carry it, most-used first (like `git tag`) |
| `sync [--remote <name>]` | Push/fetch `refs/backlog/*` against a remote, reconciling any items edited concurrently on both sides |
| `version` | Print the git-backlog version |

`<id>` accepts any unambiguous prefix of an item's id (shown by `list` and
`add` in git's usual auto-growing short form).

`--json` on `list`/`show` prints machine-readable output instead (a JSON
array of items for `list`, one item plus its full op-log history for
`show`) — same filtering, same sort order, unset priority simply omitted
from the object rather than printed as a placeholder string.

A backlog accumulates items forever, so bare `list` (no `--track`) is a
dashboard, not a dump: each track shows a `(<N> total[, <priority
breakdown>])` line and its 3 highest-priority items (most-recently-updated
breaking ties), with an `... and N more` note when there's more to see.
The priority breakdown is left off once a track is small enough that
there's nothing left to summarize. Naming a track explicitly — `--track
backlog` — is a deliberate, narrow request, so it bypasses the dashboard
entirely: full track, uncapped, grouped by priority, paged. See
[List: dashboard vs. a track in full](#list-dashboard-vs-a-track-in-full)
below for the full shape of both views.

### Command reference

Full flag-by-flag detail for every command; the table above is the quick
index.

**`init`**
```
git backlog init
```
Starts tracking backlog items in the current repo (sets `backlog.init` in
local git config, i.e. `.git/config`, never committed). Safe to re-run —
no-ops with "backlog already initialized". No flags.

**`add`**
```
git backlog add "<title>" [--track backlog|current|closed] [--priority p0|p1|p2]
                           [--description "<text>"] [--label <name>]... [--as-agent]
```
Creates a new item and prints its short id.
- `--track` — starting track (default `backlog`)
- `--priority` — `p0`/`p1`/`p2` (default: unset)
- `--description "<text>"` — set the description at creation time; equivalent to `add` then `describe`
- `--label <name>` — attach a label at creation; repeatable
- `--as-agent` — see [Agent identity](#agent-identity); for `add` specifically, the agent becomes the item's permanent owner

**`list`**
```
git backlog list [--track <value>] [--priority <value>[,<value>...]] [--label <value>]...
                  [--no-pager] [--json]
```
With no `--track`: a capped dashboard, one section per track, 3
highest-priority items each. With `--track`: that one track in full,
grouped by priority sub-headers, uncapped, paged — today's original
`list` behavior. Both show sync status when a remote's configured.
- `--track backlog|current|closed` — switch to that one track's full, uncapped view
- `--priority p0,p1,...` — comma-separated list of `p0`/`p1`/`p2`/`none`; matches items at *any* of the given values (OR, unlike `--label`'s AND)
- `--label <name>` — narrow to items carrying all of these labels; repeatable
- `--no-pager` — never pipe through a pager, even on a terminal
- `--json` — machine-readable output instead (always the filtered-and-capped set, same as the dashboard, unless `--track` is given)

**`show`**
```
git backlog show <id> [--json]
```
Prints an item's full state (id, title, description, track, priority,
labels, comment, owner, timestamps, sync status) plus its complete op-log
history, newest first.
- `--json` — machine-readable output, including the full history array

**`history`**
```
git backlog history [--track <value>] [--priority <value>[,<value>...]] [--label <value>]...
                     [--no-pager] [--json]
```
Chronological activity trail across every item, newest first. Filters
match items' *current* state, same semantics as `list`'s filters.
- `--track`, `--priority`, `--label` — same as `list`
- `--no-pager`, `--json` — same as `list`

**`move`**
```
git backlog move <id> <backlog|current|closed> [--as-agent]
```
Moves an item to a different track (closing an item is just
`move <id> closed`).
- `--as-agent` — see [Agent identity](#agent-identity)

**`priority`**
```
git backlog priority <id> <p0|p1|p2|none> [--as-agent]
```
Sets an item's priority, or clears it with `none`.
- `--as-agent` — see [Agent identity](#agent-identity)

**`title`**
```
git backlog title <id> "<new title>" [--as-agent]
```
Renames an item.
- `--as-agent` — see [Agent identity](#agent-identity)

**`describe`**
```
git backlog describe <id> "<text>" [--as-agent]
```
Sets an item's description; an empty string clears it. See the
description-vs-comment note below for how this differs from `comment`.
- `--as-agent` — see [Agent identity](#agent-identity)

**`comment`**
```
git backlog comment <id> "<text>" [--as-agent]
```
Sets an item's comment; an empty string clears it. Each edit replaces the
value — use `comment show` to read past comment text.
- `--as-agent` — see [Agent identity](#agent-identity)

**`comment show`**
```
git backlog comment show <id> [--json]
```
Shows an item's comments, newest first (each with timestamp, commit,
author, and text; cleared entries print `(cleared)`).
- `--json` — machine-readable output

**`label`**
```
git backlog label <id> <name>... [--remove] [--as-agent]
```
Attaches one or more labels to an item; `--remove` detaches them instead.
Adding an already-present label (or removing an absent one) is a no-op —
no empty commit.
- `--remove` — detach the given labels instead of attaching them
- `--as-agent` — see [Agent identity](#agent-identity)

**`label` (no args)**
```
git backlog label [--json]
```
Lists every label currently in use, with how many items carry it,
most-used first (ties alphabetical) — the `git tag`-style bare-noun
roster.
- `--json` — machine-readable output (`{"label": ..., "count": ...}` per entry)

**`sync`**
```
git backlog sync [--remote <name>]
```
Pushes/fetches `refs/backlog/*` against a remote, reconciling any items
edited concurrently on both sides. Reports items adopted from the remote,
fast-forwarded, and merged.
- `--remote <name>` — remote to sync with (default: `origin`, or the only configured remote)

**`version`**
```
git backlog version
```
Prints the git-backlog version. No flags.

### List: dashboard vs. a track in full

Bare `list` (no `--track`) is a dashboard: a quick "what's the state of
things" glance, not a retrieval mechanism — that's what naming a track
explicitly is for. Each section shows a fixed 3 items (highest priority
first, ties broken by most-recently-updated), so the whole dashboard's
line count stays predictable no matter how many items exist or how
they're spread across priorities:

```
sync: 2 ahead, 1 behind, 3 not yet synced
last 24h: 2 new, 1 closed

current (1 total):

  p0 2c6334e  confidence scoring for deterministic  ↑1

backlog (42 total, 0 p0, 3 p1, 1 p2, 38 none):

  p1 a7fb723  Transcript/Actions view can show stale content after a…  (not synced)
  p1 b936bd7  things in a series are broken if multiple confirmations …
  p1 dd64ca3  compound statements latter half are being ignored.
  ... and 39 more (use --track backlog to see them all)

closed (18 total, 5 p0, 2 p1, 1 p2, 10 none):

  p0 0038514  whisper prompt bias  [reminder-mvp]
  p0 4b13782  add reminder due date
  p0 d3cc34e  add reminder notes
  ... and 15 more (use --track closed to see them all)
```

- `sync:` and `last 24h:` are always global — the whole repo's state, not
  just whatever's currently filtered into view (same as `sync:` already
  was). `last 24h` is a rolling 24-hour window (not a calendar day, which
  is meaningless across time zones), and only counts two events: items
  created and items moved to `closed`. Renames, priority/label/comment
  edits, and moves between `backlog`/`current` aren't counted — unlike
  "created" or "closed", they have no unambiguous meaning in aggregate
  ("10 priority changes" could mean anything). Prints `last 24h: none`
  when nothing happened in the window, rather than disappearing.
- The `(<N> total, ...)` breakdown always lists all four priority tiers,
  including zero counts, `npm audit`-style — no wondering whether `p0`
  was omitted or just empty. It's left off entirely once a track's total
  is already ≤ 3 (nothing left to summarize beyond what's on screen).
- `--priority`/`--label` filters still apply to the dashboard the same
  way they apply everywhere else; a track's cap and breakdown reflect the
  filtered set, not the track's unfiltered contents.

Naming a track explicitly bypasses all of this — `--track backlog` is a
deliberate, narrow request, so it gets the real thing: every item, grouped
under priority sub-headers, uncapped, paged (this is `list`'s original
behavior, unchanged):

```
backlog (42):
  p1:
    a7fb723  Transcript/Actions view can show stale content after a long …  (not synced)
    b936bd7  things in a series are broken if multiple confirmations requ…
    dd64ca3  compound statements latter half are being ignored.
  p2:
    7892993  FILTER_EXTRACTION_GUIDANCE thematic-keyword guardrail unreli…
  unprioritized:
    7105878  yes/no audio flush is slow  (not synced)
    ... (37 more unprioritized items, one line each)
```

`--json` follows the same rule: the filtered-and-capped set for bare
`list`, the filtered-and-uncapped set for `--track <value>` — same data
either output mode shows, just machine-readable.

### Labels

Labels are flat, free-form tags for grouping items — a sprint, an area, a
theme. Attach any number to an item and filter by them:

```
git backlog add "wire up auth" --label sprint-12 --label backend
git backlog label <id> sprint-12          # attach to an existing item
git backlog label <id> sprint-12 --remove # detach
git backlog list --label sprint-12        # everything in sprint-12
git backlog label                         # every label in use, with counts
```

`list --label` (and `history --label`) narrow to items carrying *all* of
the given labels. Labels show inline in `list` and on their own line in
`show`. For heavier grouping (milestones with due dates, key=value labels)
git-backlog stays out of the way on purpose — a label like `sprint-12`
already covers "bundle these and find them later."

### Sync status

When a remote's configured, `list` prints a `git status`-style summary at
the top ("2 ahead, 1 behind, 1 diverged, 3 not yet synced" — or "up to
date with origin" when there's nothing to report), and marks affected
items in place: `↑N` (N local commits not yet pushed), `↓N` (N remote
commits not yet pulled), `⇕` (diverged — the next `sync` will produce a
merge), `(not synced)` (no remote-tracking info for this item at all
yet). `show <id>` prints the same status as a `sync:` line. Both read
straight from git's local, already-fetched knowledge of the remote (same
as `git status`'s own ahead/behind) — neither command fetches, so this
reflects state as of the last `sync`, not a live check; run `sync` again
for fresher numbers. With no remote configured at all, both commands omit
sync status entirely rather than showing an empty/zeroed-out summary.

Want to see *what* changed on an ahead/behind/diverged item, not just that
it did? There's no `git backlog diff` — plain `git diff` against the two
refs already gives a clean field-level diff for free, once the noisy
`clock` field (bumped on every op, regardless of what else changed) is
excluded:
```
git diff refs/remotes/<remote>/backlog/<id> refs/backlog/<id> -- . ':!clock'
```

`comment` is a single freeform field, edited the same way as `title` (each
edit replaces the value; `comment <id> ""` clears it). `show <id>` and
`history` only render `Updated comment`/`Cleared comment` for it, same as
every other field — to read past comment text, use `comment show <id>`,
which walks the op-log and prints just the comment changes, newest first
(matching `history`'s convention).

`description` is also single freeform field, replace-on-edit just like
`title`/`comment` — but semantically different from `comment`: it's meant
to hold a single *permanent* explanation of what the item actually is
(only shown in `show`/`--json`, never in `list`'s compact view),
whereas `comment` is for ongoing, conversational back-and-forth (hence
`comment show`'s threaded history view). There's no `describe show` —
unlike a comment thread, a description has one current value worth
showing, not a log worth browsing. `add --description "<text>"` sets it
at creation time as a convenience — under the hood it's still two op-log
entries (create, then a description edit), the same as running `add` then
`describe` by hand, just in one command line.

### Moving items between repos

There's no dedicated import/export command — an item's id *is* its create
commit's hash, so its ref round-trips through plain git unchanged, history
and owner attribution included. Two procedures cover the common cases:

**Move a single item to a different repo** (e.g. it was accidentally
added to the wrong one):
```
# from the repo it's in now, push just that one ref to the right remote
git push <target-remote> refs/backlog/<id>:refs/backlog/<id>

# from a checkout of the target repo, pull it into refs/backlog
git fetch <that-remote> refs/backlog/<id>:refs/backlog/<id>
```
`list`/`show` pick it up immediately — no import step. Then remove it from
the original repo if it shouldn't stay there:
```
git push <original-remote> :refs/backlog/<id>   # delete on the remote
git update-ref -d refs/backlog/<id>             # delete the local copy
```

**Fork the whole backlog forward** (e.g. deprecating a repo in favor of
a new one): this is just `sync` against a new remote — it already pushes
every item, local-only ones included.
```
git remote add fork <new-repo-url>
git backlog sync --remote fork
```

### Agent identity

Every op-log commit's author comes from whatever git identity is ambient
when the command runs — same as any plain `git commit`. If a human and
an AI coding agent both drive `git backlog` from the same checkout, every
op either of them makes gets attributed to the *same* identity, so e.g.
back-and-forth `comment`s look like one person talking to themselves.

Fix: configure a separate identity for the agent (local repo config, no
git-backlog wrapper command needed — same as `user.name`/`user.email`):
```
git config backlog.agent.name "Claude"
git config backlog.agent.email "noreply@anthropic.com"
```
(`noreply@anthropic.com` is just an example — any name/email works, no
real account or mailbox required; see below.)

then pass `--as-agent` on `add`/`title`/`describe`/`priority`/`move`/`comment` to
record that specific operation under the agent's identity instead:
```
git backlog comment <id> "looks flaky under -race" --as-agent
```
Without `--as-agent`, nothing changes — commands attribute to the
ambient identity exactly as before. `--as-agent` errors out immediately
if `backlog.agent.name`/`backlog.agent.email` aren't configured, rather
than silently falling back to the ambient identity. It does **not**
require a separate GitHub account — git-backlog never touches GitHub's
API, only plain git commit authorship, which git already supports for
any two contributors regardless of whether their email belongs to a real
registered account anywhere — `sync`/`git push` never validate a commit's
author/committer against any account system, so an agent identity that
doesn't correspond to a real account anywhere never causes a push to
fail. (On GitHub specifically, an author email that doesn't match a
verified account just renders as plain gray-icon text instead of a
linked avatar in the web UI — cosmetic only, no functional difference.)

On `title`/`describe`/`priority`/`move`/`comment`, `--as-agent` only affects that
one op-log entry — it never touches an item's `owner` (fixed permanently
from the *create* commit, per the design doc's "no reassignment" rule).
`add` is the one exception: since the create commit's author **is** the
item's owner, `add --as-agent` makes the agent the item's permanent
owner — which is correct, not a special case, once you define `owner` as
"whoever physically ran the command," not "whoever thought of it." If you
ask the agent to file an item on your behalf and it runs
`add --as-agent`, the agent legitimately is who added it.

See [`docs/design/git-backlog.md`](docs/design/git-backlog.md)'s "Agent
identity" section for the full rationale, including why this can't be
applied retroactively to past comments.

On a terminal, `history` pipes its output through a pager, same as `git
log`/`git diff` (`$GIT_PAGER`, `core.pager`, then `$PAGER`, then `less` —
git's own precedence, so an existing git pager setup already applies);
`--no-pager` disables it, and piped/redirected/`--json` output is never
paged.

**Paging not kicking in even though the output is longer than one
screen?** Check `$GIT_PAGER` first — it wins over `core.pager` and
`$PAGER`, so `GIT_PAGER=cat` (or `=""`) anywhere in your environment
silently disables paging for `list`/`history` exactly as it would for
`git log`/`git diff`, with no warning. It's an easy thing to pick up
without meaning to: a `TERM` value with no local terminfo entry makes
`less` print `WARNING: terminal is not fully functional`, and setting
`GIT_PAGER=cat` in a shell rc file is a common way people silence that —
which then quietly kills paging everywhere, forever, even after the
terminfo gap gets fixed. If that's the case, fix the terminfo entry
instead of leaving paging off (`infocmp "$TERM"` tells you whether one's
already installed). Either way, `unset GIT_PAGER`/`export GIT_PAGER=`
in your rc file only affects *new* shells — an already-exported value
sticks in your current shell's environment until you `unset` it there
too or open a fresh terminal.

`history` flattens every item's op-log into one chronological feed —
same `--track`/`--priority` filters as `list`, applied to items' *current*
state (so `--track current` shows the full history of everything presently
in `current`, including its `Added item` entry from back when it may have
started in `backlog`). Each entry shows the item's current title (not its
title at the time of that operation) so entries stay identifiable even
after a rename.

## Development

```
go build      # builds ./git-backlog (gitignored)
go vet ./...
go test ./...
```
