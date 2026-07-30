# git-backlog — design

**Status: ready for implementation.** Open questions resolved; written to
hand off to a fresh coding session.

Companion docs (in the `nosaka` repo, where this project originated):
[brainstorm](https://github.com/nosaka4i/nosaka/blob/main/docs/brainstorm/git-backlog.md)
(architecture pivot + naming search),
[requirements](https://github.com/nosaka4i/nosaka/blob/main/docs/requirements/git-backlog.md)
(functional/non-functional requirements, non-goals). This doc is the next
stage: concrete enough to start implementing from.

## What this is

A git-native CLI for a small, personal/small-team backlog — deciding WHAT
to work on next, not tracking why or how (that's GitHub Issues or a
brainstorm/requirements/design doc pipeline). Invoked as a git subcommand
(`git backlog <verb>`), state stored as real git objects (no SQLite, no
markdown file, no external service), synced via plain `git push`/`fetch`.

Modeled on git-bug's architecture (verified via WebSearch, not git-bug's
code — git-bug is GPL-3.0, this is MIT), independently implemented in Go.

## Schema

Five fields, nothing else:

| Field      | Type                              | Notes                                    |
|------------|------------------------------------|-------------------------------------------|
| `title`    | string                             | brief, like a commit subject line          |
| `list`     | `backlog` \| `current` \| `closed` | named after Trello's terminology; this IS the field that tracks bucket membership, no separate "status" concept |
| `priority` | `p0` \| `p1` \| `p2` \| unset       | unset ≠ a literal "none" value — the entry is simply absent |
| `owner`    | git identity (name + email)        | read from the create commit's author field, fixed permanently, no reassignment |
| `id`       | content hash (of the create commit)| permanent forever, displayed as an auto-growing short prefix (`git rev-parse --short` convention) |

Explicitly excluded (see requirements doc's non-goals for the reasoning
behind each): label/type, references/dependencies, comments/notes,
ordinal/fractional-index ranking (Trello drag-to-reorder), owner
reassignment.

## Storage model

- Each item is an append-only op-log: one real git **commit** per
  operation (create, list change, priority change, title edit), chained
  under a dedicated ref that moves forward with each new operation — same
  relationship as a branch ref to its commit history.
- An operation's payload is a git **tree** with one named entry per field
  being set on that operation (e.g. a create op's tree has `title`,
  `list`, `priority` entries; `priority` simply absent if unset). This
  means `git cat-file`/`git show <tree>:field` can inspect state directly
  with no custom parser, and an operation touching only one field reuses
  the already-existing blob for every field it didn't touch (git's normal
  tree-diffing efficiency, no extra work needed to get it).
- No separate database is the source of truth. State is reconstructed by
  walking git objects. No local cache for v1 — at this data size
  (personal/small-team backlog, not thousands of items) a git-object walk
  is fast enough that a disposable index buys nothing worth the added
  moving part.
- An item's `id` is the hash of its *create* commit specifically — not the
  ref, which moves forward with each new operation. Permanent from the
  moment of creation, never reused or renumbered.
- Collision-avoidance (two commits never accidentally hashing the same)
  comes from ordinary git commit metadata (author, timestamp, parent
  pointer) — real-world entropy is enough that independent machines never
  collide. Ordering across a *merged* history is a separate problem and
  needs more than that; see Sync & conflict resolution below.
- Sync is `git push`/`fetch` of the dedicated ref namespace against
  whatever remote is already configured, via an explicit refspec (the
  namespace lives outside `refs/heads/`, so it isn't touched by a plain
  push/fetch of the current branch). No GitHub REST/GraphQL API calls, no
  rate limits, no textual merge-conflict surface for the common case
  (each item lives on its own ref) — divergence is resolved
  automatically, not handed to the user as a conflict.

## Sync & conflict resolution

**Ref namespace**: one ref per item, `refs/backlog/<full-hash-of-create-commit>`,
moving forward with each op-log commit. Full hash in the ref name (needs
global uniqueness); the short auto-growing prefix from the schema table is
purely a display convention, unrelated to the ref name itself.

**Why `sync` can't be plain `git push`/`fetch`**: `refs/backlog/*` isn't
under `refs/heads/`, so a normal push/fetch of the current branch never
touches it. `sync` uses an explicit refspec — push
`refs/backlog/*:refs/backlog/*`, fetch into a remote-tracking namespace
`refs/backlog/*:refs/remotes/<remote>/backlog/*` (same convention as
branches).

