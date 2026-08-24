package domain

import (
	"testing"
	"time"
)

func TestEnumValidity(t *testing.T) {
	for _, status := range []Status{StatusBacklog, StatusInProgress, StatusReview, StatusDone, StatusCancelled} {
		if !status.Valid() {
			t.Errorf("status %q is not valid", status)
		}
	}
	if Status("blocked").Valid() {
		t.Error("unexpected valid status")
	}

	for _, priority := range []Priority{PriorityLow, PriorityMedium, PriorityHigh} {
		if !priority.Valid() {
			t.Errorf("priority %q is not valid", priority)
		}
	}
	if Priority("urgent").Valid() {
		t.Error("unexpected valid priority")
	}

	for _, ticketType := range []TicketType{TicketTypeTask, TicketTypeBug, TicketTypeSpike} {
		if !ticketType.Valid() {
			t.Errorf("ticket type %q is not valid", ticketType)
		}
	}
	if TicketType("feature").Valid() || TicketType("scope").Valid() {
		t.Error("unexpected valid ticket type")
	}
}

func TestProjectNormalizeAndValidate(t *testing.T) {
	project := Project{Key: " app42 ", Name: "  My\tProject  "}
	project.Normalize()

	if project.Key != "APP42" {
		t.Errorf("Key = %q, want APP42", project.Key)
	}
	if project.Name != "My Project" {
		t.Errorf("Name = %q, want My Project", project.Name)
	}
	if err := project.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateProjectKey(t *testing.T) {
	valid := []string{"A", "APP", "A1", "ABCDEFGHIJKL"}
	for _, key := range valid {
		if err := ValidateProjectKey(key); err != nil {
			t.Errorf("ValidateProjectKey(%q) error = %v", key, err)
		}
	}

	invalid := []string{"", "1APP", "app", "APP-1", "APP_1", "ABCDEFGHIJKLM", "A\u00c4"}
	for _, key := range invalid {
		if err := ValidateProjectKey(key); err == nil {
			t.Errorf("ValidateProjectKey(%q) succeeded", key)
		}
	}
}

func TestProjectValidationFailures(t *testing.T) {
	tests := []Project{
		{ID: -1, Key: "APP", Name: "App"},
		{Key: "APP", Name: ""},
		{Key: "APP", Name: "  "},
	}
	for _, project := range tests {
		if err := project.Validate(); err == nil {
			t.Errorf("Validate(%+v) succeeded", project)
		}
	}
}

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"User Auth":             "user-auth",
		"  API__v2 / Accounts ": "api-v2-accounts",
		"A---B":                 "a-b",
		"Caf\u00e9 & Planning":  "caf-planning",
		"Project 2026":          "project-2026",
	}
	for input, want := range tests {
		got, err := Slugify(input)
		if err != nil {
			t.Errorf("Slugify(%q) error = %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("Slugify(%q) = %q, want %q", input, got, want)
		}
	}

	for _, input := range []string{"", "   ", "---", "\u3053\u3093\u306b\u3061\u306f"} {
		if _, err := Slugify(input); err == nil {
			t.Errorf("Slugify(%q) succeeded", input)
		}
	}
}

