# Karya — CLAUDE.md

## What It Is

A filesystem-native project tracker for a single human + AI agents. No server, no auth, no UI. CLI is the primary interface. Sanskrit: कार्य (work/task/deed).

## Language & Build

- **Go** — single binary, fast, easy to distribute
- Build: `make build` → `./karya`
- Install system-wide: `make install` → `/usr/local/bin/karya`
- Go binary: `/usr/local/go/bin/go`

## Hierarchy

```
Project → Epic → Ticket
```

- **Project** — top-level unit of work
- **Epic** — folder grouping related tickets (slug-named, e.g. `user-auth`)
- **Ticket** — actual unit of work; type is `task` or `bug`

## Storage Layout

All data lives at `~/.config/karya/`:

```
~/.config/karya/
├── config.toml                  # global config (active_project)
└── projects/
    └── My App/
        ├── .karya.toml          # project config (name, prefix)
        └── user-auth/           # epic (folder)
            └── MYAPP-001/       # ticket folder
                ├── ticket.md
                └── <attachments>
```

## ticket.md Format

YAML frontmatter between `---`, free-form markdown body below.

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

| Field      | Required | Default    | Values                                      |
|------------|----------|------------|---------------------------------------------|
| `id`       | yes      | auto       | `<PREFIX>-<NNN>`                            |
| `title`    | yes      | —          | string                                      |
| `status`   | yes      | `backlog`  | `backlog` `in-progress` `review` `done`     |
| `type`     | no       | `task`     | `task` `bug`                                |
| `priority` | no       | `medium`   | `low` `medium` `high`                       |
| `epic`     | no       | —          | epic folder name (slug)                     |
| `flagged`  | no       | `false`    | bool — marks impediments/blockers           |

## CLI Reference

```bash
# Projects
karya project new "My App" --prefix MYAPP
karya project ls
karya use <project>

# Epics
karya epic new "User Auth"
karya epic ls

# Tickets
karya ticket new "Login page" --epic user-auth [--type bug] [--priority high]
karya ticket ls [--epic user-auth] [--status in-progress] [--flagged] [--json]
karya ticket set MYAPP-001 status in-progress
karya ticket set MYAPP-001 priority high
karya ticket flag MYAPP-001        # toggle flagged
karya ticket show MYAPP-001        # print ticket to stdout
karya ticket open MYAPP-001        # open ticket.md in $EDITOR
karya ticket delete MYAPP-001      # delete ticket (confirms first)
```

`--json` flag on `ticket ls` for AI agent consumption.

## Code Structure

```
karya/
├── main.go                   # entry point
├── Makefile                  # build / install
├── cmd/
│   ├── root.go               # root cobra command + Execute()
│   ├── project.go            # project new, project ls, use
│   ├── epic.go               # epic new, epic ls; activeProjectDir() helper
│   └── ticket.go             # ticket new, set, flag, show, open, ls
└── internal/
    ├── model/model.go        # Ticket, Epic, Project structs; Status/Priority/Type enums
    ├── store/store.go        # read/write tickets+epics+projects; NextID; SortTickets
    └── config/config.go      # global config (active project); ProjectsDir()
```

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — frontmatter parsing
- `github.com/BurntSushi/toml` — project + global config

## Status Scanning

No index, no background server. Every command scans `ticket.md` frontmatter on the fly. Fast enough at personal project scale.
