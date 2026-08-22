package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/somare/karya/internal/config"
	"github.com/somare/karya/internal/model"
	"github.com/somare/karya/internal/store"
)

type Handler struct {
	mux *http.ServeMux
}

func NewHandler(staticFS fs.FS) *Handler {
	h := &Handler{mux: http.NewServeMux()}
	h.mux.HandleFunc("GET /api/projects", h.getProjects)
	h.mux.HandleFunc("GET /api/epics", h.getEpics)
	h.mux.HandleFunc("GET /api/tickets/{id}", h.getTicket)
	h.mux.HandleFunc("GET /api/tickets", h.getTickets)
	h.mux.Handle("/", http.FileServer(http.FS(staticFS)))
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func resolveProjectDir(r *http.Request) (string, error) {
	projectName := r.URL.Query().Get("project")
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	if projectName == "" {
		projectName = cfg.ActiveProject
	}
	if projectName == "" {
		return "", fmt.Errorf("no project specified and no active project set")
	}
	dir := filepath.Join(config.ProjectsDir(), projectName)
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("project %q not found", projectName)
	}
	return dir, nil
}

// GET /api/projects
func (h *Handler) getProjects(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projects, err := store.ListProjects(config.ProjectsDir())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type projectInfo struct {
		Name   string `json:"name"`
		Prefix string `json:"prefix"`
		Active bool   `json:"active"`
	}
	resp := make([]projectInfo, 0, len(projects))
	for _, p := range projects {
		resp = append(resp, projectInfo{
			Name:   p.Name,
			Prefix: p.Prefix,
			Active: p.Name == cfg.ActiveProject,
		})
	}
	writeJSON(w, resp)
}

// GET /api/epics?project=<name>
func (h *Handler) getEpics(w http.ResponseWriter, r *http.Request) {
	projectDir, err := resolveProjectDir(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	epics, err := store.ListEpics(projectDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type epicInfo struct {
		Name string `json:"name"`
	}
	resp := make([]epicInfo, 0, len(epics))
	for _, e := range epics {
		resp = append(resp, epicInfo{Name: e.Name})
	}
	writeJSON(w, resp)
}

// GET /api/tickets?project=<name>&epic=<slug>&status=<status>&flagged=true
func (h *Handler) getTickets(w http.ResponseWriter, r *http.Request) {
	projectDir, err := resolveProjectDir(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tickets, err := store.ListTickets(projectDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	epic := q.Get("epic")
	status := q.Get("status")
	flaggedOnly := q.Get("flagged") == "true"

	filtered := make([]*model.Ticket, 0)
	for _, t := range tickets {
		if epic != "" && t.Epic != epic {
			continue
		}
		if status != "" && string(t.Status) != status {
			continue
		}
		if flaggedOnly && !t.Flagged {
			continue
		}
		filtered = append(filtered, t)
	}
	store.SortTickets(filtered)
	writeJSON(w, filtered)
}

// GET /api/tickets/{id}?project=<name>
func (h *Handler) getTicket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	projectDir, err := resolveProjectDir(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	t, err := store.FindTicket(projectDir, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, t)
}
