# Karya — Build Steps

## Step 1: Project scaffold
- [ ] `go mod init github.com/somare/karya`
- [ ] Install deps: `cobra`, `yaml.v3`, `BurntSushi/toml`
- [ ] Directory structure: `cmd/`, `internal/model/`, `internal/store/`, `internal/config/`
- [ ] `main.go` entry point
- [ ] `cmd/root.go` — root cobra command
- [ ] Smoke test: `karya --help` works

## Step 2: Core types & storage
- [ ] `internal/model/` — `Ticket`, `Epic`, `Project` structs
- [ ] `internal/store/` — read/write `ticket.md` (frontmatter + body)
- [ ] `internal/store/` — read/write `.karya.toml` project config
- [ ] ID generation (`PREFIX-NNN`, auto-increment)
- [ ] `internal/config/` — global config (`~/.config/karya/config.toml`): active project

## Step 3: `project` commands
- [ ] `karya project new "My App" --prefix MYAPP` — creates project dir + `.karya.toml`
- [ ] `karya project ls` — lists all projects, marks active one
- [ ] `karya use <project>` — sets active project in global config

## Step 4: `epic` commands
- [ ] `karya epic new "User Auth"` — creates epic folder inside active project
- [ ] `karya epic ls` — lists all epics in active project

## Step 5: `ticket` commands (write)
- [ ] `karya ticket new "Login page" --epic user-auth [--type bug] [--priority high]`
- [ ] `karya ticket set MYAPP-001 status in-progress`
- [ ] `karya ticket set MYAPP-001 priority high`
- [ ] `karya ticket flag MYAPP-001` — toggle flagged

## Step 6: `ticket` commands (read)
- [ ] `karya ticket show MYAPP-001` — print ticket to stdout
- [ ] `karya ticket open MYAPP-001` — open ticket.md in $EDITOR
- [ ] `karya ticket ls [--epic] [--status] [--flagged] [--json]`
