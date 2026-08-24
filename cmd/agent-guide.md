# Karya Agent Guide

Karya is a local SQLite tracker for one human and their AI agents:

```text
Project -> Area -> Ticket -> Note
```

An Area is an optional stable grouping inside a project. A Ticket may also have
one parent Ticket in the same project. Notes are append-only observations and
decisions attached to a Ticket.

Use the CLI for every mutation. Never read or write
`~/.config/karya/karya.db` directly. `karya serve`, the Web UI, and every HTTP
API endpoint are read-only.

## Required Agent Workflow

1. Run `karya docs` when unfamiliar with the installed Karya version.
2. Discover the project with `karya project list --json` or `project get`.
3. Before creating or changing data, list or get the relevant project-scoped
   Areas and Tickets with `--json`.
4. Immediately before a Ticket update, get it and retain its `revision`.
5. Pass that revision to `ticket update --revision`. On a conflict, re-read the
   Ticket and reconsider the update; never blindly retry stale intent.
6. Never delete anything unless the user explicitly authorizes that exact
   destructive action. Deletions require `--yes`.

Use `--json` for agent-consumed output. JSON is written to stdout and errors to
stderr. Human table output is not a stable machine interface. Project keys are
uppercase alphanumeric, such as `KARYA`; Ticket keys use `KARYA-42`; Area names
resolve to lowercase ASCII slugs such as `sync-engine`.

## Values And Semantics

```text
status:   backlog | in-progress | review | done | cancelled
priority: low | medium | high
type:     task | bug | spike
```

Ticket creation defaults to `backlog`, `medium`, and `task`. Use `spike` only
when the primary output is knowledge. Scoping and refinement are normal parts
of every Ticket, not a Ticket type.

Cancelling requires a reason:

```bash
karya ticket update KARYA-42 --project KARYA --status cancelled \
  --reason "Superseded by smaller child tickets" --revision 3 --json
```

The current `cancellation_reason` is structured Ticket data. Karya also appends
a timestamped `cancellation` Note atomically. Reopening with another status
clears the current reason but preserves that Note as history.

## Complete Command Surface

```bash
# Projects
karya project create "Karya" --key KARYA --json
karya project list --json
karya project get KARYA --json
karya project delete KARYA --yes

# Areas: always include --project
karya area create "Sync Engine" --project KARYA --json
karya area list --project KARYA --json
karya area get sync-engine --project KARYA --json
karya area delete sync-engine --project KARYA --yes

# Tickets: always include --project
karya ticket create "Repair replay ordering" --project KARYA \
  --area sync-engine --type bug --priority high --description "..." --json
karya ticket list --project KARYA --status backlog --json
karya ticket get KARYA-42 --project KARYA --json
karya ticket update KARYA-42 --project KARYA --status in-progress \
  --revision 1 --json
karya ticket delete KARYA-42 --project KARYA --yes
```

`ticket list` accepts `--area`, `--parent`, `--status`, `--type`, `--priority`,
`--search`, and `--flagged=true|false`. `ticket update` accepts `--title`,
`--description`, `--area`, `--type`, `--status`, `--reason`, `--priority`,
`--flagged=true|false`, and `--revision`.

## Splitting Work

Parenting is deliberately small: a parent is selected only when creating a
child, must already exist, and must belong to the same project.

```bash
karya ticket create "Cache offline guidance" --project KARYA \
  --area client --parent KARYA-42 --json
karya ticket create "Queue deferred submission" --project KARYA \
  --area client --parent KARYA-42 --json
karya ticket list --project KARYA --parent KARYA-42 --json
```

The original can remain active as an umbrella or be cancelled with a reason.
Karya does not roll up child status automatically. A parent cannot be deleted
while children reference it.

## Moving Between Areas

Area movement preserves the Ticket key and is revision-protected:

```bash
karya ticket update KARYA-42 --project KARYA \
  --area client --revision 4 --json
karya ticket update KARYA-42 --project KARYA \
  --area="" --revision 5 --json
```

No `--area` means unchanged; `--area=""` removes Area assignment. The target
Area must belong to the Ticket's project. Read the Ticket again after moving it
because the revision increments.

## Append-Only Notes

Use Notes for findings, decisions, blockers, handoffs, and context that should
not be lost when the Ticket description changes:

```bash
karya ticket note add KARYA-42 \
  "Reproduced after acknowledgement persistence fails." \
  --project KARYA --actor agent-debug --json
karya ticket note list KARYA-42 --project KARYA --json
```

Each Note has `id`, `ticket_id`, `kind`, `body`, optional `actor`, and
`created_at`. Karya generates the UTC timestamp; callers cannot supply it.
`actor` is an optional label, not an authenticated identity. Omit it when the
agent has no stable identity. Notes cannot be edited or deleted individually,
and appending a Note does not change the Ticket revision. Notes are deleted if
the user explicitly deletes their Ticket.

## JSON And Concurrency

Ticket JSON includes stable `key`, nullable `area_id`, nullable `parent_key`,
nullable `cancellation_reason`, current `revision`, and UTC `created_at` and
`updated_at`. Optional values are JSON `null`; collections are `[]` when empty.

If an update fails with `ticket revision: conflict`, another writer changed the
Ticket. Fetch fresh JSON, compare the new state with the intended change, and
either adapt or stop. Do not simply substitute the new revision and replay.

Use `--db <path>` only for an explicitly isolated test database. The API is for
browsing, never mutation, and accepts only GET and HEAD:

```text
/api/v1/projects
/api/v1/projects/{key}
/api/v1/projects/{key}/areas
/api/v1/tickets?project=KEY
/api/v1/tickets/{ticket}?project=KEY
/api/v1/tickets/{ticket}/notes?project=KEY
```
