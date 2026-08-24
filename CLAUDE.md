# Karya — CLAUDE.md

## Product Contract

Karya is a local project tracker for a single human and AI agents. Its architecture is SQLite at `~/.config/karya/karya.db`, a CLI write interface, and an embedded read-only local web UI and HTTP API. There is no authentication, remote service, active-project setting, migration, or backward compatibility with the former filesystem/Markdown store.

## Build and Test

- Language: Go, distributed as a single `karya` binary.
- Build: `make build` produces `./karya`.
- Test: `make test` runs `go test ./...`.
- Clean: `make clean` removes the built binary.
- Install: `make install` installs `/usr/local/bin/karya` and requires appropriate privileges.
- Go binary on this machine: `/usr/local/go/bin/go`.

Run `make test` after Go changes. Documentation-only changes do not require a build or test run.

## Target Model

```
Project -> Area -> Ticket
```

- Project keys are uppercase alphanumeric, for example `KARYA`.
- Ticket keys are `<PROJECT_KEY>-<number>`, for example `KARYA-42`.
- Ticket status: `backlog`, `in-progress`, `review`, `done`, `cancelled`.
- Ticket priority: `low`, `medium`, `high`.
- Ticket type: `task`, `bug`, `spike`.
- Ticket updates may use `--revision` for optimistic conflict protection.

All Area and Ticket operations require `--project <PROJECT_KEY>`. Commands that return a resource or collection support `--json`. Tickets support creation-only parents, revision-safe Area moves, cancellation reasons, and append-only timestamped Notes through `ticket note add/list`.

## Agent Rules

- Never write to, query for mutation through, or otherwise modify `~/.config/karya/karya.db` directly. Use the CLI.
- Before any mutation, discover the project and list or get the relevant Area or Ticket. Use `--json` for machine-readable discovery.
- Before updating a ticket, get it and pass its observed revision with `--revision` when conflict protection is needed.
- Do not run `delete` or another destructive action without explicit user authorization.
- The UI and HTTP API are human-browsing surfaces only and must remain read-only.

## Target Layout

The implementation uses this layout:

```text
karya/
├── main.go                    # entry point
├── Makefile                   # build, test, install, clean
├── cmd/                       # Cobra root, project, area, ticket, and serve commands
├── cmd/web/                   # embedded read-only UI assets
└── internal/
    ├── api/                   # read-only web API handlers
    ├── domain/                # validated entities and enums
    ├── service/               # key-based application operations
    └── sqlite/                # database access and schema migrations
```

Do not retain a filesystem migration or compatibility layer in the target layout.
