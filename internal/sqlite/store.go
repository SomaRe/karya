// Package sqlite provides Karya's authoritative local SQLite store.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/somare/karya/internal/domain"
	_ "modernc.org/sqlite"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrAlreadyExists = errors.New("already exists")
)

// Store is a SQLite-backed Karya data store.
type Store struct {
	db *sql.DB
}

// Open creates or opens a SQLite database and migrates it to the latest schema.
func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat database directory: %w", err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return nil, fmt.Errorf("protect database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", databaseURI(path))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.configureAndMigrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("protect database file: %w", err)
	}
	return store, nil
}

func databaseURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

// Close closes the underlying database connection pool.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) configureAndMigrate(ctx context.Context) error {
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := s.db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure database: %w", err)
		}
	}

	conn, err := beginImmediate(ctx, s.db)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer conn.Close()
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()

	var version int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > 2 {
		return fmt.Errorf("database schema version %d is newer than supported version 2", version)
	}
	if version == 0 {
		for _, statement := range schemaV1 {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply schema version 1: %w", err)
			}
		}
		if _, err := conn.ExecContext(ctx, "PRAGMA user_version = 1"); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
		version = 1
	}
	if version == 1 {
		for _, statement := range schemaV2 {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply schema version 2: %w", err)
			}
		}
		if _, err := conn.ExecContext(ctx, "PRAGMA user_version = 2"); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	committed = true
	return nil
}

var schemaV1 = []string{
	`CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY,
		key TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		next_ticket_number INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS areas (
		id INTEGER PRIMARY KEY,
		project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
		name TEXT NOT NULL,
		slug TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		UNIQUE(project_id, slug)
	)`,
	`CREATE TABLE IF NOT EXISTS tickets (
		id INTEGER PRIMARY KEY,
		project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
		area_id INTEGER NULL REFERENCES areas(id) ON DELETE RESTRICT,
		number INTEGER NOT NULL,
		key TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL,
		status TEXT NOT NULL,
		priority TEXT NOT NULL,
		flagged INTEGER NOT NULL DEFAULT 0 CHECK(flagged IN (0, 1)),
		revision INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		UNIQUE(project_id, number)
	)`,
}

var schemaV2 = []string{
	`ALTER TABLE tickets ADD COLUMN parent_id INTEGER NULL REFERENCES tickets(id) ON DELETE RESTRICT`,
	`ALTER TABLE tickets ADD COLUMN cancellation_reason TEXT NULL`,
	`UPDATE tickets SET type = 'task' WHERE type = 'scope'`,
	`CREATE INDEX tickets_parent_id_idx ON tickets(parent_id)`,
	`CREATE TABLE ticket_notes (
		id INTEGER PRIMARY KEY,
		ticket_id INTEGER NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
		kind TEXT NOT NULL CHECK(kind IN ('note', 'cancellation')),
		body TEXT NOT NULL CHECK(length(trim(body)) > 0),
		actor TEXT NULL CHECK(actor IS NULL OR length(trim(actor)) > 0),
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX ticket_notes_ticket_created_idx ON ticket_notes(ticket_id, created_at, id)`,
}

func (s *Store) CreateProject(ctx context.Context, project domain.Project) (domain.Project, error) {
	project.Normalize()
	if err := project.Validate(); err != nil {
		return domain.Project{}, err
	}
	now := timestamp(time.Now())
	result, err := s.db.ExecContext(ctx, `INSERT INTO projects (key, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, project.Key, project.Name, now, now)
	if err != nil {
		return domain.Project{}, classifyError(err)
	}
	project.ID, err = result.LastInsertId()
	if err != nil {
		return domain.Project{}, fmt.Errorf("read project ID: %w", err)
	}
	return project, nil
}

func (s *Store) ListProjects(ctx context.Context) ([]domain.Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, key, name FROM projects ORDER BY key ASC`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]domain.Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return projects, nil
}

func (s *Store) GetProject(ctx context.Context, key string) (domain.Project, error) {
	project, err := scanProject(s.db.QueryRowContext(ctx, `SELECT id, key, name FROM projects WHERE key = ?`, domain.NormalizeProjectKey(key)))
	if err != nil {
		return domain.Project{}, notFound(err, "project")
	}
	return project, nil
}

