package model

import "time"

type Status string
type Priority string
type TicketType string

const (
	StatusBacklog    Status = "backlog"
	StatusInProgress Status = "in-progress"
	StatusReview     Status = "review"
	StatusDone       Status = "done"

	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"

	TypeTask  TicketType = "task"
	TypeBug   TicketType = "bug"
	TypeSpike TicketType = "spike"
)

type Ticket struct {
	ID       string     `yaml:"id"       json:"id"`
	Title    string     `yaml:"title"    json:"title"`
	Type     TicketType `yaml:"type"     json:"type"`
	Status   Status     `yaml:"status"   json:"status"`
	Priority Priority   `yaml:"priority" json:"priority"`
	Epic     string     `yaml:"epic"     json:"epic"`
	Flagged  bool       `yaml:"flagged"  json:"flagged"`

	Body     string    `json:"body"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
	Dir      string    `json:"-"`
}

type Epic struct {
	Name    string
	Project string
	Dir     string
}

type Project struct {
	Name   string `toml:"name"`
	Prefix string `toml:"prefix"`
	Dir    string
}
