// Package service provides Karya's application-level, key-based operations.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/somare/karya/internal/domain"
	"github.com/somare/karya/internal/sqlite"
)

// Service coordinates domain validation with SQLite persistence.
type Service struct {
	store *sqlite.Store
}

// New creates a Service backed by store.
func New(store *sqlite.Store) *Service {
	return &Service{store: store}
}

func (s *Service) CreateProject(ctx context.Context, key, name string) (domain.Project, error) {
	project := domain.Project{Key: domain.NormalizeProjectKey(key), Name: domain.NormalizeName(name)}
	if err := project.Validate(); err != nil {
		return domain.Project{}, err
	}
	return s.store.CreateProject(ctx, project)
}

func (s *Service) ListProjects(ctx context.Context) ([]domain.Project, error) {
	return s.store.ListProjects(ctx)
}

func (s *Service) GetProject(ctx context.Context, key string) (domain.Project, error) {
	key, err := normalizeProjectKey(key)
	if err != nil {
		return domain.Project{}, err
	}
	return s.store.GetProject(ctx, key)
}

func (s *Service) DeleteProject(ctx context.Context, key string) error {
	key, err := normalizeProjectKey(key)
	if err != nil {
		return err
	}
	return s.store.DeleteProject(ctx, key)
}

func (s *Service) CreateArea(ctx context.Context, projectKey, name, slug string) (domain.Area, error) {
	project, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return domain.Area{}, err
	}
	if slug != "" {
		slug, err = domain.Slugify(slug)
		if err != nil {
			return domain.Area{}, err
		}
	}

	area := domain.Area{ProjectID: project.ID, Name: domain.NormalizeName(name), Slug: slug}
	if err := area.Normalize(); err != nil {
		return domain.Area{}, err
	}
	if err := area.Validate(); err != nil {
		return domain.Area{}, err
	}
	return s.store.CreateArea(ctx, area)
}

func (s *Service) ListAreas(ctx context.Context, projectKey string) ([]domain.Area, error) {
	project, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	return s.store.ListAreas(ctx, project.ID)
}

func (s *Service) GetArea(ctx context.Context, projectKey, slug string) (domain.Area, error) {
	project, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return domain.Area{}, err
	}
	slug, err = normalizeSlug(slug)
	if err != nil {
		return domain.Area{}, err
	}
	return s.store.GetArea(ctx, project.ID, slug)
}

func (s *Service) DeleteArea(ctx context.Context, projectKey, slug string) error {
	project, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return err
	}
	slug, err = normalizeSlug(slug)
	if err != nil {
		return err
	}
	return s.store.DeleteArea(ctx, project.ID, slug)
}

type TicketCreateInput struct {
	ProjectKey, AreaSlug, ParentKey, Title, Description string
	Type                                                domain.TicketType
	Priority                                            domain.Priority
}

func (s *Service) CreateTicket(ctx context.Context, input TicketCreateInput) (domain.Ticket, error) {
	project, err := s.GetProject(ctx, input.ProjectKey)
	if err != nil {
		return domain.Ticket{}, err
	}

	ticketType := normalizeTicketType(input.Type)
	if ticketType == "" {
		ticketType = domain.TicketTypeTask
	}
	priority := normalizePriority(input.Priority)
	if priority == "" {
		priority = domain.PriorityMedium
	}
	if err := validateTicketFields(domain.NormalizeName(input.Title), ticketType, priority); err != nil {
		return domain.Ticket{}, err
	}

	var areaID *int64
	if input.AreaSlug != "" {
		slug, err := normalizeSlug(input.AreaSlug)
		if err != nil {
			return domain.Ticket{}, err
		}
		area, err := s.store.GetArea(ctx, project.ID, slug)
		if err != nil {
			return domain.Ticket{}, err
		}
		areaID = &area.ID
	}
	var parentID *int64
	if strings.TrimSpace(input.ParentKey) != "" {
		parent, err := s.store.GetTicket(ctx, project.ID, normalizeTicketKey(input.ParentKey))
		if err != nil {
			return domain.Ticket{}, fmt.Errorf("parent ticket: %w", err)
		}
		parentID = &parent.ID
	}
	return s.store.CreateTicket(ctx, project, areaID, parentID, domain.NormalizeName(input.Title), input.Description, ticketType, priority)
}

