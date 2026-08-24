# Karya Design Specification

## Purpose

Karya is a local project tracker for a single human and AI agents. It is a Go single binary with a CLI as its write interface and an embedded local web UI for read-only browsing. There is no authentication or remote service.

## Data Model

```
Project -> optional Area -> Ticket -> Note
```

- A **Project** is the top-level unit of work. Its key is uppercase alphanumeric, such as `KARYA` or `APP2`.
- An **Area** groups tickets within one project.
- A **Ticket** is a unit of work. Its key is `<PROJECT_KEY>-<number>`, such as `KARYA-42`.
- A Ticket may have one parent Ticket in the same project. Parenting is assigned only when creating the child; child statuses do not roll up automatically.
- A **Note** is an append-only observation with a Karya-generated UTC timestamp and optional unauthenticated actor label.

Tickets have these constrained fields:

| Field | Default | Allowed values |
| --- | --- | --- |
| `status` | `backlog` | `backlog`, `in-progress`, `review`, `done`, `cancelled` |
| `priority` | `medium` | `low`, `medium`, `high` |
| `type` | `task` | `task`, `bug`, `spike` |

Tickets expose a revision value. An update that supplies `--revision` succeeds only when that value matches the ticket's current revision; otherwise it reports a conflict.

Cancellation requires a nonblank reason. The current reason is stored on the Ticket and a cancellation Note is appended in the same transaction. Leaving `cancelled` clears the current reason but does not remove historical Notes. Area movement preserves the Ticket key and requires revision protection. Appending an ordinary Note does not mutate the Ticket revision.

## Storage

All persistent application data is stored in SQLite at:

```text
~/.config/karya/karya.db
```

This replaces the former filesystem/Markdown design. There is no migration path and no backward compatibility layer: the SQLite implementation must not scan, import, or write the prior project folders, TOML config, YAML frontmatter, or `ticket.md` files.

## CLI Contract

The command groups are:

```text
project create | list | get | delete
area    create | list | get | delete
ticket  create | list | get | update | delete
ticket note add | list
```

Every Area and Ticket command requires `--project <PROJECT_KEY>`, including `get`, `list`, `update`, and `delete`. Karya does not retain or infer an active project.

Every command that returns a resource or collection supports `--json`. This is the stable machine-readable interface for agents. The CLI is the supported write surface.

`karya docs` prints the embedded agent guide without opening the database. It
is the first command an unfamiliar agent should run.

Example workflow:

```bash
karya project create "Karya" --key KARYA
karya area create "Architecture" --project KARYA
karya ticket create "Add SQLite storage" --project KARYA --area architecture --priority high
karya ticket get KARYA-42 --project KARYA --json
karya ticket update KARYA-42 --project KARYA --status in-progress --revision 3
```

## Web UI and API

`karya serve [--port 8787]` serves the embedded UI locally. Both the human UI and HTTP API are strictly read-only. They may expose project, Area, Ticket, parent, cancellation, and Note data for browsing, but may not create, update, or delete records.

## Agent Safety

Agents must use CLI commands for writes and must never open or modify `karya.db` directly. Before any mutation, an agent discovers the target project and lists or gets the relevant Area or Ticket. Deletes and other destructive actions require explicit user direction. Agents should use `--json` for discovery and use the revision returned by a ticket read when updating a ticket.