func (s *Store) DeleteProject(ctx context.Context, key string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE key = ?`, domain.NormalizeProjectKey(key))
	if err != nil {
		return classifyError(err)
	}
	return requireAffected(result, "project")
}

func (s *Store) CreateArea(ctx context.Context, area domain.Area) (domain.Area, error) {
	if err := area.Normalize(); err != nil {
		return domain.Area{}, err
	}
	if err := area.Validate(); err != nil {
		return domain.Area{}, err
	}
	now := timestamp(time.Now())
	result, err := s.db.ExecContext(ctx, `INSERT INTO areas (project_id, name, slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, area.ProjectID, area.Name, area.Slug, now, now)
	if err != nil {
		return domain.Area{}, classifyError(err)
	}
	area.ID, err = result.LastInsertId()
	if err != nil {
		return domain.Area{}, fmt.Errorf("read area ID: %w", err)
	}
	return area, nil
}

func (s *Store) ListAreas(ctx context.Context, projectID int64) ([]domain.Area, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, name, slug FROM areas WHERE project_id = ? ORDER BY slug ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list areas: %w", err)
	}
	defer rows.Close()

	areas := make([]domain.Area, 0)
	for rows.Next() {
		area, err := scanArea(rows)
		if err != nil {
			return nil, err
		}
		areas = append(areas, area)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate areas: %w", err)
	}
	return areas, nil
}

func (s *Store) GetArea(ctx context.Context, projectID int64, slug string) (domain.Area, error) {
	area, err := scanArea(s.db.QueryRowContext(ctx, `SELECT id, project_id, name, slug FROM areas WHERE project_id = ? AND slug = ?`, projectID, slug))
	if err != nil {
		return domain.Area{}, notFound(err, "area")
	}
	return area, nil
}

func (s *Store) DeleteArea(ctx context.Context, projectID int64, slug string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM areas WHERE project_id = ? AND slug = ?`, projectID, slug)
	if err != nil {
		return classifyError(err)
	}
	return requireAffected(result, "area")
}

func (s *Store) CreateTicket(ctx context.Context, project domain.Project, areaID, parentID *int64, title, description string, ticketType domain.TicketType, priority domain.Priority) (domain.Ticket, error) {
	project.Normalize()
	if project.ID <= 0 {
		return domain.Ticket{}, fmt.Errorf("project ID must be positive")
	}
	if err := project.Validate(); err != nil {
		return domain.Ticket{}, err
	}
	ticket := domain.Ticket{
		ProjectID: project.ID, AreaID: areaID, ParentID: parentID, Title: title, Description: description,
		Type: ticketType, Status: domain.StatusBacklog, Priority: priority,
	}
	ticket.Normalize()

	conn, err := beginImmediate(ctx, s.db)
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("begin ticket creation: %w", err)
	}
	defer conn.Close()
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()

	var storedKey string
	var number int64
	if err := conn.QueryRowContext(ctx, `SELECT key, next_ticket_number FROM projects WHERE id = ?`, project.ID).Scan(&storedKey, &number); err != nil {
		return domain.Ticket{}, notFound(err, "project")
	}
	if storedKey != project.Key {
		return domain.Ticket{}, fmt.Errorf("project key changed: %w", ErrConflict)
	}
	if areaID != nil {
		var areaProjectID int64
		if err := conn.QueryRowContext(ctx, `SELECT project_id FROM areas WHERE id = ?`, *areaID).Scan(&areaProjectID); err != nil {
			return domain.Ticket{}, notFound(err, "area")
		}
		if areaProjectID != project.ID {
			return domain.Ticket{}, fmt.Errorf("area belongs to another project: %w", ErrConflict)
		}
	}
	if parentID != nil {
		var parentProjectID int64
		var parentKey string
		if err := conn.QueryRowContext(ctx, `SELECT project_id, key FROM tickets WHERE id = ?`, *parentID).Scan(&parentProjectID, &parentKey); err != nil {
			return domain.Ticket{}, notFound(err, "parent ticket")
		}
		if parentProjectID != project.ID {
			return domain.Ticket{}, fmt.Errorf("parent ticket belongs to another project: %w", ErrConflict)
		}
		ticket.ParentKey = &parentKey
	}
	if err := conn.QueryRowContext(ctx, `UPDATE projects SET next_ticket_number = next_ticket_number + 1, updated_at = ? WHERE id = ? AND key = ? RETURNING next_ticket_number - 1`, timestamp(time.Now()), project.ID, project.Key).Scan(&number); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var currentKey string
			err = conn.QueryRowContext(ctx, `SELECT key FROM projects WHERE id = ?`, project.ID).Scan(&currentKey)
			if errors.Is(err, sql.ErrNoRows) {
				return domain.Ticket{}, fmt.Errorf("project: %w", ErrNotFound)
			}
			if err != nil {
				return domain.Ticket{}, fmt.Errorf("check project allocation: %w", err)
			}
			return domain.Ticket{}, fmt.Errorf("project key changed: %w", ErrConflict)
		}
		return domain.Ticket{}, classifyError(err)
	}

	now := time.Now().UTC()
	ticket.Number = number
	ticket.Key = fmt.Sprintf("%s-%d", project.Key, number)
	ticket.Revision = 1
	ticket.CreatedAt = now
	ticket.UpdatedAt = now
	if err := ticket.Validate(); err != nil {
		return domain.Ticket{}, err
	}
	result, err := conn.ExecContext(ctx, `INSERT INTO tickets (project_id, area_id, parent_id, number, key, title, description, type, status, cancellation_reason, priority, flagged, revision, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ticket.ProjectID, ticket.AreaID, ticket.ParentID, ticket.Number, ticket.Key, ticket.Title, ticket.Description, ticket.Type, ticket.Status, ticket.CancellationReason, ticket.Priority, boolInt(ticket.Flagged), ticket.Revision, timestamp(now), timestamp(now))
	if err != nil {
		return domain.Ticket{}, classifyError(err)
	}
	ticket.ID, err = result.LastInsertId()
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("read ticket ID: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return domain.Ticket{}, fmt.Errorf("commit ticket creation: %w", err)
	}
	committed = true
	return ticket, nil
}