type TicketListInput struct {
	ProjectKey, AreaSlug, ParentKey, Status, Type, Priority, Search string
	Flagged                                                         *bool
}

func (s *Service) ListTickets(ctx context.Context, input TicketListInput) ([]domain.Ticket, error) {
	project, err := s.GetProject(ctx, input.ProjectKey)
	if err != nil {
		return nil, err
	}
	filter := sqlite.TicketFilter{
		Status:   normalizeStatus(domain.Status(input.Status)),
		Type:     normalizeTicketType(domain.TicketType(input.Type)),
		Priority: normalizePriority(domain.Priority(input.Priority)),
		Search:   strings.TrimSpace(input.Search),
		Flagged:  input.Flagged,
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, fmt.Errorf("invalid ticket status %q", filter.Status)
	}
	if filter.Type != "" && !filter.Type.Valid() {
		return nil, fmt.Errorf("invalid ticket type %q", filter.Type)
	}
	if filter.Priority != "" && !filter.Priority.Valid() {
		return nil, fmt.Errorf("invalid ticket priority %q", filter.Priority)
	}
	if input.AreaSlug != "" {
		filter.AreaSlug, err = normalizeSlug(input.AreaSlug)
		if err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(input.ParentKey) != "" {
		filter.ParentKey = normalizeTicketKey(input.ParentKey)
	}
	return s.store.ListTickets(ctx, project.ID, filter)
}

func (s *Service) GetTicket(ctx context.Context, projectKey, key string) (domain.Ticket, error) {
	project, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return domain.Ticket{}, err
	}
	return s.store.GetTicket(ctx, project.ID, normalizeTicketKey(key))
}

func (s *Service) DeleteTicket(ctx context.Context, projectKey, key string) error {
	project, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return err
	}
	return s.store.DeleteTicket(ctx, project.ID, normalizeTicketKey(key))
}

type TicketUpdateInput struct {
	ProjectKey, Key    string
	Title, Description *string
	AreaSlug           *string
	Type               *domain.TicketType
	Status             *domain.Status
	Reason             *string
	Priority           *domain.Priority
	Flagged            *bool
	Revision           *int64
}

