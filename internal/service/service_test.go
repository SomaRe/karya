package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/somare/karya/internal/domain"
	"github.com/somare/karya/internal/sqlite"
)

func TestProjectAndAreaKeyAndSlugOperations(t *testing.T) {
	service := testService(t)
	ctx := context.Background()

	project, err := service.CreateProject(ctx, " app ", " Application ")
	if err != nil {
		t.Fatal(err)
	}
	if project.Key != "APP" || project.Name != "Application" {
		t.Fatalf("CreateProject() = %+v", project)
	}
	area, err := service.CreateArea(ctx, "app", " API Platform ", " API / V2 ")
	if err != nil {
		t.Fatal(err)
	}
	if area.Slug != "api-v2" {
		t.Fatalf("CreateArea() slug = %q", area.Slug)
	}
	derived, err := service.CreateArea(ctx, "APP", " Mobile Apps ", "")
	if err != nil {
		t.Fatal(err)
	}
	if derived.Slug != "mobile-apps" {
		t.Fatalf("derived slug = %q", derived.Slug)
	}
	got, err := service.GetArea(ctx, " APP ", "API / V2")
	if err != nil || got.ID != area.ID {
		t.Fatalf("GetArea() = %+v, %v", got, err)
	}
	areas, err := service.ListAreas(ctx, "app")
	if err != nil || len(areas) != 2 {
		t.Fatalf("ListAreas() = %+v, %v", areas, err)
	}
	if _, err := service.GetProject(ctx, "BAD-KEY"); err == nil {
		t.Fatal("GetProject accepted an invalid key")
	}
	if err := service.DeleteArea(ctx, "app", "mobile apps"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateTicketDefaultsAreaAndProjectScoping(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	createProject(t, service, "APP")
	createProject(t, service, "OTHER")
	area, err := service.CreateArea(ctx, "APP", "API", "")
	if err != nil {
		t.Fatal(err)
	}

	defaulted, err := service.CreateTicket(ctx, TicketCreateInput{ProjectKey: " app ", Title: " Default ticket "})
	if err != nil {
		t.Fatal(err)
	}
	if defaulted.Type != domain.TicketTypeTask || defaulted.Priority != domain.PriorityMedium || defaulted.AreaID != nil {
		t.Fatalf("default ticket = %+v", defaulted)
	}
	assigned, err := service.CreateTicket(ctx, TicketCreateInput{
		ProjectKey: "APP", AreaSlug: " API ", Title: "Assigned", Type: " BUG ", Priority: " HIGH ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if assigned.AreaID == nil || *assigned.AreaID != area.ID || assigned.Type != domain.TicketTypeBug || assigned.Priority != domain.PriorityHigh {
		t.Fatalf("assigned ticket = %+v", assigned)
	}
	if _, err := service.CreateTicket(ctx, TicketCreateInput{ProjectKey: "APP", AreaSlug: "missing", Title: "No area"}); !errors.Is(err, sqlite.ErrNotFound) {
		t.Fatalf("missing area error = %v, want ErrNotFound", err)
	}
	if _, err := service.GetTicket(ctx, "OTHER", assigned.Key); !errors.Is(err, sqlite.ErrNotFound) {
		t.Fatalf("cross-project ticket error = %v, want ErrNotFound", err)
	}
	if _, err := service.CreateTicket(ctx, TicketCreateInput{ProjectKey: "APP", Title: "Bad", Type: "feature"}); err == nil {
		t.Fatal("CreateTicket accepted an invalid type")
	}
}

func TestListTicketsValidatesAndNormalizesFilters(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	createProject(t, service, "APP")
	if _, err := service.CreateArea(ctx, "APP", "API", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateTicket(ctx, TicketCreateInput{ProjectKey: "APP", AreaSlug: "api", Title: "Search API", Type: "bug", Priority: "high"}); err != nil {
		t.Fatal(err)
	}
	flagged := false
	tickets, err := service.ListTickets(ctx, TicketListInput{ProjectKey: " app ", AreaSlug: " API ", Type: " BUG ", Priority: " HIGH ", Status: " BACKLOG ", Search: "  search  ", Flagged: &flagged})
	if err != nil || len(tickets) != 1 {
		t.Fatalf("ListTickets() = %+v, %v", tickets, err)
	}
	for _, input := range []TicketListInput{
		{ProjectKey: "APP", Status: "next"},
		{ProjectKey: "APP", Type: "feature"},
		{ProjectKey: "APP", Priority: "urgent"},
	} {
		if _, err := service.ListTickets(ctx, input); err == nil {
			t.Fatalf("ListTickets accepted invalid filter %+v", input)
		}
	}
}

func TestUpdateTicketPartialPatchAndRevision(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	createProject(t, service, "APP")
	ticket, err := service.CreateTicket(ctx, TicketCreateInput{ProjectKey: "APP", Title: "Original", Description: "Keep me"})
	if err != nil {
		t.Fatal(err)
	}
	title := " Updated title "
	flagged := true
	updated, err := service.UpdateTicket(ctx, TicketUpdateInput{ProjectKey: "APP", Key: " app-1 ", Title: &title, Flagged: &flagged, Revision: &ticket.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated title" || updated.Description != "Keep me" || !updated.Flagged || updated.Revision != 2 {
		t.Fatalf("UpdateTicket() = %+v", updated)
	}
	if _, err := service.UpdateTicket(ctx, TicketUpdateInput{ProjectKey: "APP", Key: ticket.Key}); err == nil {
		t.Fatal("UpdateTicket accepted an empty patch")
	}
	empty := "  "
	if _, err := service.UpdateTicket(ctx, TicketUpdateInput{ProjectKey: "APP", Key: ticket.Key, Title: &empty}); err == nil {
		t.Fatal("UpdateTicket accepted an empty title")
	}
	badType := domain.TicketType(" FEATURE ")
	if _, err := service.UpdateTicket(ctx, TicketUpdateInput{ProjectKey: "APP", Key: ticket.Key, Type: &badType}); err == nil {
		t.Fatal("UpdateTicket accepted an invalid type")
	}
	zero := int64(0)
	if _, err := service.UpdateTicket(ctx, TicketUpdateInput{ProjectKey: "APP", Key: ticket.Key, Flagged: &flagged, Revision: &zero}); err == nil {
		t.Fatal("UpdateTicket accepted revision zero")
	}
	staleTitle := "Stale"
	if _, err := service.UpdateTicket(ctx, TicketUpdateInput{ProjectKey: "APP", Key: ticket.Key, Title: &staleTitle, Revision: &ticket.Revision}); !errors.Is(err, sqlite.ErrConflict) {
		t.Fatalf("stale revision error = %v, want ErrConflict", err)
	}
}

func TestTicketEvolutionParentNotesCancellationAndAreaMove(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	createProject(t, service, "APP")
	if _, err := service.CreateArea(ctx, "APP", "First", ""); err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateArea(ctx, "APP", "Second", "")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := service.CreateTicket(ctx, TicketCreateInput{ProjectKey: "APP", AreaSlug: "first", Title: "Broad work"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := service.CreateTicket(ctx, TicketCreateInput{ProjectKey: "APP", AreaSlug: "first", ParentKey: parent.Key, Title: "Split work"})
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentKey == nil || *child.ParentKey != parent.Key {
		t.Fatalf("child = %+v", child)
	}
	children, err := service.ListTickets(ctx, TicketListInput{ProjectKey: "APP", ParentKey: parent.Key})
	if err != nil || len(children) != 1 || children[0].Key != child.Key {
		t.Fatalf("children = %+v, %v", children, err)
	}

	actor := "agent-a"
	if _, err := service.AddTicketNote(ctx, "APP", child.Key, "Observed behavior", &actor); err != nil {
		t.Fatal(err)
	}
	cancelled := domain.StatusCancelled
	reason := "No longer useful"
	area := second.Slug
	updated, err := service.UpdateTicket(ctx, TicketUpdateInput{ProjectKey: "APP", Key: child.Key, AreaSlug: &area, Status: &cancelled, Reason: &reason, Revision: &child.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if updated.AreaID == nil || *updated.AreaID != second.ID || updated.CancellationReason == nil || *updated.CancellationReason != reason {
		t.Fatalf("updated = %+v", updated)
	}
	notes, err := service.ListTicketNotes(ctx, "APP", child.Key)
	if err != nil || len(notes) != 2 || notes[1].Kind != domain.NoteKindCancellation {
		t.Fatalf("notes = %+v, %v", notes, err)
	}

	if _, err := service.UpdateTicket(ctx, TicketUpdateInput{ProjectKey: "APP", Key: parent.Key, Status: &cancelled, Revision: &parent.Revision}); err == nil {
		t.Fatal("cancellation without reason succeeded")
	}
	backlog := domain.StatusBacklog
	reopened, err := service.UpdateTicket(ctx, TicketUpdateInput{ProjectKey: "APP", Key: child.Key, Status: &backlog, Revision: &updated.Revision})
	if err != nil || reopened.CancellationReason != nil {
		t.Fatalf("reopened = %+v, %v", reopened, err)
	}
	if _, err := service.CreateTicket(ctx, TicketCreateInput{ProjectKey: "APP", Title: "Old type", Type: "scope"}); err == nil {
		t.Fatal("scope ticket creation succeeded")
	}
}

func testService(t *testing.T) *Service {
	t.Helper()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "karya.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return New(store)
}

func createProject(t *testing.T, service *Service, key string) domain.Project {
	t.Helper()
	project, err := service.CreateProject(context.Background(), key, key+" Project")
	if err != nil {
		t.Fatal(err)
	}
	return project
}
