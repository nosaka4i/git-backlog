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
git backlog all
git backlog list <id> current
git backlog priority <id> p0
git backlog show <id>
git backlog sync
```

## Commands

| Command | Effect |
|---|---|
| `init` | Start tracking backlog items in the current repo |
| `add "<title>" [--list backlog\|current\|closed] [--priority p0\|p1\|p2]` | Create an item (defaults to `--list backlog`, unset priority) |
| `all [--list <value>] [--priority <value>] [--closed-limit N] [--json]` | List every item, grouped by list then priority, oldest-first within each group (empty lists shown as `(empty)`) |
| `show <id> [--json]` | Full item state plus its complete op-log history |
| `history [--list <value>] [--priority <value>] [--json]` | Chronological activity trail across every item, newest first |
| `list <id> <backlog\|current\|closed>` | Move an item between lists (closing an item is just `list <id> closed`) |
| `priority <id> <p0\|p1\|p2\|none>` | Set or clear priority |
| `title <id> "<new title>"` | Rename an item |
| `sync [--remote <name>]` | Push/fetch `refs/backlog/*` against a remote, reconciling any items edited concurrently on both sides |
| `version` | Print the git-backlog version |

`<id>` accepts any unambiguous prefix of an item's id (shown by `all` and
`add` in git's usual auto-growing short form).

`--json` on `all`/`show` prints machine-readable output instead (a JSON
array of items for `all`, one item plus its full op-log history for
`show`) — same filtering, same sort order, unset priority simply omitted
from the object rather than printed as a placeholder string.

A successful backlog accumulates `closed` items forever, so `all` caps the
closed section to the 10 most recently updated items by default (a
`... and N more` note shows how many are hidden). `--closed-limit 0`
removes the cap; `--list closed` also shows the full closed list, since
asking for it specifically is already an explicit, narrow request.

`history` flattens every item's op-log into one chronological feed —
same `--list`/`--priority` filters as `all`, applied to items' *current*
state (so `--list current` shows the full history of everything presently
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
