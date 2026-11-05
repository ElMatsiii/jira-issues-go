package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Project represents a container for issues.
type Project struct {
	ID           string    `json:"id" db:"id"`
	Key          string    `json:"key" db:"key"` // e.g. "PROJ"
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	IssueCounter int       `json:"issueCounter" db:"issue_counter"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt    time.Time `json:"updatedAt" db:"updated_at"`
}

// NewProject constructs a validated Project entity.
func NewProject(key, name, description string) (*Project, error) {
	k := strings.TrimSpace(key)
	if k == "" {
		return nil, ErrEmptyTitle
	}
	n := strings.TrimSpace(name)
	if n == "" {
		return nil, ErrEmptyTitle
	}

	now := time.Now().UTC()
	return &Project{
		ID:           uuid.New().String(),
		Key:          strings.ToUpper(k),
		Name:         n,
		Description:  description,
		IssueCounter: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}
