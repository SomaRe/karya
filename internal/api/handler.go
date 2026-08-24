// Package api provides Karya's versioned, read-only HTTP API.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/somare/karya/internal/domain"
	"github.com/somare/karya/internal/service"
	"github.com/somare/karya/internal/sqlite"
)

type Handler struct {
	static http.Handler
	svc    *service.Service
}

// NewHandler serves the read-only API and the embedded static UI.
func NewHandler(staticFS fs.FS, svc *service.Service) *Handler {
	return &Handler{static: http.FileServer(http.FS(staticFS)), svc: svc}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/v1" || strings.HasPrefix(r.URL.Path, "/api/v1/") {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "invalid_request", "only GET and HEAD are permitted")
			return
		}
		h.serveAPI(w, r)
		return
	}
	h.static.ServeHTTP(w, r)
}

func (h *Handler) serveAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/api/v1/projects":
		h.listProjects(w, r)
	case strings.HasPrefix(path, "/api/v1/projects/"):
		h.projectRoute(w, r, strings.TrimPrefix(path, "/api/v1/projects/"))
	case path == "/api/v1/tickets":
		h.listTickets(w, r)
	case strings.HasPrefix(path, "/api/v1/tickets/"):
		h.ticketRoute(w, r, strings.TrimPrefix(path, "/api/v1/tickets/"))
	default:
		writeError(w, http.StatusNotFound, "not_found", "API route not found")
	}
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.svc.ListProjects(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (h *Handler) projectRoute(w http.ResponseWriter, r *http.Request, route string) {
	key, suffix, hasSuffix := strings.Cut(route, "/")
	if key == "" || strings.Contains(suffix, "/") || (hasSuffix && suffix != "areas") {
		writeError(w, http.StatusNotFound, "not_found", "API route not found")
		return
	}
	if err := validateProjectKey(key); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if suffix == "areas" {
		areas, err := h.svc.ListAreas(r.Context(), key)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, areas)
		return
	}
	project, err := h.svc.GetProject(r.Context(), key)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) listTickets(w http.ResponseWriter, r *http.Request) {
	input, err := ticketListInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	tickets, err := h.svc.ListTickets(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tickets)
}

func (h *Handler) ticketRoute(w http.ResponseWriter, r *http.Request, route string) {
	key, suffix, hasSuffix := strings.Cut(route, "/")
	if key == "" || strings.Contains(suffix, "/") || (hasSuffix && suffix != "notes") {
		writeError(w, http.StatusNotFound, "not_found", "API route not found")
		return
	}
	projectKey := r.URL.Query().Get("project")
	if err := validateProjectKey(projectKey); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateTicketKey(key); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if suffix == "notes" {
		notes, err := h.svc.ListTicketNotes(r.Context(), projectKey, key)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, notes)
		return
	}
	ticket, err := h.svc.GetTicket(r.Context(), projectKey, key)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

func ticketListInput(r *http.Request) (service.TicketListInput, error) {
	q := r.URL.Query()
	input := service.TicketListInput{
		ProjectKey: q.Get("project"),
		AreaSlug:   q.Get("area"),
		ParentKey:  q.Get("parent"),
		Status:     q.Get("status"),
		Type:       q.Get("type"),
		Priority:   q.Get("priority"),
		Search:     q.Get("search"),
	}
	if err := validateProjectKey(input.ProjectKey); err != nil {
		return service.TicketListInput{}, err
	}
	if input.AreaSlug != "" {
		if _, err := domain.Slugify(input.AreaSlug); err != nil {
			return service.TicketListInput{}, err
		}
	}
	if !validStatus(input.Status) {
		return service.TicketListInput{}, errors.New("invalid ticket status")
	}
	if !validType(input.Type) {
		return service.TicketListInput{}, errors.New("invalid ticket type")
	}
	if !validPriority(input.Priority) {
		return service.TicketListInput{}, errors.New("invalid ticket priority")
	}
	if raw, present := q["flagged"]; present {
		if len(raw) != 1 {
			return service.TicketListInput{}, errors.New("flagged must be true or false")
		}
		if raw[0] != "true" && raw[0] != "false" {
			return service.TicketListInput{}, errors.New("flagged must be true or false")
		}
		flagged := raw[0] == "true"
		input.Flagged = &flagged
	}
	return input, nil
}

func validateProjectKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("project is required")
	}
	return domain.ValidateProjectKey(domain.NormalizeProjectKey(key))
}

func validStatus(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "" || domain.Status(value).Valid()
}

func validType(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "" || domain.TicketType(value).Valid()
}

func validPriority(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "" || domain.Priority(value).Valid()
}

func validateTicketKey(key string) error {
	project, number, found := strings.Cut(strings.ToUpper(strings.TrimSpace(key)), "-")
	if !found || domain.ValidateProjectKey(project) != nil || number == "" {
		return errors.New("ticket key must use the format PROJECT-NUMBER")
	}
	parsed, err := strconv.ParseInt(number, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != number {
		return errors.New("ticket key must use the format PROJECT-NUMBER")
	}
	return nil
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sqlite.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, sqlite.ErrConflict), errors.Is(err, sqlite.ErrAlreadyExists):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
