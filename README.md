# git-backlog

A git-native CLI for a small, personal/small-team backlog — deciding WHAT
to work on next. State is stored as real git objects under `refs/backlog/*`
(no SQLite, no markdown file, no external service) and synced via plain
`git push`/`fetch`.

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
| `all [--list <value>] [--priority <value>]` | List every item, grouped by list then priority, oldest-first within each group (empty lists shown as `(empty)`) |
| `show <id>` | Full item state plus its complete op-log history |
| `list <id> <backlog\|current\|closed>` | Move an item between lists (closing an item is just `list <id> closed`) |
| `priority <id> <p0\|p1\|p2\|none>` | Set or clear priority |
| `title <id> "<new title>"` | Rename an item |
| `sync [--remote <name>]` | Push/fetch `refs/backlog/*` against a remote, reconciling any items edited concurrently on both sides |

`<id>` accepts any unambiguous prefix of an item's id (shown by `all` and
`add` in git's usual auto-growing short form).

## Development

```
go build      # builds ./git-backlog (gitignored)
go vet ./...
go test ./...
```
