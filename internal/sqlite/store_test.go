package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/somare/karya/internal/domain"
)

func TestOpenMigratesAndConfiguresDatabase(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	var version, foreignKeys, busyTimeout int
	var journalMode string
	if err := store.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if version != 2 || foreignKeys != 1 || busyTimeout != 5000 || journalMode != "wal" {
		t.Fatalf("version=%d foreign_keys=%d busy_timeout=%d journal_mode=%q", version, foreignKeys, busyTimeout, journalMode)
	}
	if store.db.Stats().MaxOpenConnections != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", store.db.Stats().MaxOpenConnections)
	}
}

func TestOpenTreatsDatabasePathAsLiteralAndProtectsNewStorage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "karya?mode=memory#.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.CreateProject(context.Background(), domain.Project{Key: "APP", Name: "Application"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("database was not created at its literal path: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("database permissions = %o, want 600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("database directory permissions = %o, want 700", dirInfo.Mode().Perm())
	}
}

func TestProjectAndAreaCRUD(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	zulu := createProject(t, store, "zulu", " Zulu Project ")
	alpha := createProject(t, store, "alpha", "Alpha Project")
	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0].ID != alpha.ID || projects[1].ID != zulu.ID {
		t.Fatalf("ListProjects() = %+v", projects)
	}
	gotProject, err := store.GetProject(ctx, " alpha ")
	if err != nil || gotProject != alpha {
		t.Fatalf("GetProject() = %+v, %v", gotProject, err)
	}
	if _, err := store.CreateProject(ctx, domain.Project{Key: "ALPHA", Name: "Duplicate"}); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate project error = %v, want ErrAlreadyExists", err)
	}

	areaB, err := store.CreateArea(ctx, domain.Area{ProjectID: alpha.ID, Name: "Beta Area", Slug: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	areaA, err := store.CreateArea(ctx, domain.Area{ProjectID: alpha.ID, Name: "Alpha Area", Slug: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	areas, err := store.ListAreas(ctx, alpha.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(areas) != 2 || areas[0].ID != areaA.ID || areas[1].ID != areaB.ID {
		t.Fatalf("ListAreas() = %+v", areas)
	}
	if got, err := store.GetArea(ctx, alpha.ID, "alpha"); err != nil || got != areaA {
		t.Fatalf("GetArea() = %+v, %v", got, err)
	}
	if _, err := store.CreateArea(ctx, domain.Area{ProjectID: alpha.ID, Name: "Again", Slug: "alpha"}); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate area error = %v, want ErrAlreadyExists", err)
	}
	if err := store.DeleteArea(ctx, alpha.ID, "beta"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteProject(ctx, zulu.Key); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteProject(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing project error = %v, want ErrNotFound", err)
	}
}

func TestTicketAllocationFiltersAndOptionalArea(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	project := createProject(t, store, "app", "Application")
	area, err := store.CreateArea(ctx, domain.Area{ProjectID: project.ID, Name: "API", Slug: "api"})
	if err != nil {
		t.Fatal(err)
	}

	high := createTicket(t, store, project, nil, "Fix Search", domain.TicketTypeBug, domain.PriorityHigh)
	medium := createTicket(t, store, project, &area.ID, "Add API", domain.TicketTypeTask, domain.PriorityMedium)
	low := createTicket(t, store, project, &area.ID, "Plan release", domain.TicketTypeSpike, domain.PriorityLow)
	if high.Number != 1 || high.Key != "APP-1" || medium.Number != 2 || low.Number != 3 {
		t.Fatalf("ticket allocation: high=%+v medium=%+v low=%+v", high, medium, low)
	}
	if high.AreaID != nil {
		t.Fatalf("unassigned ticket area ID = %v", *high.AreaID)
	}
	if high.Status != domain.StatusBacklog || high.Revision != 1 || high.CreatedAt.Location() != timeUTC {
		t.Fatalf("created ticket = %+v", high)
	}

	all, err := store.ListTickets(ctx, project.ID, TicketFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].ID != high.ID || all[1].ID != medium.ID || all[2].ID != low.ID {
		t.Fatalf("priority order = %+v", all)
	}
	api, err := store.ListTickets(ctx, project.ID, TicketFilter{AreaSlug: "api", Type: domain.TicketTypeTask, Search: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if len(api) != 1 || api[0].ID != medium.ID {
		t.Fatalf("filtered tickets = %+v", api)
	}
	flagged := true
	medium.Flagged = true
	if _, err := store.UpdateTicket(ctx, medium, &medium.Revision); err != nil {
		t.Fatal(err)
	}
	flaggedTickets, err := store.ListTickets(ctx, project.ID, TicketFilter{Flagged: &flagged})
	if err != nil {
		t.Fatal(err)
	}
	if len(flaggedTickets) != 1 || flaggedTickets[0].ID != medium.ID {
		t.Fatalf("flagged tickets = %+v", flaggedTickets)
	}
}

func TestTicketCrossProjectAreaRevisionAndForeignKeyProtection(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	first := createProject(t, store, "one", "One")
	second := createProject(t, store, "two", "Two")
	area, err := store.CreateArea(ctx, domain.Area{ProjectID: second.ID, Name: "Other", Slug: "other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateTicket(ctx, first, &area.ID, nil, "Wrong area", "", domain.TicketTypeTask, domain.PriorityLow); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-project area error = %v, want ErrConflict", err)
	}

	ticket := createTicket(t, store, second, &area.ID, "Update me", domain.TicketTypeTask, domain.PriorityMedium)
	ticket.Title = "Updated"
	updated, err := store.UpdateTicket(ctx, ticket, &ticket.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Title != "Updated" || !updated.UpdatedAt.After(ticket.UpdatedAt) && !updated.UpdatedAt.Equal(ticket.UpdatedAt) {
		t.Fatalf("updated ticket = %+v", updated)
	}
	if _, err := store.UpdateTicket(ctx, ticket, &ticket.Revision); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale revision error = %v, want ErrConflict", err)
	}
	if err := store.DeleteArea(ctx, second.ID, area.Slug); !errors.Is(err, ErrConflict) {
		t.Fatalf("delete referenced area error = %v, want ErrConflict", err)
	}
	if err := store.DeleteProject(ctx, second.Key); !errors.Is(err, ErrConflict) {
		t.Fatalf("delete referenced project error = %v, want ErrConflict", err)
	}
	if err := store.DeleteTicket(ctx, second.ID, ticket.Key); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteArea(ctx, second.ID, area.Slug); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteProject(ctx, second.Key); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentTicketCreationAllocatesUniqueNumbers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "karya.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })

	project := createProject(t, first, "APP", "Application")
	stores := []*Store{first, second}
	results := make(chan domain.Ticket, 20)
	errs := make(chan error, 20)
	var group sync.WaitGroup
	for _, store := range stores {
		group.Add(1)
		go func(store *Store) {
			defer group.Done()
			for range 10 {
				ticket, err := store.CreateTicket(context.Background(), project, nil, nil, "Concurrent ticket", "", domain.TicketTypeTask, domain.PriorityMedium)
				if err != nil {
					errs <- err
					return
				}
				results <- ticket
			}
		}(store)
	}
	group.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	numbers := make(map[int64]bool)
	for ticket := range results {
		if numbers[ticket.Number] {
			t.Fatalf("ticket number %d was allocated twice", ticket.Number)
		}
		numbers[ticket.Number] = true
	}
	if len(numbers) != 20 {
		t.Fatalf("created %d tickets, want 20", len(numbers))
	}
}

func TestParentNotesCancellationAndAreaMove(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	project := createProject(t, store, "APP", "Application")
	firstArea, err := store.CreateArea(ctx, domain.Area{ProjectID: project.ID, Name: "First", Slug: "first"})
	if err != nil {
		t.Fatal(err)
	}
	secondArea, err := store.CreateArea(ctx, domain.Area{ProjectID: project.ID, Name: "Second", Slug: "second"})
	if err != nil {
		t.Fatal(err)
	}
	parent := createTicket(t, store, project, &firstArea.ID, "Parent", domain.TicketTypeTask, domain.PriorityMedium)
	child, err := store.CreateTicket(ctx, project, &firstArea.ID, &parent.ID, "Child", "Description", domain.TicketTypeTask, domain.PriorityHigh)
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentKey == nil || *child.ParentKey != parent.Key {
		t.Fatalf("child parent = %+v", child)
	}
	children, err := store.ListTickets(ctx, project.ID, TicketFilter{ParentKey: parent.Key})
	if err != nil || len(children) != 1 || children[0].Key != child.Key {
		t.Fatalf("children = %+v, %v", children, err)
	}
	if err := store.DeleteTicket(ctx, project.ID, parent.Key); !errors.Is(err, ErrConflict) {
		t.Fatalf("delete parent error = %v, want conflict", err)
	}

	actor := "agent-a"
	note, err := store.AddTicketNote(ctx, project.ID, child.Key, domain.TicketNote{Kind: domain.NoteKindNote, Body: "Found the cause", Actor: &actor})
	if err != nil {
		t.Fatal(err)
	}
	if note.Actor == nil || *note.Actor != actor || note.CreatedAt.IsZero() || note.CreatedAt.Location() != time.UTC {
		t.Fatalf("created note = %+v", note)
	}

	child.AreaID = &secondArea.ID
	reason := "Replaced by a smaller child"
	child.Status = domain.StatusCancelled
	child.CancellationReason = &reason
	updated, err := store.UpdateTicket(ctx, child, &child.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if updated.AreaID == nil || *updated.AreaID != secondArea.ID || updated.Status != domain.StatusCancelled || updated.Revision != 2 {
		t.Fatalf("updated ticket = %+v", updated)
	}
	notes, err := store.ListTicketNotes(ctx, project.ID, child.Key)
	if err != nil || len(notes) != 2 || notes[1].Kind != domain.NoteKindCancellation || notes[1].Body != reason {
		t.Fatalf("notes = %+v, %v", notes, err)
	}
	if _, err := store.UpdateTicket(ctx, child, &child.Revision); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale cancellation error = %v", err)
	}
	notes, err = store.ListTicketNotes(ctx, project.ID, child.Key)
	if err != nil || len(notes) != 2 {
		t.Fatalf("stale update appended note: %+v, %v", notes, err)
	}
}

func TestMigratesV1ScopeTickets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "karya.db")
	db, err := sql.Open("sqlite", databaseURI(path))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, statement := range schemaV1 {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	now := timestamp(time.Now())
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, key, name, next_ticket_number, created_at, updated_at) VALUES (1, 'APP', 'Application', 2, ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO tickets (project_id, number, key, title, description, type, status, priority, flagged, revision, created_at, updated_at) VALUES (1, 1, 'APP-1', 'Legacy', '', 'scope', 'backlog', 'medium', 0, 1, ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ticket, err := store.GetTicket(ctx, 1, "APP-1")
	if err != nil || ticket.Type != domain.TicketTypeTask {
		t.Fatalf("migrated ticket = %+v, %v", ticket, err)
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "nested", "karya.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return store
}

func createProject(t *testing.T, store *Store, key, name string) domain.Project {
	t.Helper()
	project, err := store.CreateProject(context.Background(), domain.Project{Key: key, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func createTicket(t *testing.T, store *Store, project domain.Project, areaID *int64, title string, ticketType domain.TicketType, priority domain.Priority) domain.Ticket {
	t.Helper()
	ticket, err := store.CreateTicket(context.Background(), project, areaID, nil, title, "Description", ticketType, priority)
	if err != nil {
		t.Fatal(err)
	}
	return ticket
}

var timeUTC = time.UTC