func TestAreaNormalizeAndValidate(t *testing.T) {
	area := Area{ProjectID: 1, Name: "  User   Auth  "}
	if err := area.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if area.Name != "User Auth" || area.Slug != "user-auth" {
		t.Errorf("area = %+v, want normalized name and slug", area)
	}
	if err := area.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestAreaNormalizationAndValidationFailures(t *testing.T) {
	area := Area{ProjectID: 1, Name: "Area", Slug: "Custom / Area"}
	if err := area.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if area.Slug != "custom-area" {
		t.Errorf("Slug = %q, want custom-area", area.Slug)
	}

	if err := (&Area{Name: "\u3053\u3093\u306b\u3061\u306f"}).Normalize(); err == nil {
		t.Error("Normalize() succeeded for a blank resulting slug")
	}

	invalid := []Area{
		{ID: -1, ProjectID: 1, Name: "Area", Slug: "area"},
		{ProjectID: 0, Name: "Area", Slug: "area"},
		{ProjectID: 1, Name: "", Slug: "area"},
		{ProjectID: 1, Name: "Area", Slug: "Area"},
		{ProjectID: 1, Name: "Area", Slug: ""},
	}
	for _, area := range invalid {
		if err := area.Validate(); err == nil {
			t.Errorf("Validate(%+v) succeeded", area)
		}
	}
}

func TestTicketNormalizeAndValidate(t *testing.T) {
	areaID := int64(2)
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	ticket := Ticket{
		ProjectID: 1,
		AreaID:    &areaID,
		Number:    42,
		Key:       " app-42 ",
		Title:     "  Add\tSQLite  ",
		Type:      " BUG ",
		Status:    " IN-PROGRESS ",
		Priority:  " HIGH ",
		Revision:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	ticket.Normalize()

	if ticket.Key != "APP-42" || ticket.Title != "Add SQLite" {
		t.Errorf("ticket strings were not normalized: %+v", ticket)
	}
	if ticket.Type != TicketTypeBug || ticket.Status != StatusInProgress || ticket.Priority != PriorityHigh {
		t.Errorf("ticket enums were not normalized: %+v", ticket)
	}
	if err := ticket.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestTicketValidationFailures(t *testing.T) {
	now := time.Now()
	valid := Ticket{
		ProjectID: 1,
		Number:    1,
		Key:       "APP-1",
		Title:     "Title",
		Type:      TicketTypeTask,
		Status:    StatusBacklog,
		Priority:  PriorityMedium,
		Revision:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	tests := []struct {
		name   string
		mutate func(*Ticket)
	}{
		{"negative ID", func(t *Ticket) { t.ID = -1 }},
		{"missing project", func(t *Ticket) { t.ProjectID = 0 }},
		{"invalid area", func(t *Ticket) { id := int64(0); t.AreaID = &id }},
		{"missing number", func(t *Ticket) { t.Number = 0 }},
		{"missing key", func(t *Ticket) { t.Key = "" }},
		{"malformed key", func(t *Ticket) { t.Key = "APP-001" }},
		{"mismatched key number", func(t *Ticket) { t.Key = "APP-2" }},
		{"missing title", func(t *Ticket) { t.Title = "" }},
		{"invalid type", func(t *Ticket) { t.Type = "feature" }},
		{"invalid status", func(t *Ticket) { t.Status = "blocked" }},
		{"invalid priority", func(t *Ticket) { t.Priority = "urgent" }},
		{"missing revision", func(t *Ticket) { t.Revision = 0 }},
		{"missing creation time", func(t *Ticket) { t.CreatedAt = time.Time{} }},
		{"missing update time", func(t *Ticket) { t.UpdatedAt = time.Time{} }},
		{"reversed times", func(t *Ticket) { t.UpdatedAt = t.CreatedAt.Add(-time.Second) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ticket := valid
			test.mutate(&ticket)
			if err := ticket.Validate(); err == nil {
				t.Errorf("Validate() succeeded for %s", test.name)
			}
		})
	}
}

func TestCancelledTicketAndNoteValidation(t *testing.T) {
	now := time.Now().UTC()
	reason := " No longer needed "
	ticket := Ticket{ProjectID: 1, Number: 1, Key: "APP-1", Title: "Retire work", Type: TicketTypeTask, Status: StatusCancelled, CancellationReason: &reason, Priority: PriorityLow, Revision: 1, CreatedAt: now, UpdatedAt: now}
	ticket.Normalize()
	if ticket.CancellationReason == nil || *ticket.CancellationReason != "No longer needed" {
		t.Fatalf("normalized cancellation reason = %v", ticket.CancellationReason)
	}
	if err := ticket.Validate(); err != nil {
		t.Fatal(err)
	}

	ticket.CancellationReason = nil
	if err := ticket.Validate(); err == nil {
		t.Fatal("cancelled ticket without reason was valid")
	}
	reason = "unexpected"
	ticket.Status = StatusBacklog
	ticket.CancellationReason = &reason
	if err := ticket.Validate(); err == nil {
		t.Fatal("active ticket with cancellation reason was valid")
	}

	actor := " agent-a "
	note := TicketNote{TicketID: 1, Body: "Finding", Actor: &actor, CreatedAt: now}
	note.Normalize()
	if note.Kind != NoteKindNote || note.Actor == nil || *note.Actor != "agent-a" {
		t.Fatalf("normalized note = %+v", note)
	}
	if err := note.Validate(); err != nil {
		t.Fatal(err)
	}
	note.Body = "  "
	if err := note.Validate(); err == nil {
		t.Fatal("blank note was valid")
	}
}
