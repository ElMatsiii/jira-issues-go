package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/kafka"
	"jira-issues-go/internal/repository"
)

type issueService struct {
	repo     repository.IssueRepository
	producer kafka.EventProducer
}

// logPublishError records a failed domain-event publication. The database change is already
// committed, so the RPC still succeeds, but the failure must be visible: MS3 analytics depend on
// these events and a silent loss leaves the burndown without data.
func logPublishError(eventType string, issue *domain.Issue, err error) {
	if err == nil {
		return
	}
	log.Printf("[ERROR] Kafka publish failed: event=%s issueId=%s key=%s: %v", eventType, issue.ID, issue.Key, err)
}

// NewIssueService constructs a new business logic service for issues.
func NewIssueService(repo repository.IssueRepository, producer kafka.EventProducer) IssueService {
	return &issueService{
		repo:     repo,
		producer: producer,
	}
}

func (s *issueService) CreateIssue(ctx context.Context, dto CreateIssueDTO) (*domain.Issue, error) {
	title := strings.TrimSpace(dto.Title)
	if title == "" {
		return nil, domain.ErrEmptyTitle
	}

	projectID := strings.TrimSpace(dto.ProjectID)
	if projectID == "" {
		return nil, domain.ErrEmptyProjectID
	}

	// Reserving the next number also proves the project exists. A failure used to be swallowed,
	// producing keys like "PROJ-0" and an opaque foreign-key error on insert.
	seq, prefix, err := s.repo.GetNextSequence(ctx, projectID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s", domain.ErrProjectNotFound, projectID)
	}
	if err != nil {
		return nil, fmt.Errorf("reserve issue key for project %s: %w", projectID, err)
	}
	if prefix == "" {
		prefix = strings.TrimSpace(dto.ProjectKey)
	}
	if prefix == "" {
		return nil, fmt.Errorf("%w: project %s has no key", domain.ErrProjectNotFound, projectID)
	}
	issueKey := fmt.Sprintf("%s-%d", prefix, seq)

	priority := dto.Priority
	if priority == domain.PriorityUnspecified {
		priority = domain.PriorityMedium
	}

	issueType := dto.Type
	if issueType == domain.TypeUnspecified {
		issueType = domain.TypeStory
	}

	now := time.Now().UTC()
	issue := &domain.Issue{
		ID:                 uuid.New().String(),
		Key:                issueKey,
		ProjectID:          projectID,
		Title:              title,
		Description:        dto.Description,
		Status:             domain.StatusTodo,
		Priority:           priority,
		Type:               issueType,
		StoryPoints:        dto.StoryPoints,
		SprintID:           dto.SprintID,
		AssigneeID:         dto.AssigneeID,
		ReporterID:         dto.ReporterID,
		AcceptanceCriteria: dto.AcceptanceCriteria,
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := issue.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, issue); err != nil {
		return nil, err
	}

	if s.producer != nil {
		logPublishError("ISSUE_CREATED", issue, s.producer.PublishIssueCreated(ctx, issue))
	}

	return issue, nil
}

func (s *issueService) GetIssue(ctx context.Context, idOrKey string) (*domain.Issue, error) {
	trimmed := strings.TrimSpace(idOrKey)
	if trimmed == "" {
		return nil, domain.ErrNotFound
	}
	return s.repo.GetByIDOrKey(ctx, trimmed)
}

func (s *issueService) ListIssues(ctx context.Context, filter repository.ListFilter) ([]*domain.Issue, int32, error) {
	return s.repo.List(ctx, filter)
}

// maxStatusUpdateAttempts bounds the optimistic-concurrency retries of UpdateIssueStatus.
const maxStatusUpdateAttempts = 3

// UpdateIssueStatus writes only status/version (never the whole row), guarded by the version that
// was read. If another write happened in between (e.g. a sprint assignment, which also bumps the
// version), it re-reads the issue, re-validates the transition and retries.
func (s *issueService) UpdateIssueStatus(ctx context.Context, id string, newStatus domain.Status) (*domain.Issue, error) {
	for attempt := 1; ; attempt++ {
		current, err := s.repo.GetByIDOrKey(ctx, id)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, domain.ErrNotFound
		}

		// Same status: nothing changes, so nothing is written and no ISSUE_STATUS_CHANGED is emitted
		// (it used to publish an event whose previousStatus equalled the new status).
		if current.Status == newStatus {
			return current, nil
		}

		// Work on a copy so the version read stays intact for the guarded update.
		next := *current
		prevStatus := current.Status
		if err := next.TransitionTo(newStatus); err != nil {
			return nil, err
		}

		err = s.repo.UpdateStatus(ctx, &next, current.Version)
		if errors.Is(err, domain.ErrVersionConflict) && attempt < maxStatusUpdateAttempts {
			continue
		}
		if err != nil {
			return nil, err
		}

		if s.producer != nil {
			logPublishError("ISSUE_STATUS_CHANGED", &next, s.producer.PublishIssueStatusChanged(ctx, &next, prevStatus))
		}
		return &next, nil
	}
}

func (s *issueService) BatchUpdateIssueSprint(ctx context.Context, ids []string, sprintID string) ([]*domain.Issue, int32, error) {
	if len(ids) == 0 {
		return []*domain.Issue{}, 0, nil
	}

	updated, err := s.repo.BatchUpdateSprint(ctx, ids, sprintID)
	if err != nil {
		return nil, 0, err
	}

	count := int32(len(updated))
	if s.producer != nil {
		for _, issue := range updated {
			logPublishError("ISSUE_SPRINT_ASSIGNED", issue, s.producer.PublishIssueSprintAssigned(ctx, issue, nil))
		}
	}

	return updated, count, nil
}
