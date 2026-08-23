# Karya

Karya is a filesystem-native project tracker for one person and their AI agents. It provides a CLI and a local, read-only web UI. Project data is stored as Markdown files under `~/.config/karya/`.

## Install

```bash
make build
./karya --help

# Optional: install to /usr/local/bin
make install
```

## Quick Start

```bash
karya project new "My App" --prefix MYAPP
karya use "My App"
karya epic new "User Auth"
karya ticket new "Login page" --epic user-auth --priority high \
  --description "Build the initial login screen."
karya ticket ls
```

Tickets are stored at `~/.config/karya/projects/<project>/<epic>/<id>/ticket.md`. Each file has YAML frontmatter and a free-form Markdown body, so it remains editable outside Karya.

## Commands

```bash
# Projects
karya project new "My App" --prefix MYAPP
karya project ls
karya use "My App"

# Epics
karya epic new "User Auth"
karya epic ls

# Tickets
karya ticket new "Login page" --epic user-auth [--type task|bug|spike] \
  [--priority low|medium|high] [--description "..."]
karya ticket ls [--epic user-auth] [--status backlog] [--type bug] \
  [--grep login] [--flagged] [--json]
karya ticket set MYAPP-001 status in-progress
karya ticket set MYAPP-001 priority high
karya ticket set MYAPP-001 type spike
karya ticket set MYAPP-001 description "Updated description"
karya ticket flag MYAPP-001
karya ticket show MYAPP-001
karya ticket open MYAPP-001
karya ticket delete MYAPP-001
```

Valid statuses are `backlog`, `in-progress`, `review`, and `done`.

## Web UI

```bash
karya serve [--port 8787]
```

Open `http://localhost:8787`. The embedded UI reads projects, epics, and tickets through local HTTP endpoints; create and edit data with the CLI or the Markdown files.

## Development

```bash
make build
make test
make clean
```