func (s *Service) UpdateTicket(ctx context.Context, input TicketUpdateInput) (domain.Ticket, error) {
	project, err := s.GetProject(ctx, input.ProjectKey)
	if err != nil {
		return domain.Ticket{}, err
	}
	if input.Title == nil && input.Description == nil && input.AreaSlug == nil && input.Type == nil && input.Status == nil && input.Reason == nil && input.Priority == nil && input.Flagged == nil {
		return domain.Ticket{}, errors.New("ticket update requires at least one mutable field")
	}
	if input.Revision != nil && *input.Revision < 1 {
		return domain.Ticket{}, errors.New("ticket revision must be positive")
	}
	if input.AreaSlug != nil && input.Revision == nil {
		return domain.Ticket{}, errors.New("ticket revision is required when moving an Area")
	}

	ticket, err := s.store.GetTicket(ctx, project.ID, normalizeTicketKey(input.Key))
	if err != nil {
		return domain.Ticket{}, err
	}
	if input.Title != nil {
		ticket.Title = domain.NormalizeName(*input.Title)
		if ticket.Title == "" {
			return domain.Ticket{}, errors.New("ticket title is required")
		}
	}
	if input.Description != nil {
		ticket.Description = *input.Description
	}
	if input.AreaSlug != nil {
		if strings.TrimSpace(*input.AreaSlug) == "" {
			ticket.AreaID = nil
		} else {
			slug, err := normalizeSlug(*input.AreaSlug)
			if err != nil {
				return domain.Ticket{}, err
			}
			area, err := s.store.GetArea(ctx, project.ID, slug)
			if err != nil {
				return domain.Ticket{}, err
			}
			ticket.AreaID = &area.ID
		}
	}
	if input.Type != nil {
		ticket.Type = normalizeTicketType(*input.Type)
		if !ticket.Type.Valid() {
			return domain.Ticket{}, fmt.Errorf("invalid ticket type %q", ticket.Type)
		}
	}
	if input.Status != nil {
		ticket.Status = normalizeStatus(*input.Status)
		if !ticket.Status.Valid() {
			return domain.Ticket{}, fmt.Errorf("invalid ticket status %q", ticket.Status)
		}
		if ticket.Status == domain.StatusCancelled {
			if input.Reason == nil || strings.TrimSpace(*input.Reason) == "" {
				return domain.Ticket{}, errors.New("cancellation reason is required when status is cancelled")
			}
			reason := strings.TrimSpace(*input.Reason)
			ticket.CancellationReason = &reason
		} else {
			if input.Reason != nil {
				return domain.Ticket{}, errors.New("cancellation reason is only valid when setting status to cancelled")
			}
			ticket.CancellationReason = nil
		}
	} else if input.Reason != nil {
		return domain.Ticket{}, errors.New("cancellation reason requires status cancelled")
	}
	if input.Priority != nil {
		ticket.Priority = normalizePriority(*input.Priority)
		if !ticket.Priority.Valid() {
			return domain.Ticket{}, fmt.Errorf("invalid ticket priority %q", ticket.Priority)
		}
	}
	if input.Flagged != nil {
		ticket.Flagged = *input.Flagged
	}
	return s.store.UpdateTicket(ctx, ticket, input.Revision)
}

func (s *Service) AddTicketNote(ctx context.Context, projectKey, key, body string, actor *string) (domain.TicketNote, error) {
	project, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return domain.TicketNote{}, err
	}
	note := domain.TicketNote{TicketID: 1, Kind: domain.NoteKindNote, Body: body, Actor: actor, CreatedAt: time.Now().UTC()}
	note.Normalize()
	if err := note.Validate(); err != nil {
		return domain.TicketNote{}, err
	}
	return s.store.AddTicketNote(ctx, project.ID, normalizeTicketKey(key), note)
}

func (s *Service) ListTicketNotes(ctx context.Context, projectKey, key string) ([]domain.TicketNote, error) {
	project, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	return s.store.ListTicketNotes(ctx, project.ID, normalizeTicketKey(key))
}

func normalizeProjectKey(key string) (string, error) {
	key = domain.NormalizeProjectKey(key)
	if err := domain.ValidateProjectKey(key); err != nil {
		return "", err
	}
	return key, nil
}

func normalizeSlug(slug string) (string, error) {
	return domain.Slugify(slug)
}

func normalizeTicketKey(key string) string {
	return strings.ToUpper(strings.TrimSpace(key))
}

func normalizeTicketType(ticketType domain.TicketType) domain.TicketType {
	return domain.TicketType(strings.ToLower(strings.TrimSpace(string(ticketType))))
}

func normalizeStatus(status domain.Status) domain.Status {
	return domain.Status(strings.ToLower(strings.TrimSpace(string(status))))
}

func normalizePriority(priority domain.Priority) domain.Priority {
	return domain.Priority(strings.ToLower(strings.TrimSpace(string(priority))))
}

func validateTicketFields(title string, ticketType domain.TicketType, priority domain.Priority) error {
	if title == "" {
		return errors.New("ticket title is required")
	}
	if !ticketType.Valid() {
		return fmt.Errorf("invalid ticket type %q", ticketType)
	}
	if !priority.Valid() {
		return fmt.Errorf("invalid ticket priority %q", priority)
	}
	return nil
}
