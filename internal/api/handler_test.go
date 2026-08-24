package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/somare/karya/internal/domain"
	"github.com/somare/karya/internal/service"
	"github.com/somare/karya/internal/sqlite"
)

func TestReadOnlyRoutesAndFilters(t *testing.T) {
	h, svc := testHandler(t)
	ctx := context.Background()
	project, err := svc.CreateProject(ctx, "APP", "Application")
	if err != nil {
		t.Fatal(err)
	}
	area, err := svc.CreateArea(ctx, project.Key, "API", "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.CreateTicket(ctx, service.TicketCreateInput{ProjectKey: project.Key, AreaSlug: area.Slug, Title: "Repair API", Description: "Plain text", Type: domain.TicketTypeBug, Priority: domain.PriorityHigh})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateTicket(ctx, service.TicketCreateInput{ProjectKey: project.Key, Title: "Plan release", Type: domain.TicketTypeTask, Priority: domain.PriorityLow})
	if err != nil {
		t.Fatal(err)
	}
	flagged := true
	if _, err := svc.UpdateTicket(ctx, service.TicketUpdateInput{ProjectKey: project.Key, Key: first.Key, Flagged: &flagged, Revision: &first.Revision}); err != nil {
		t.Fatal(err)
	}

	var projects []domain.Project
	requestJSON(t, h, http.MethodGet, "/api/v1/projects", http.StatusOK, &projects)
	if len(projects) != 1 || projects[0] != project {
		t.Fatalf("projects = %#v", projects)
	}

	var gotProject domain.Project
	requestJSON(t, h, http.MethodGet, "/api/v1/projects/APP", http.StatusOK, &gotProject)
	if gotProject != project {
		t.Fatalf("project = %#v, want %#v", gotProject, project)
	}

	var areas []domain.Area
	requestJSON(t, h, http.MethodGet, "/api/v1/projects/APP/areas", http.StatusOK, &areas)
	if len(areas) != 1 || areas[0] != area {
		t.Fatalf("areas = %#v", areas)
	}

	var tickets []domain.Ticket
	requestJSON(t, h, http.MethodGet, "/api/v1/tickets?project=APP&area=api&status=backlog&type=bug&priority=high&search=repair&flagged=true", http.StatusOK, &tickets)
	if len(tickets) != 1 || tickets[0].Key != first.Key || !tickets[0].Flagged {
		t.Fatalf("filtered tickets = %#v", tickets)
	}

	requestJSON(t, h, http.MethodGet, "/api/v1/tickets?project=APP", http.StatusOK, &tickets)
	if len(tickets) != 2 {
		t.Fatalf("unfiltered tickets = %#v", tickets)
	}

	requestJSON(t, h, http.MethodGet, "/api/v1/tickets?project=APP&flagged=false", http.StatusOK, &tickets)
	if len(tickets) != 1 || tickets[0].Key != second.Key {
		t.Fatalf("unflagged tickets = %#v", tickets)
	}

	var ticket domain.Ticket
	requestJSON(t, h, http.MethodGet, "/api/v1/tickets/APP-1?project=APP", http.StatusOK, &ticket)
	if ticket.Key != first.Key || ticket.Description != "Plain text" {
		t.Fatalf("ticket = %#v", ticket)
	}
}

func TestRequestErrorsAndNoWrites(t *testing.T) {
	h, svc := testHandler(t)
	ctx := context.Background()
	if _, err := svc.CreateProject(ctx, "APP", "Application"); err != nil {
		t.Fatal(err)
	}

	assertError(t, h, http.MethodGet, "/api/v1/projects/NOPE", http.StatusNotFound, "not_found")
	assertError(t, h, http.MethodGet, "/api/v1/tickets", http.StatusBadRequest, "invalid_request")
	assertError(t, h, http.MethodGet, "/api/v1/tickets/APP-1", http.StatusBadRequest, "invalid_request")
	assertError(t, h, http.MethodGet, "/api/v1/tickets/not-a-ticket?project=APP", http.StatusBadRequest, "invalid_request")
	assertError(t, h, http.MethodGet, "/api/v1/tickets?project=APP&flagged=yes", http.StatusBadRequest, "invalid_request")
	assertError(t, h, http.MethodGet, "/api/v1/tickets?project=APP&type=scope", http.StatusBadRequest, "invalid_request")

	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/projects", nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST response = %d, Allow %q", recorder.Code, recorder.Header().Get("Allow"))
	}
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "invalid_request" {
		t.Fatalf("POST error code = %q", response.Error.Code)
	}

	recorder = httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest(http.MethodHead, "/api/v1/projects", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("HEAD response = %d, Content-Type %q", recorder.Code, recorder.Header().Get("Content-Type"))
	}

	var projects []domain.Project
	requestJSON(t, h, http.MethodGet, "/api/v1/projects", http.StatusOK, &projects)
	if len(projects) != 1 {
		t.Fatalf("read-only API changed projects: %#v", projects)
	}

	recorder = httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "Karya" {
		t.Fatalf("static response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestTicketNotesParentAndCancellationRoutes(t *testing.T) {
	h, svc := testHandler(t)
	ctx := context.Background()
	if _, err := svc.CreateProject(ctx, "APP", "Application"); err != nil {
		t.Fatal(err)
	}
	parent, err := svc.CreateTicket(ctx, service.TicketCreateInput{ProjectKey: "APP", Title: "Parent"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateTicket(ctx, service.TicketCreateInput{ProjectKey: "APP", ParentKey: parent.Key, Title: "Child"})
	if err != nil {
		t.Fatal(err)
	}
	actor := "agent-a"
	if _, err := svc.AddTicketNote(ctx, "APP", child.Key, "Finding", &actor); err != nil {
		t.Fatal(err)
	}
	status := domain.StatusCancelled
	reason := "Superseded"
	child, err = svc.UpdateTicket(ctx, service.TicketUpdateInput{ProjectKey: "APP", Key: child.Key, Status: &status, Reason: &reason, Revision: &child.Revision})
	if err != nil {
		t.Fatal(err)
	}

	var tickets []domain.Ticket
	requestJSON(t, h, http.MethodGet, "/api/v1/tickets?project=APP&parent=APP-1&status=cancelled", http.StatusOK, &tickets)
	if len(tickets) != 1 || tickets[0].ParentKey == nil || *tickets[0].ParentKey != parent.Key || tickets[0].CancellationReason == nil {
		t.Fatalf("tickets = %+v", tickets)
	}
	var notes []domain.TicketNote
	requestJSON(t, h, http.MethodGet, "/api/v1/tickets/APP-2/notes?project=APP", http.StatusOK, &notes)
	if len(notes) != 2 || notes[0].Actor == nil || *notes[0].Actor != actor || notes[1].Kind != domain.NoteKindCancellation {
		t.Fatalf("notes = %+v", notes)
	}
	assertError(t, h, http.MethodGet, "/api/v1/tickets/APP-2/unknown?project=APP", http.StatusNotFound, "not_found")
}

func testHandler(t *testing.T) (*Handler, *service.Service) {
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
	return NewHandler(staticFiles(), service.New(store)), service.New(store)
}

func staticFiles() fs.FS {
	return fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("Karya")}}
}

func requestJSON(t *testing.T, h http.Handler, method, target string, wantStatus int, value any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	if recorder.Code != wantStatus {
		t.Fatalf("%s %s status = %d, body = %s", method, target, recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", recorder.Header().Get("Content-Type"))
	}
	if err := json.NewDecoder(recorder.Body).Decode(value); err != nil {
		t.Fatal(err)
	}
}

func assertError(t *testing.T, h http.Handler, method, target string, wantStatus int, wantCode string) {
	t.Helper()
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	requestJSON(t, h, method, target, wantStatus, &response)
	if response.Error.Code != wantCode {
		t.Fatalf("%s %s error code = %q, want %q", method, target, response.Error.Code, wantCode)
	}
}