type TicketFilter struct {
	AreaSlug  string
	ParentKey string
	Status    domain.Status
	Type      domain.TicketType
	Priority  domain.Priority
	Search    string
	Flagged   *bool
}

func (s *Store) ListTickets(ctx context.Context, projectID int64, filter TicketFilter) ([]domain.Ticket, error) {
	query := ticketSelect
	args := make([]any, 0, 7)
	if filter.AreaSlug != "" {
		query += ` LEFT JOIN areas a ON a.id = t.area_id`
	}
	query += ` WHERE t.project_id = ?`
	args = append(args, projectID)
	if filter.AreaSlug != "" {
		query += ` AND a.slug = ?`
		args = append(args, filter.AreaSlug)
	}
	if filter.ParentKey != "" {
		query += ` AND p.key = ?`
		args = append(args, filter.ParentKey)
	}
	if filter.Status != "" {
		query += ` AND t.status = ?`
		args = append(args, filter.Status)
	}
	if filter.Type != "" {
		query += ` AND t.type = ?`
		args = append(args, filter.Type)
	}
	if filter.Priority != "" {
		query += ` AND t.priority = ?`
		args = append(args, filter.Priority)
	}
	if filter.Search != "" {
		query += ` AND instr(lower(t.title), lower(?)) > 0`
		args = append(args, filter.Search)
	}
	if filter.Flagged != nil {
		query += ` AND t.flagged = ?`
		args = append(args, boolInt(*filter.Flagged))
	}
	query += ` ORDER BY CASE t.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, t.number ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tickets: %w", err)
	}
	defer rows.Close()

	tickets := make([]domain.Ticket, 0)
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tickets: %w", err)
	}
	return tickets, nil
}

func (s *Store) GetTicket(ctx context.Context, projectID int64, key string) (domain.Ticket, error) {
	ticket, err := scanTicket(s.db.QueryRowContext(ctx, ticketSelect+` WHERE t.project_id = ? AND t.key = ?`, projectID, strings.ToUpper(strings.TrimSpace(key))))
	if err != nil {
		return domain.Ticket{}, notFound(err, "ticket")
	}
	return ticket, nil
}

func (s *Store) UpdateTicket(ctx context.Context, ticket domain.Ticket, expectedRevision *int64) (domain.Ticket, error) {
	ticket.Normalize()
	if ticket.ProjectID <= 0 || ticket.ID <= 0 {
		return domain.Ticket{}, fmt.Errorf("invalid ticket")
	}

	conn, err := beginImmediate(ctx, s.db)
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("begin ticket update: %w", err)
	}
	defer conn.Close()
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()

	stored, err := scanTicket(conn.QueryRowContext(ctx, ticketSelect+` WHERE t.id = ? AND t.project_id = ? AND t.key = ?`, ticket.ID, ticket.ProjectID, ticket.Key))
	if err != nil {
		return domain.Ticket{}, notFound(err, "ticket")
	}
	// Immutable fields come from the stored record, so callers need only supply
	// the fields this method changes.
	previousStatus := stored.Status
	previousReason := stored.CancellationReason
	stored.AreaID = ticket.AreaID
	stored.Title = ticket.Title
	stored.Description = ticket.Description
	stored.Type = ticket.Type
	stored.Status = ticket.Status
	stored.CancellationReason = ticket.CancellationReason
	stored.Priority = ticket.Priority
	stored.Flagged = ticket.Flagged
	stored.Normalize()
	if err := stored.Validate(); err != nil {
		return domain.Ticket{}, err
	}
	if stored.AreaID != nil {
		var areaProjectID int64
		if err := conn.QueryRowContext(ctx, `SELECT project_id FROM areas WHERE id = ?`, *stored.AreaID).Scan(&areaProjectID); err != nil {
			return domain.Ticket{}, notFound(err, "area")
		}
		if areaProjectID != stored.ProjectID {
			return domain.Ticket{}, fmt.Errorf("area belongs to another project: %w", ErrConflict)
		}
	}

	now := time.Now().UTC()
	query := `UPDATE tickets SET area_id = ?, title = ?, description = ?, type = ?, status = ?, cancellation_reason = ?, priority = ?, flagged = ?, revision = revision + 1, updated_at = ? WHERE id = ? AND project_id = ? AND key = ?`
	args := []any{stored.AreaID, stored.Title, stored.Description, stored.Type, stored.Status, stored.CancellationReason, stored.Priority, boolInt(stored.Flagged), timestamp(now), stored.ID, stored.ProjectID, stored.Key}
	if expectedRevision != nil {
		query += ` AND revision = ?`
		args = append(args, *expectedRevision)
	}
	result, err := conn.ExecContext(ctx, query, args...)
	if err != nil {
		return domain.Ticket{}, classifyError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("read ticket update result: %w", err)
	}
	if affected == 0 {
		return domain.Ticket{}, fmt.Errorf("ticket revision: %w", ErrConflict)
	}
	if stored.Status == domain.StatusCancelled && (previousStatus != domain.StatusCancelled || !equalOptionalString(previousReason, stored.CancellationReason)) {
		if _, err := conn.ExecContext(ctx, `INSERT INTO ticket_notes (ticket_id, kind, body, actor, created_at) VALUES (?, ?, ?, NULL, ?)`, stored.ID, domain.NoteKindCancellation, *stored.CancellationReason, timestamp(now)); err != nil {
			return domain.Ticket{}, classifyError(err)
		}
	}

	updated, err := scanTicket(conn.QueryRowContext(ctx, ticketSelect+` WHERE t.id = ?`, stored.ID))
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("read updated ticket: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return domain.Ticket{}, fmt.Errorf("commit ticket update: %w", err)
	}
	committed = true
	return updated, nil
}

func (s *Store) AddTicketNote(ctx context.Context, projectID int64, key string, note domain.TicketNote) (domain.TicketNote, error) {
	note.Normalize()
	note.CreatedAt = time.Now().UTC()
	row := s.db.QueryRowContext(ctx, `INSERT INTO ticket_notes (ticket_id, kind, body, actor, created_at)
		SELECT id, ?, ?, ?, ? FROM tickets WHERE project_id = ? AND key = ?
		RETURNING id, ticket_id, kind, body, actor, created_at`, note.Kind, note.Body, note.Actor, timestamp(note.CreatedAt), projectID, strings.ToUpper(strings.TrimSpace(key)))
	created, err := scanTicketNote(row)
	if err != nil {
		return domain.TicketNote{}, notFound(err, "ticket")
	}
	if err := created.Validate(); err != nil {
		return domain.TicketNote{}, err
	}
	return created, nil
}

func (s *Store) ListTicketNotes(ctx context.Context, projectID int64, key string) ([]domain.TicketNote, error) {
	if _, err := s.GetTicket(ctx, projectID, key); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT n.id, n.ticket_id, n.kind, n.body, n.actor, n.created_at
		FROM ticket_notes n JOIN tickets t ON t.id = n.ticket_id
		WHERE t.project_id = ? AND t.key = ? ORDER BY n.created_at ASC, n.id ASC`, projectID, strings.ToUpper(strings.TrimSpace(key)))
	if err != nil {
		return nil, fmt.Errorf("list ticket notes: %w", err)
	}
	defer rows.Close()

	notes := make([]domain.TicketNote, 0)
	for rows.Next() {
		note, err := scanTicketNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ticket notes: %w", err)
	}
	return notes, nil
}