**Why fetch-then-push isn't enough**: two machines can each add an
operation to the same item while offline, starting from the same ref tip.
Both then try to advance the ref, which is exactly a non-fast-forward —
git rejects it the same as a diverged branch push. `sync` catches that
rejection and reconciles automatically instead of surfacing a conflict:
fetch the remote tip, merge the two op-log chains, commit the merged
result, then push (now a fast-forward since it descends from what's on
the remote).

**Merge algorithm** (independently implemented, architecture verified via
git-bug's `doc/design/data-model.md`, not its GPL-3.0 code): wall-clock
timestamps are unsuitable for ordering divergent operations — clock skew
between machines can make an earlier-real-time edit look "later." Instead,
each operation carries an explicit **Lamport logical clock** value (a
counter, not a time: new op's clock = highest clock value it has seen + 1),
stored as its own tree entry alongside the field entries (e.g.
`create-clock-14`). This captures causality without depending on
synchronized clocks.

Reconciling two divergent op-log chains means computing one deterministic
total order over *all* operations from both sides, then replaying them:

1. Commit-DAG parent-pointer order, wherever the DAG already implies one.
2. Lamport clock value, for anything the DAG doesn't order.
3. Lexicographic order of the operation's content hash, as a tiebreak for
   truly concurrent operations (same clock value, no causal relationship)
   — arbitrary but identical on every clone, since it's just a hash
   comparison.

Nothing is discarded: the full op-log remains visible via
`git backlog show <id>`. A field's "current" value is just whichever
operation touching that field sorts last in the total order — so two
concurrent edits to *different* fields (e.g. one sets `list`, the other
`priority`) both survive automatically, and two concurrent edits to the
*same* field resolve deterministically without needing a "winner" the user
has to be told about.

## CLI command surface

Every mutable field (`list`, `priority`, `title`) gets its own setter
command, same shape: `<verb> <id> <value>`. No `done`/`start`/`reopen`/
`move` special-cased verbs — closing an item is just
`git backlog list <id> closed`.

**Create**
```
git backlog add "<title>" [--list backlog|current|closed] [--priority p0|p1|p2]
```
Defaults to `--list backlog` if omitted. `--priority` omitted ⇒ unset.

**Read**
```
git backlog all [--list <value>] [--priority <value>]
git backlog show <id>
```
`all` prints everything, grouped by list (backlog, current, then closed —
Trello's To Do/Doing/Done column order), then by priority tier within each
list (p0, p1, p2, then unprioritized), sorted oldest-first by creation time
within each group. Long titles truncate for display (~60 chars + "…");
`show` always prints the full title untruncated, plus the item's full
op-log history.

**Update**
```
git backlog list <id> <backlog|current|closed>
git backlog priority <id> <p0|p1|p2|none>
git backlog title <id> "<new title>"
```

**Sync / Init**
```
git backlog init
git backlog sync
```
`init` starts tracking backlog items in the current repo (creates the ref
namespace). `sync` pushes/fetches that namespace against the configured
remote, reconciling divergent op-logs automatically — see Sync & conflict
resolution above.

## Monorepo scoping: considered and closed

Discussed at length: prefixing ref paths with the directory an item was
created in (`refs/backlog/<dir-path>/<hash>`), with `all` scoping to the
nearest enclosing directory via an upward walk (git/npm-root-style) — an
empty prefix at repo root naturally matches every item, so root always
shows everything with no special-casing. Also considered a scope field
in the payload tree plus a per-directory marker file instead of encoding
scope in the ref path (avoids orphaning items on directory renames, at
the cost of needing to read every item to filter by scope, since git has
no native "list refs by field value" primitive).

Both are real, buildable designs — but rejected for v1. This tool targets
a personal/small-team backlog, not a large monorepo with genuinely
unrelated projects (Gmail/Kubernetes-in-one-repo scale) sharing a single
list. A single shared, repo-wide backlog is enough for that target. If a
project ever outgrows that, it's a different tool's problem, not a
feature to add here.

## Implementation notes

- Language: Go, single static binary, no runtime dependency on either
  machine this is developed from (Linux dev box, Mac test machine).
- CLI framework: `spf13/cobra` (Apache 2.0, verified).
- Not depending on git-bug's code (GPL-3.0) — architecture borrowed,
  implementation independent, MIT licensed throughout.
