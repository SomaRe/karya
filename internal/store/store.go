package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/somare/karya/internal/model"
	"gopkg.in/yaml.v3"
)

// --- Projects ---

func WriteProjectConfig(dir string, p *model.Project) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, ".karya.toml"))
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(p)
}

func ReadProjectConfig(dir string) (*model.Project, error) {
	var p model.Project
	_, err := toml.DecodeFile(filepath.Join(dir, ".karya.toml"), &p)
	if err != nil {
		return nil, err
	}
	p.Dir = dir
	return &p, nil
}

func ListProjects(projectsDir string) ([]*model.Project, error) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var projects []*model.Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(projectsDir, e.Name())
		p, err := ReadProjectConfig(dir)
		if err != nil {
			continue
		}
		projects = append(projects, p)
	}
	return projects, nil
}

// --- Epics ---

func CreateEpic(projectDir, name string) (*model.Epic, error) {
	slug := toSlug(name)
	dir := filepath.Join(projectDir, slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &model.Epic{Name: slug, Dir: dir}, nil
}

func ListEpics(projectDir string) ([]*model.Epic, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}
	var epics []*model.Epic
	for _, e := range entries {
		if !e.IsDir() || e.Name() == ".karya.toml" {
			continue
		}
		// skip hidden dirs
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		epics = append(epics, &model.Epic{
			Name: e.Name(),
			Dir:  filepath.Join(projectDir, e.Name()),
		})
	}
	return epics, nil
}

// --- Tickets ---

type ticketFrontmatter struct {
	ID       string           `yaml:"id"`
	Title    string           `yaml:"title"`
	Type     model.TicketType `yaml:"type"`
	Status   model.Status     `yaml:"status"`
	Priority model.Priority   `yaml:"priority"`
	Epic     string           `yaml:"epic"`
	Flagged  bool             `yaml:"flagged"`
}

func WriteTicket(t *model.Ticket) error {
	if err := os.MkdirAll(t.Dir, 0755); err != nil {
		return err
	}
	fm := ticketFrontmatter{
		ID:       t.ID,
		Title:    t.Title,
		Type:     t.Type,
		Status:   t.Status,
		Priority: t.Priority,
		Epic:     t.Epic,
		Flagged:  t.Flagged,
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(0)
	if err := enc.Encode(fm); err != nil {
		return err
	}
	buf.WriteString("---\n")
	if t.Body != "" {
		buf.WriteString("\n")
		buf.WriteString(t.Body)
	}
	return os.WriteFile(filepath.Join(t.Dir, "ticket.md"), buf.Bytes(), 0644)
}

func ReadTicket(ticketDir string) (*model.Ticket, error) {
	path := filepath.Join(ticketDir, "ticket.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("ticket.md missing frontmatter: %s", path)
	}
	rest := content[4:]
	end := strings.Index(rest, "\n---\n")
	if end == -1 {
		return nil, fmt.Errorf("ticket.md frontmatter not closed: %s", path)
	}
	fmRaw := rest[:end]
	body := strings.TrimPrefix(rest[end+5:], "\n")

	var fm ticketFrontmatter
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return nil, err
	}

	info, err := os.Stat(ticketDir)
	if err != nil {
		return nil, err
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	return &model.Ticket{
		ID:       fm.ID,
		Title:    fm.Title,
		Type:     fm.Type,
		Status:   fm.Status,
		Priority: fm.Priority,
		Epic:     fm.Epic,
		Flagged:  fm.Flagged,
		Body:     body,
		Created:  info.ModTime(), // best approximation; birthtime via syscall if needed
		Modified: fileInfo.ModTime(),
		Dir:      ticketDir,
	}, nil
}

func ListTickets(projectDir string) ([]*model.Ticket, error) {
	epics, err := ListEpics(projectDir)
	if err != nil {
		return nil, err
	}
	var tickets []*model.Ticket
	for _, epic := range epics {
		entries, err := os.ReadDir(epic.Dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			t, err := ReadTicket(filepath.Join(epic.Dir, e.Name()))
			if err != nil {
				continue
			}
			tickets = append(tickets, t)
		}
	}
	return tickets, nil
}

func FindTicket(projectDir, id string) (*model.Ticket, error) {
	tickets, err := ListTickets(projectDir)
	if err != nil {
		return nil, err
	}
	id = strings.ToUpper(id)
	for _, t := range tickets {
		if strings.ToUpper(t.ID) == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("ticket %s not found", id)
}

// NextID scans existing tickets and returns the next sequential ID.
func NextID(projectDir, prefix string) (string, error) {
	tickets, err := ListTickets(projectDir)
	if err != nil {
		return "", err
	}
	max := 0
	for _, t := range tickets {
		parts := strings.SplitN(t.ID, "-", 2)
		if len(parts) == 2 {
			n, err := strconv.Atoi(parts[1])
			if err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("%s-%03d", strings.ToUpper(prefix), max+1), nil
}

// --- Helpers ---

func toSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

// SortTickets sorts by priority (high→low) then by ID.
func SortTickets(tickets []*model.Ticket) {
	order := map[model.Priority]int{
		model.PriorityHigh:   0,
		model.PriorityMedium: 1,
		model.PriorityLow:    2,
	}
	sort.Slice(tickets, func(i, j int) bool {
		pi := order[tickets[i].Priority]
		pj := order[tickets[j].Priority]
		if pi != pj {
			return pi < pj
		}
		return tickets[i].ID < tickets[j].ID
	})
}

// unused import guard
var _ = time.Now
