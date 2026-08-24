# Karya Agent Instructions

The Karya data store is SQLite at `~/.config/karya/karya.db`. Agents must not write to the database directly. Use the Karya CLI for every mutation; the local web UI and HTTP API are read-only human-browsing interfaces.

## Required Workflow

1. Run `karya docs` for the installed binary's complete agent usage guide.
2. Discover the target project with `karya project list --json` or `karya project get <PROJECT_KEY> --json`.
3. Before creating or mutating an Area or Ticket, list or get the relevant records with the required `--project <PROJECT_KEY>` and `--json`.
4. Before a ticket update, get the ticket and use its revision with `--revision <revision>` to prevent overwriting a concurrent change. On conflict, re-read and reconsider instead of blindly retrying.
5. Only run `delete` or another destructive command when the user explicitly authorizes it.

## Target Command Rules

- Project keys are uppercase alphanumeric, for example `KARYA`.
- Ticket keys use `<PROJECT_KEY>-<number>`, for example `KARYA-42`.
- Area and Ticket commands always include `--project <PROJECT_KEY>`; do not rely on an active-project setting.
- Use `--json` on commands returning project, area, or ticket data.
- Valid ticket values are: status `backlog`, `in-progress`, `review`, `done`, `cancelled`; priority `low`, `medium`, `high`; type `task`, `bug`, `spike`.
- Cancellation requires a reason. Parent Tickets must be in the same project. Area moves require a revision.
- Notes are append-only and receive Karya-generated UTC timestamps. An optional actor is only a label; omit it when identity is unknown.
