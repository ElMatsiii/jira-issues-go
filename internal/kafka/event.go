package kafka

import (
	"time"

	"github.com/google/uuid"
	"jira-issues-go/internal/domain"
)

type EventType string

const (
	EventTypeIssueCreated        EventType = "ISSUE_CREATED"
	EventTypeIssueUpdated        EventType = "ISSUE_UPDATED"
	EventTypeIssueStatusChanged  EventType = "ISSUE_STATUS_CHANGED"
	EventTypeIssueSprintAssigned EventType = "ISSUE_SPRINT_ASSIGNED"
	EventTypeIssueDeleted        EventType = "ISSUE_DELETED"
)

// IssueEventEnvelope conforms strictly to jira-contracts/schemas/issues.events.json
type IssueEventEnvelope struct {
	EventID     string            `json:"eventId"`
	EventType   EventType         `json:"eventType"`
	AggregateID string            `json:"aggregateId,omitempty"`
	Timestamp   string            `json:"timestamp"`
	Version     int               `json:"version,omitempty"`
	Payload     IssueEventPayload `json:"payload"`
}

// IssueEvent is an alias for IssueEventEnvelope
type IssueEvent = IssueEventEnvelope

// IssueEventPayload conforms strictly to payload in issues.events.json (additionalProperties: false)
type IssueEventPayload struct {
	ID                 string  `json:"id"`
	Key                string  `json:"key"`
	ProjectID          string  `json:"projectId"`
	SprintID           *string `json:"sprintId,omitempty"`
	Title              string  `json:"title"`
	Description        string  `json:"description,omitempty"`
	Status             string  `json:"status"`
	Priority           string  `json:"priority"`
	Type               string  `json:"type"`
	StoryPoints        int32   `json:"storyPoints"`
	AssigneeID         *string `json:"assigneeId,omitempty"`
	ReporterID         *string `json:"reporterId,omitempty"`
	AcceptanceCriteria string  `json:"acceptanceCriteria,omitempty"`
	CreatedAt          string  `json:"createdAt,omitempty"`
	UpdatedAt          string  `json:"updatedAt,omitempty"`
	PreviousStatus     *string `json:"previousStatus,omitempty"`
	PreviousSprintID   *string `json:"previousSprintId,omitempty"`
}

// NewIssueEnvelope creates an envelope for an issue event.
func NewIssueEnvelope(eventType EventType, issue *domain.Issue, prevStatus *domain.Status, prevSprintID *string) *IssueEventEnvelope {
	var prevStatusStr *string
	if prevStatus != nil && *prevStatus != domain.StatusUnspecified {
		s := prevStatus.String()
		prevStatusStr = &s
	}

	version := issue.Version
	if version <= 0 {
		version = 1
	}

	createdStr := issue.CreatedAt.UTC().Format(time.RFC3339Nano)
	updatedStr := issue.UpdatedAt.UTC().Format(time.RFC3339Nano)

	return &IssueEventEnvelope{
		EventID:     uuid.New().String(),
		EventType:   eventType,
		AggregateID: issue.ID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		Version:     version,
		Payload: IssueEventPayload{
			ID:                 issue.ID,
			Key:                issue.Key,
			ProjectID:          issue.ProjectID,
			SprintID:           issue.SprintID,
			Title:              issue.Title,
			Description:        issue.Description,
			Status:             issue.Status.String(),
			Priority:           issue.Priority.String(),
			Type:               issue.Type.String(),
			StoryPoints:        issue.StoryPoints,
			AssigneeID:         issue.AssigneeID,
			ReporterID:         issue.ReporterID,
			AcceptanceCriteria: issue.AcceptanceCriteria,
			CreatedAt:          createdStr,
			UpdatedAt:          updatedStr,
			PreviousStatus:     prevStatusStr,
			PreviousSprintID:   prevSprintID,
		},
	}
}

// Helper constructors for explorer_m2_3 test compatibility
func NewIssueCreatedEvent(issue *domain.Issue) *IssueEventEnvelope {
	return NewIssueEnvelope(EventTypeIssueCreated, issue, nil, nil)
}

func NewIssueUpdatedEvent(issue *domain.Issue) *IssueEventEnvelope {
	return NewIssueEnvelope(EventTypeIssueUpdated, issue, nil, nil)
}

func NewIssueStatusChangedEvent(issue *domain.Issue, prevStatusStr string) *IssueEventEnvelope {
	ps := domain.ParseStatus(prevStatusStr)
	return NewIssueEnvelope(EventTypeIssueStatusChanged, issue, &ps, nil)
}

func NewIssueSprintAssignedEvent(issue *domain.Issue, prevSprintID *string) *IssueEventEnvelope {
	return NewIssueEnvelope(EventTypeIssueSprintAssigned, issue, nil, prevSprintID)
}

func NewIssueDeletedEvent(issue *domain.Issue) *IssueEventEnvelope {
	return NewIssueEnvelope(EventTypeIssueDeleted, issue, nil, nil)
}
