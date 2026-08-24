// Package domain contains the validated business entities used by Karya.
package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusInProgress Status = "in-progress"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
	StatusCancelled  Status = "cancelled"
)

func (s Status) Valid() bool {
	switch s {
	case StatusBacklog, StatusInProgress, StatusReview, StatusDone, StatusCancelled:
		return true
	default:
		return false
	}
}

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	default:
		return false
	}
}

type TicketType string

const (
	TicketTypeTask  TicketType = "task"
	TicketTypeBug   TicketType = "bug"
	TicketTypeSpike TicketType = "spike"
)

func (t TicketType) Valid() bool {
	switch t {
	case TicketTypeTask, TicketTypeBug, TicketTypeSpike:
		return true
	default:
		return false
	}
}

type Project struct {
	ID   int64  `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Normalize canonicalizes fields that users may enter with incidental spacing
// or lower-case letters.
func (p *Project) Normalize() {
	p.Key = NormalizeProjectKey(p.Key)
	p.Name = NormalizeName(p.Name)
}

func (p Project) Validate() error {
	if p.ID < 0 {
		return errors.New("project ID must not be negative")
	}
	if err := ValidateProjectKey(p.Key); err != nil {
		return err
	}
	if NormalizeName(p.Name) == "" {
		return errors.New("project name is required")
	}
	return nil
}

type Area struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
}

// Normalize canonicalizes an area's name and slug. A missing slug is derived
// from the name; a supplied slug is normalized as an ASCII slug.
func (a *Area) Normalize() error {
	a.Name = NormalizeName(a.Name)

	source := a.Slug
	if source == "" {
		source = a.Name
	}

	slug, err := Slugify(source)
	if err != nil {
		return err
	}
	a.Slug = slug
	return nil
}

func (a Area) Validate() error {
	if a.ID < 0 {
		return errors.New("area ID must not be negative")
	}
	if a.ProjectID <= 0 {
		return errors.New("area project ID must be positive")
	}
	if NormalizeName(a.Name) == "" {
		return errors.New("area name is required")
	}
	if a.Slug == "" {
		return errors.New("area slug is required")
	}
	canonical, err := Slugify(a.Slug)
	if err != nil || canonical != a.Slug {
		return errors.New("area slug must be a canonical ASCII slug")
	}
	return nil
}

type Ticket struct {
	ID                 int64      `json:"id"`
	ProjectID          int64      `json:"project_id"`
	AreaID             *int64     `json:"area_id"`
	ParentID           *int64     `json:"-"`
	ParentKey          *string    `json:"parent_key"`
	Number             int64      `json:"number"`
	Key                string     `json:"key"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Type               TicketType `json:"type"`
	Status             Status     `json:"status"`
	CancellationReason *string    `json:"cancellation_reason"`
	Priority           Priority   `json:"priority"`
	Flagged            bool       `json:"flagged"`
	Revision           int64      `json:"revision"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Normalize canonicalizes user-entered ticket fields. Description formatting
// is intentionally preserved because it is free-form content.
func (t *Ticket) Normalize() {
	t.Key = strings.ToUpper(strings.TrimSpace(t.Key))
	t.Title = NormalizeName(t.Title)
	t.Type = TicketType(strings.ToLower(strings.TrimSpace(string(t.Type))))
	t.Status = Status(strings.ToLower(strings.TrimSpace(string(t.Status))))
	t.Priority = Priority(strings.ToLower(strings.TrimSpace(string(t.Priority))))
	if t.ParentKey != nil {
		value := strings.ToUpper(strings.TrimSpace(*t.ParentKey))
		t.ParentKey = &value
	}
	if t.CancellationReason != nil {
		value := strings.TrimSpace(*t.CancellationReason)
		t.CancellationReason = &value
	}
}

func (t Ticket) Validate() error {
	if t.ID < 0 {
		return errors.New("ticket ID must not be negative")
	}
	if t.ProjectID <= 0 {
		return errors.New("ticket project ID must be positive")
	}
	if t.AreaID != nil && *t.AreaID <= 0 {
		return errors.New("ticket area ID must be positive")
	}
	if t.ParentID != nil && *t.ParentID <= 0 {
		return errors.New("ticket parent ID must be positive")
	}
	if t.ParentID != nil && t.ID > 0 && *t.ParentID == t.ID {
		return errors.New("ticket cannot be its own parent")
	}
	if t.Number <= 0 {
		return errors.New("ticket number must be positive")
	}
	keyNumber, err := ticketKeyNumber(t.Key)
	if err != nil {
		return err
	}
	if keyNumber != t.Number {
		return errors.New("ticket key number must match ticket number")
	}
	if NormalizeName(t.Title) == "" {
		return errors.New("ticket title is required")
	}
	if !t.Type.Valid() {
		return fmt.Errorf("invalid ticket type %q", t.Type)
	}
	if !t.Status.Valid() {
		return fmt.Errorf("invalid ticket status %q", t.Status)
	}
	if t.Status == StatusCancelled {
		if t.CancellationReason == nil || strings.TrimSpace(*t.CancellationReason) == "" {
			return errors.New("cancelled ticket requires a cancellation reason")
		}
	} else if t.CancellationReason != nil {
		return errors.New("cancellation reason is only valid for a cancelled ticket")
	}
	if !t.Priority.Valid() {
		return fmt.Errorf("invalid ticket priority %q", t.Priority)
	}
	if t.Revision < 1 {
		return errors.New("ticket revision must be positive")
	}
	if t.CreatedAt.IsZero() {
		return errors.New("ticket creation time is required")
	}
	if t.UpdatedAt.IsZero() {
		return errors.New("ticket update time is required")
	}
	if t.UpdatedAt.Before(t.CreatedAt) {
		return errors.New("ticket update time cannot precede creation time")
	}
	return nil
}

type NoteKind string

const (
	NoteKindNote         NoteKind = "note"
	NoteKindCancellation NoteKind = "cancellation"
)

func (k NoteKind) Valid() bool {
	return k == NoteKindNote || k == NoteKindCancellation
}

// TicketNote is an append-only observation or system event attached to a
// ticket. Karya assigns its ID and UTC creation time.
type TicketNote struct {
	ID        int64     `json:"id"`
	TicketID  int64     `json:"ticket_id"`
	Kind      NoteKind  `json:"kind"`
	Body      string    `json:"body"`
	Actor     *string   `json:"actor"`
	CreatedAt time.Time `json:"created_at"`
}

func (n *TicketNote) Normalize() {
	n.Kind = NoteKind(strings.ToLower(strings.TrimSpace(string(n.Kind))))
	if n.Kind == "" {
		n.Kind = NoteKindNote
	}
	if n.Actor != nil {
		actor := strings.TrimSpace(*n.Actor)
		if actor == "" {
			n.Actor = nil
		} else {
			n.Actor = &actor
		}
	}
}

func (n TicketNote) Validate() error {
	if n.ID < 0 {
		return errors.New("note ID must not be negative")
	}
	if n.TicketID <= 0 {
		return errors.New("note ticket ID must be positive")
	}
	if !n.Kind.Valid() {
		return fmt.Errorf("invalid note kind %q", n.Kind)
	}
	if strings.TrimSpace(n.Body) == "" {
		return errors.New("note body is required")
	}
	if n.Actor != nil && strings.TrimSpace(*n.Actor) == "" {
		return errors.New("note actor must not be blank")
	}
	if n.CreatedAt.IsZero() {
		return errors.New("note creation time is required")
	}
	return nil
}

// NormalizeProjectKey trims whitespace and converts a key to uppercase.
func NormalizeProjectKey(key string) string {
	return strings.ToUpper(strings.TrimSpace(key))
}

// ValidateProjectKey verifies the project key format used as the ticket-key
// prefix: an uppercase letter followed by at most eleven uppercase letters or
// digits.
func ValidateProjectKey(key string) error {
	if key == "" {
		return errors.New("project key is required")
	}
	if len(key) > 12 {
		return errors.New("project key must be at most 12 characters")
	}
	for i, r := range key {
		if (i == 0 && (r < 'A' || r > 'Z')) || (i > 0 && !isUpperAlphanumeric(r)) {
			return errors.New("project key must start with an uppercase letter and contain only uppercase letters and digits")
		}
	}
	return nil
}

func isUpperAlphanumeric(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func ticketKeyNumber(key string) (int64, error) {
	projectKey, number, found := strings.Cut(key, "-")
	if !found {
		return 0, errors.New("ticket key must use the format PROJECT-NUMBER")
	}
	if err := ValidateProjectKey(projectKey); err != nil {
		return 0, fmt.Errorf("invalid ticket project key: %w", err)
	}
	if number == "" {
		return 0, errors.New("ticket key number is required")
	}

	parsed, err := strconv.ParseInt(number, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != number {
		return 0, errors.New("ticket key number must be a positive decimal integer")
	}
	return parsed, nil
}

// NormalizeName trims leading/trailing whitespace and reduces internal runs of
// whitespace to one space.
func NormalizeName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

// Slugify converts an ASCII human-readable name to a lower-case, hyphenated
// slug. Non-ASCII characters and punctuation are separators, never output.
func Slugify(name string) (string, error) {
	var b strings.Builder
	lastHyphen := true

	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + ('a' - 'A'))
			lastHyphen = false
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}

	slug := strings.TrimSuffix(b.String(), "-")
	if slug == "" {
		return "", errors.New("slug must contain an ASCII letter or digit")
	}
	return slug, nil
}
