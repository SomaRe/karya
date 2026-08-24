# Karya

Karya is a local project tracker for one person and their AI agents. It uses a SQLite-backed CLI and an embedded, read-only local web UI. There is no authentication, remote service, or active-project state.

## Architecture

- Database: `~/.config/karya/karya.db`
- Hierarchy: `Project -> Area -> Ticket`
- Project keys: uppercase alphanumeric, for example `KARYA` or `APP2`
- Ticket keys: `<PROJECT_KEY>-<number>`, for example `KARYA-42`
- No migration or backward compatibility: the SQLite implementation does not read, import, or preserve the former filesystem layout.
- The CLI is the only supported write interface. The local web UI and its HTTP API are read-only.

Ticket enums:

| Field | Values | Default |
| --- | --- | --- |
| `status` | `backlog`, `in-progress`, `review`, `done`, `cancelled` | `backlog` |
| `priority` | `low`, `medium`, `high` | `medium` |
| `type` | `task`, `bug`, `spike` | `task` |

## CLI Contract

Every command that returns a resource or collection accepts `--json` for machine-readable output. All Area and Ticket commands require an explicit `--project <PROJECT_KEY>`; there is no implicit active project. Use `--db <path>` to override the default database location, such as in a test environment.

```text
karya project create <name> --key <PROJECT_KEY> [--json]
karya project list [--json]
karya project get <PROJECT_KEY> [--json]
karya project delete <PROJECT_KEY> --yes

karya area create <name> --project <PROJECT_KEY> [--slug <slug>] [--json]
karya area list --project <PROJECT_KEY> [--json]
karya area get <area> --project <PROJECT_KEY> [--json]
karya area delete <area> --project <PROJECT_KEY> --yes

karya ticket create <title> --project <PROJECT_KEY> [--area <area>] [--parent <TICKET_KEY>] \
  [--description <text>] [--type task|bug|spike] [--priority low|medium|high] [--json]
karya ticket list --project <PROJECT_KEY> [--area <area>] [--parent <TICKET_KEY>] [--status <status>] \
  [--type <type>] [--priority <priority>] [--search <text>] [--flagged=true|false] [--json]
karya ticket get <TICKET_KEY> --project <PROJECT_KEY> [--json]
karya ticket update <TICKET_KEY> --project <PROJECT_KEY> [--title <title>] [--description <text>] \
  [--area <area>] [--type <type>] [--priority <priority>] [--status <status>] \
  [--reason <text>] [--flagged=true|false] [--revision <revision>] [--json]
karya ticket note add <TICKET_KEY> <body> --project <PROJECT_KEY> [--actor <label>] [--json]
karya ticket note list <TICKET_KEY> --project <PROJECT_KEY> [--json]
karya ticket delete <TICKET_KEY> --project <PROJECT_KEY> --yes
```

`ticket update --revision <revision>` provides optimistic conflict protection. Supply the revision returned by a preceding `ticket get` or `ticket list`; the update must fail if the ticket has changed since that revision was read.

`cancelled` requires `--reason`; Karya stores the current reason and appends a timestamped cancellation Note. A child created with `--parent` remains queryable through `ticket list --parent`. Move a Ticket with `ticket update --area <slug> --revision <revision>` or clear its Area with `--area=""`. Notes are append-only, receive Karya-generated UTC timestamps, and may have an optional unauthenticated actor label.

## Usage Examples

```bash
# Create and inspect a project.
karya project create "Karya" --key KARYA
karya project get KARYA --json

# Areas and tickets are always explicitly scoped to a project.
karya area create "Architecture" --project KARYA
karya ticket create "Add SQLite storage" --project KARYA --area architecture \
  --type task --priority high
karya ticket list --project KARYA --status backlog --json

# Read before updating, then protect the update with the observed revision.
karya ticket get KARYA-42 --project KARYA --json
karya ticket update KARYA-42 --project KARYA --status in-progress --revision 3
```

## Web UI

`karya serve [--port 8787]` binds to `127.0.0.1`. The embedded UI and HTTP API are for human browsing only and remain read-only. Create, update, and delete data through the CLI, never through the UI or API.

The API includes `GET|HEAD /api/v1/projects`, project and Area detail routes, `GET|HEAD /api/v1/tickets`, `GET|HEAD /api/v1/tickets/{key}`, and `GET|HEAD /api/v1/tickets/{key}/notes`. Ticket routes require an explicit `project` query parameter.

## Agent Workflow

Agents use the CLI, not SQLite or the web API, to mutate tracker data:

1. Run `karya docs` to print the complete guide for the installed binary.
2. Discover the project with `karya project list --json` or `karya project get <key> --json`.
3. Before creating or changing an Area or Ticket, list or get the relevant project-scoped records with `--json`.
4. For ticket updates, read the ticket first and pass its revision with `--revision`; re-read and reconsider after a conflict rather than blindly retrying.
5. Run `delete` only when the user explicitly requests a destructive operation.

## Development

```bash
make build
make test
make clean
```
