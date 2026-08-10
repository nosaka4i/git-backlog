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
| `list [--track <value>] [--priority <value>] [--label <value>]... [--closed-limit N] [--no-pager] [--json]` | List every item, grouped by track then priority, most-recently-updated-first within each group (empty tracks shown as `(empty)`); shows sync status when a remote's configured |
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

A successful backlog accumulates `closed` items forever, so `list` caps the
closed section to the 10 most recently updated items by default (a
`... and N more` note shows how many are hidden). `--closed-limit 0`
removes the cap; `--track closed` also shows the full closed list, since
asking for it specifically is already an explicit, narrow request.

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
