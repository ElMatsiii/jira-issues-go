package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Issue is the core domain aggregate representing an Agile work item.
type Issue struct {
	ID                 string    `json:"id" db:"id"`
	Key                string    `json:"key" db:"key"`
	ProjectID          string    `json:"projectId" db:"project_id"`
	Title              string    `json:"title" db:"title"`
	Description        string    `json:"description" db:"description"`
	Status             Status    `json:"status" db:"status"`
	Priority           Priority  `json:"priority" db:"priority"`
	Type               Type      `json:"type" db:"type"`
	StoryPoints        int32     `json:"storyPoints" db:"story_points"`
	SprintID           *string   `json:"sprintId,omitempty" db:"sprint_id"`
	AssigneeID         *string   `json:"assigneeId,omitempty" db:"assignee_id"`
	ReporterID         *string   `json:"reporterId,omitempty" db:"reporter_id"`
	AcceptanceCriteria string    `json:"acceptanceCriteria" db:"acceptance_criteria"`
	Version            int       `json:"version" db:"version"`
	CreatedAt          time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt          time.Time `json:"updatedAt" db:"updated_at"`
}

// NewIssue constructs and validates a new Issue aggregate.
func NewIssue(
	projectID, key, title, description string,
	priority Priority,
	issueType Type,
	storyPoints int32,
	acceptanceCriteria string,
) (*Issue, error) {
	now := time.Now().UTC()
	issue := &Issue{
		ID:                 uuid.New().String(),
		Key:                strings.TrimSpace(key),
		ProjectID:          strings.TrimSpace(projectID),
		Title:              strings.TrimSpace(title),
		Description:        description,
		Status:             StatusTodo,
		Priority:           priority,
		Type:               issueType,
		StoryPoints:        storyPoints,
		AcceptanceCriteria: acceptanceCriteria,
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := issue.Validate(); err != nil {
		return nil, err
	}

	return issue, nil
}

// Validate checks business rules on an issue prior to persistence.
func (i *Issue) Validate() error {
	trimmedTitle := strings.TrimSpace(i.Title)
	if trimmedTitle == "" {
		return ErrEmptyTitle
	}
	if len(i.Title) > 255 {
		return ErrTitleTooLong
	}
	if strings.TrimSpace(i.ProjectID) == "" {
		return ErrEmptyProjectID
	}
	if i.StoryPoints < 0 {
		return ErrNegativeStoryPoints
	}
	if i.Status == StatusUnspecified {
		i.Status = StatusTodo
	} else if !i.Status.IsValid() {
		return ErrInvalidStatus
	}

	if i.Priority == PriorityUnspecified {
		i.Priority = PriorityMedium
	} else if !i.Priority.IsValid() {
		return ErrInvalidPriority
	}
	if i.Type == TypeUnspecified {
		i.Type = TypeStory
	} else if !i.Type.IsValid() {
		return ErrInvalidType
	}

	return nil
}

// TransitionTo validates and performs a state machine transition.
func (i *Issue) TransitionTo(newStatus Status) error {
	if err := ValidateStatusTransition(i.Status, newStatus); err != nil {
		return err
	}

	i.Status = newStatus
	i.UpdatedAt = time.Now().UTC()
	i.Version++
	return nil
}