func (s *Store) DeleteTicket(ctx context.Context, projectID int64, key string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM tickets WHERE project_id = ? AND key = ?`, projectID, strings.ToUpper(strings.TrimSpace(key)))
	if err != nil {
		return classifyError(err)
	}
	return requireAffected(result, "ticket")
}

const ticketSelect = `SELECT t.id, t.project_id, t.area_id, t.parent_id, p.key, t.number, t.key, t.title, t.description, t.type, t.status, t.cancellation_reason, t.priority, t.flagged, t.revision, t.created_at, t.updated_at FROM tickets t LEFT JOIN tickets p ON p.id = t.parent_id`

type rowScanner interface {
	Scan(...any) error
}

func scanProject(row rowScanner) (domain.Project, error) {
	var project domain.Project
	if err := row.Scan(&project.ID, &project.Key, &project.Name); err != nil {
		return domain.Project{}, fmt.Errorf("scan project: %w", err)
	}
	return project, nil
}

func scanArea(row rowScanner) (domain.Area, error) {
	var area domain.Area
	if err := row.Scan(&area.ID, &area.ProjectID, &area.Name, &area.Slug); err != nil {
		return domain.Area{}, fmt.Errorf("scan area: %w", err)
	}
	return area, nil
}

func scanTicket(row rowScanner) (domain.Ticket, error) {
	var ticket domain.Ticket
	var areaID, parentID sql.NullInt64
	var parentKey, cancellationReason sql.NullString
	var flagged int
	var createdAt, updatedAt string
	if err := row.Scan(&ticket.ID, &ticket.ProjectID, &areaID, &parentID, &parentKey, &ticket.Number, &ticket.Key, &ticket.Title, &ticket.Description, &ticket.Type, &ticket.Status, &cancellationReason, &ticket.Priority, &flagged, &ticket.Revision, &createdAt, &updatedAt); err != nil {
		return domain.Ticket{}, fmt.Errorf("scan ticket: %w", err)
	}
	if areaID.Valid {
		ticket.AreaID = &areaID.Int64
	}
	if parentID.Valid {
		ticket.ParentID = &parentID.Int64
	}
	if parentKey.Valid {
		ticket.ParentKey = &parentKey.String
	}
	if cancellationReason.Valid {
		ticket.CancellationReason = &cancellationReason.String
	}
	if flagged != 0 && flagged != 1 {
		return domain.Ticket{}, fmt.Errorf("scan ticket: invalid flagged value %d", flagged)
	}
	var err error
	ticket.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("parse ticket creation time: %w", err)
	}
	ticket.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("parse ticket update time: %w", err)
	}
	ticket.CreatedAt = ticket.CreatedAt.UTC()
	ticket.UpdatedAt = ticket.UpdatedAt.UTC()
	ticket.Flagged = flagged == 1
	return ticket, nil
}

func scanTicketNote(row rowScanner) (domain.TicketNote, error) {
	var note domain.TicketNote
	var actor sql.NullString
	var createdAt string
	if err := row.Scan(&note.ID, &note.TicketID, &note.Kind, &note.Body, &actor, &createdAt); err != nil {
		return domain.TicketNote{}, fmt.Errorf("scan ticket note: %w", err)
	}
	if actor.Valid {
		note.Actor = &actor.String
	}
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return domain.TicketNote{}, fmt.Errorf("parse note creation time: %w", err)
	}
	note.CreatedAt = parsed.UTC()
	return note, nil
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func timestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func beginImmediate(ctx context.Context, db *sql.DB) (*sql.Conn, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func notFound(err error, entity string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", entity, ErrNotFound)
	}
	return err
}

func requireAffected(result sql.Result, entity string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read %s deletion result: %w", entity, err)
	}
	if affected == 0 {
		return fmt.Errorf("%s: %w", entity, ErrNotFound)
	}
	return nil
}

func classifyError(err error) error {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "unique constraint failed"):
		return fmt.Errorf("database uniqueness constraint: %w", ErrAlreadyExists)
	case strings.Contains(message, "foreign key constraint failed"):
		return fmt.Errorf("database foreign key constraint: %w", ErrConflict)
	default:
		return err
	}
}
