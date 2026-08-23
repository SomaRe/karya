# Karya — Design Spec

## What It Is

A filesystem-native project tracker for a single human + AI agents. The CLI is the primary interface, with an embedded read-only local web UI for browsing. There is no auth or remote service.

## Language

Go — single binary, fast, easy to distribute.

## Hierarchy

```
Project → Epic → Ticket
```

- **Project** — top-level unit of work (e.g. `my-app`)
- **Epic** — a theme or goal grouping related tickets (e.g. `user-auth`)
- **Ticket** — the actual unit of work; type is `task`, `bug`, or `spike`

No sprints. No stories. No sub-tasks (for now).

## Storage Layout

Central location: `~/.config/karya/projects/`

```
~/.config/karya/projects/
└── my-app/
    ├── .karya.toml         # project config (name, ID prefix)
    └── user-auth/          # epic (folder)
        ├── MYAPP-001/      # ticket folder
        │   ├── ticket.md
        │   └── mockup.png  # attachments live alongside ticket.md
        └── MYAPP-002/
            └── ticket.md
```

## ticket.md

Frontmatter (YAML between `---`) holds structured fields. Body is free-form markdown for description, notes, links.

```markdown
---
id: MYAPP-001
title: Login page
type: task
status: backlog
priority: medium
epic: user-auth
flagged: false
---

Description, notes, links go here.
```

`created` and `modified` are derived from filesystem metadata — not stored in frontmatter.

## Fields

| Field      | Required | Default    | Values                              |
|------------|----------|------------|-------------------------------------|
| `id`       | yes      | auto       | `<PREFIX>-<NNN>`                    |
| `title`    | yes      | —          | string                              |
| `status`   | yes      | `backlog`  | `backlog` `in-progress` `review` `done` |
| `type`     | no       | `task`     | `task` `bug` `spike`                |
| `priority` | no       | `medium`   | `low` `medium` `high`               |
| `epic`     | no       | —          | epic folder name                    |
| `flagged`  | no       | `false`    | bool — marks impediments/blockers   |

## Status Tracking

Scan `ticket.md` frontmatter on every CLI command and web API request. There is no index or background process; at personal project scale this is instant.

## CLI Shape

```
karya project new "My App" --prefix MYAPP
karya project ls

karya epic new "User Auth"
karya epic ls

karya ticket new "Login page" --epic user-auth [--type bug] [--priority high] [--description "..."]
karya ticket ls [--epic user-auth] [--status in-progress] [--type bug] [--grep login] [--flagged] [--json]
karya ticket set MYAPP-001 status in-progress
karya ticket set MYAPP-001 priority high
karya ticket set MYAPP-001 description "Updated description"
karya ticket flag MYAPP-001        # toggle flagged
karya ticket open MYAPP-001        # open ticket.md in $EDITOR
karya ticket show MYAPP-001        # print ticket to stdout
```

`--json` flag on list commands for AI agent consumption.

## Web UI

`karya serve [--port 8787]` starts an embedded local server, defaulting to `http://localhost:8787`. The UI browses projects, epics, and tickets through read-only HTTP endpoints; create and edit data through the CLI or the Markdown files.
