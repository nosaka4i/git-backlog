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
| `comment`  | string, optional                   | freeform "why," not threaded — see below |
| `id`       | content hash (of the create commit)| permanent forever, displayed as an auto-growing short prefix (`git rev-parse --short` convention) |

Explicitly excluded (see requirements doc's non-goals for the reasoning
behind each): label/type, references/dependencies, ordinal/fractional-index
ranking (Trello drag-to-reorder), owner reassignment.

`comment` was originally on this excluded list too ("comments/notes"), on
the theory that WHY/HOW belongs in GitHub Issues or a
brainstorm/requirements/design doc, not the backlog item. Revisited: if the
backlog is useful enough on its own, it shouldn't need to overload Issues
just to hold a line of context — that pushes Issues back into being a pure
bug tracker, which is closer to what it was originally for. The scope kept
narrow, though: `comment` is a single freeform field, edited the same way
as `title` (each edit fully replaces the value; no threading, no per-note
authorship beyond whichever op-log commit made that edit). It does not
reopen the rest of the excluded list — this is deliberately the smallest
version of "why," not a move toward comment threads, labels, or
dependencies.

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
git backlog add "<title>" [--list backlog|current|closed] [--priority p0|p1|p2] [--as-agent]
```
Defaults to `--list backlog` if omitted. `--priority` omitted ⇒ unset.
`--as-agent` — see Agent identity below — is meaningfully different here
than on the Update commands: since the create commit's author becomes
the item's permanent `owner`, `add --as-agent` doesn't just attribute one
op, it makes the agent the item's owner forever.

**Read**
```
git backlog all [--list <value>] [--priority <value>] [--closed-limit N] [--json]
git backlog show <id> [--json]
git backlog history [--list <value>] [--priority <value>] [--json]
```
`all` prints everything, grouped by list (current, backlog, then closed —
what you're actively doing first, what's queued next, what's done last),
then by priority tier within each list (p0, p1, p2, then unprioritized),
sorted oldest-first by creation time within each group. Long titles
truncate for display (~60 chars + "…"); `show` always prints the full
title untruncated, plus the item's full op-log history.

A tool that's succeeding at its job accumulates `closed` items forever, so
by default `all` caps the closed section to the 10 most recently *updated*
(not created) items — using the tip op-log commit's own timestamp, already
read as part of loading the item, so no extra git calls. `--closed-limit 0`
removes the cap; `--list closed` (an explicit, narrow ask) always shows the
complete closed list regardless of the cap.

`--json` swaps the human-readable output for machine-readable JSON (same
filtering, same sort order, same closed cap) — a flag on the existing read
commands, not a separate verb, matching how `kubectl`/`gh`/`docker` expose
it.

`history` flattens every item's op-log into one feed, newest-first,
instead of `show`'s per-item view. Each entry renders its field changes as
a short verb phrase — `Added item`, `Moved to <list>`, `Updated priority
to <value>`, `Cleared priority`, `Renamed item` — rather than raw
`field: value`, and `show <id>`'s own history section uses the same
phrasing, so the two commands read as one consistent style rather than
two different formats for the same underlying op-log data. A single
op-log commit that touches multiple fields at once (the common case being
a `sync` merge commit reconciling two concurrent edits) renders as one
timestamp/id/author header followed by one action line per field — the
header's author is whoever's commit that is (e.g. whoever ran `sync`
locally for a merge commit), not necessarily whoever originally made each
individual field change on the other side; a pre-existing property of
diffing against a commit's first parent, not something `history`
introduces. `--list`/`--priority` filter by an item's *current* state,
same as `all` — not by what the value was at the time of each historical
op.

**Update**
```
git backlog list <id> <backlog|current|closed>
git backlog priority <id> <p0|p1|p2|none>
git backlog title <id> "<new title>"
git backlog comment <id> "<text>"
git backlog comment show <id> [--json]
```
`comment` follows the same replace-on-edit shape as `title` — passing `""`
clears it. `show <id>`'s own history only renders `Updated
comment`/`Cleared comment` (consistent with how it never inlines a
renamed title either), so it doesn't surface past comment text by itself.
`comment show <id>` is the dedicated read path: it walks the op-log,
picks out just the `comment` field's changes, and prints them newest
first (matching `history`'s convention) with timestamp/commit/author, so
past comments read like a thread — without needing a real comment-thread
data structure, since every edit was already a real git commit. Nested
under `comment` rather than a
separate top-level verb, the same way `git remote add`/`git remote show`
share one namespace instead of being unrelated top-level commands.

**Sync / Init**
```
git backlog init
git backlog sync
git backlog version
```
`init` starts tracking backlog items in the current repo (creates the ref
namespace). `sync` pushes/fetches that namespace against the configured
remote, reconciling divergent op-logs automatically — see Sync & conflict
resolution above. `version` prints the build version.

## Agent identity

**The problem**: every op-log commit's author comes from whatever git
identity is ambient when the command runs (`internal/gitx.CommitTree`
shells out to `git commit-tree` with no author override, same as any
plain `git commit`). If a human and an AI coding agent both drive
`git backlog` from the same checkout — the common case, since the agent
runs git commands as the human's own shell/git config — every op either
of them makes carries the *same* author. Concretely: if both add
`comment`s to the same item, `comment show <id>` renders them
indistinguishably, as if one person were talking to themselves.

**Not a GitHub-account problem**: the natural first instinct is "give the
bot its own GitHub account," the way `dependabot[bot]`/
`github-actions[bot]` get a `[bot]` badge on github.com. That's solving
the wrong layer. git-backlog never calls GitHub's API — it only ever sees
plain git commit authorship (`author <name> <email> <date>` in the
commit object), which is metadata git already tracks for *any* two
contributors, bot or human, with zero relationship to whether `<email>`
belongs to a real, registered account anywhere. No GitHub account,
"machine user" or otherwise, is required.

**The mechanism**: `internal/gitx.CommitTreeAs(tree, parents, message,
name, email)` runs `git commit-tree` with `GIT_AUTHOR_NAME`/
`GIT_AUTHOR_EMAIL`/`GIT_COMMITTER_NAME`/`GIT_COMMITTER_EMAIL` set in the
subprocess environment, overriding the ambient git config for that one
commit only — nothing durable is changed, no repo or global git config is
touched, and every other command's commits are unaffected. Both author
*and* committer are overridden (not just author) so a raw `git log`/
`git show` on the commit object is fully consistent too, not just what
git-backlog itself chooses to display — otherwise someone inspecting the
repo with plain git (exactly the kind of "found out in an obscure place"
surprise worth avoiding) would see a mismatched author/committer pair and
have no idea why.

**Setup** (plain git config, no git-backlog wrapper command for it —
consistent with how `user.name`/`user.email` themselves are configured):
```
git config backlog.agent.name "Claude"
git config backlog.agent.email "noreply@anthropic.com"
```
Local (repo) config, not global — the same agent identity doesn't
necessarily make sense in every repo, and local config is what
`backlog.init` itself already uses.

**Usage**: `--as-agent` on `add` and the field-setter commands — `title`,
`priority`, `list`, `comment` — reads `backlog.agent.name`/
`backlog.agent.email` and records that operation under the agent's
identity instead of the ambient one:
```
git backlog comment <id> "<text>" --as-agent
```
Without `--as-agent`, behavior is unchanged — commands attribute to
whatever git identity was already ambient, exactly as before this
feature existed. Deliberately *not* a global mode/env var that silently
changes every command's behavior — each invocation opts in explicitly,
so `comment show <id>`'s author column stays a reliable, per-op signal
of who (or what) actually made each change, not just what the last
`git config` toggle happened to be set to.

If `--as-agent` is passed but `backlog.agent.name`/`backlog.agent.email`
aren't configured, the command errors immediately (before writing
anything) rather than silently falling back to the ambient identity —
falling back silently would defeat the entire point, since the whole
failure mode being avoided here is misattributed authorship going
unnoticed.

**What this does *not* affect, except on `add`**: an item's `owner`
(`OwnerName`/`OwnerEmail`) is read from the *create* commit's author
specifically and is permanent by design (see Schema above — "no
reassignment") — using `--as-agent` on a later `comment`/`priority`/
`list`/`title` op never changes who owns the item, only who's credited
for that one op-log entry. `add --as-agent` is the deliberate exception:
because the create commit's author *is* the owner, using `--as-agent`
there makes the agent the item's permanent owner, not just that one
op's author. This isn't a special case bolted on — it falls out of
`owner`'s existing definition once you read it literally: "read from the
create commit's author field" already means *whoever physically ran
`add`*, not whoever came up with the idea for the item. Asking the agent
to file an item on your behalf and having it run `add --as-agent` means
the agent legitimately is who added it.

**Not retroactive**: `--as-agent` only affects operations recorded from
that point forward. There's no supported way to relabel a *past*
op-log commit's author after the fact — doing so would require rewriting
that commit and every descendant commit after it (a commit's hash is
derived from its content, author included, so "editing" it is always
actually "replace with a new object and every commit after it"), and if
it's the item's *create* commit, rewriting it changes the item's
permanent ID outright, since the ID **is** that commit's hash. Consistent
with every other place in this design that treats the op-log as
append-only and recovers forward, never by rewriting history (see Sync &
conflict resolution above).

## Schema evolution & version compatibility

**No migration from other tools.** git-backlog doesn't import from
Trello/Jira/GitHub Issues/etc. — `init` starts empty. This is a
deliberate non-goal, not a gap: importing means mapping someone else's
schema onto this one, which is a per-source problem, not a general
feature this tool needs to own. Every backlog here starts fresh.

**No migration between git-backlog schema versions either — because none
is ever needed for the kind of change made so far.** The schema table
above has already grown once (`comment` added after v1 shipped) without
any migration step, and that's not a coincidence: the tree-of-named-
optional-entries storage model (see Storage model above) makes purely
additive schema changes safe in both directions, by construction, not by
convention someone has to remember to follow. Verified against the
actual code, not just asserted:

- **Old data, new binary**: `fieldValue` (`internal/store/item.go`)
  treats a tree entry that doesn't exist as `""` — the same "absent, not
  a literal empty value" semantics `priority` already relies on. An item
  written before `comment` existed simply has no `comment` entry, so it
  reads as unset. No backfill, no default-value migration.
- **New data, old binary**: an older binary simply never asks
  `fieldValue` for a field name it doesn't know about, so it's invisible
  to `show`/`all` but not touched, corrupted, or lost — it's still sitting
  right there in the tree object waiting for a binary that knows to look
  for it.
- **Mixed-version `sync`**: the case most likely to actually happen (a
  team upgrades gradually, not atomically) is also safe, verified by
  reading `mergeItem` (`internal/store/sync.go`) directly rather than
  assuming: it builds `merged` from whatever entries exist in the base
  tree and keys `touches` by whatever `DiffTree` reports changed —
  nowhere does it enumerate "the known fields." An old binary that has
  never heard of `comment` still correctly carries a `comment` change
  forward through a merge (right clock-value winner, right tiebreak),
  purely because the algorithm operates on raw entry names, not a
  hardcoded field list. Old binaries can safely participate in `sync`
  with new ones — not merely tolerate it.
- **Unknown fields in `history`/`show`**: `actionLine` (`cmd/action.go`)
  has a `default: return field + ": " + value` fallback for any field
  name it doesn't have a named case for. This predates `comment` — it
  was written in from the start for exactly this situation, so an old
  binary rendering a newer item's op-log degrades to a plain `field:
  value` line instead of erroring or silently dropping the entry.
- **Bug fixes** (as opposed to schema/feature changes) are logic-only —
  e.g. the fix that stopped `fieldValue` from silently swallowing a
  transient `git cat-file` read error into an empty string (see git log)
  changed nothing about what's written to git objects. Upgrading past a
  bug fix is never a format concern, only old data ever behaving
  differently is (never happened yet — no fix so far has changed how
  already-written data is interpreted).

**Consequence**: no explicit schema-version field (a `schema-version`
tree entry, or similar) exists, and none is being added speculatively.
One would only earn its keep for a genuinely *breaking* change — renaming
an existing field, or redefining what an existing field's value means
(e.g. if `priority`'s tier values were ever redefined) — not for adding
a new optional field, which is already safe for free by the mechanisms
above. That's a real risk to keep in mind for the future, but not one
that exists yet, and building version-negotiation machinery for a
breaking change that hasn't been designed yet would be speculative
complexity with no current payoff.

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
