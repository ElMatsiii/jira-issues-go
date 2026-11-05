package service

import (
	"context"
	"fmt"
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
		projectID = "00000000-0000-0000-0000-000000000001" // Default project UUID
	}

	// Determine next sequence
	var issueKey string
	seq, prefix, err := s.repo.GetNextSequence(ctx, projectID)
	if err == nil && prefix != "" {
		issueKey = fmt.Sprintf("%s-%d", prefix, seq)
	} else if dto.ProjectKey != "" {
		issueKey = fmt.Sprintf("%s-%d", dto.ProjectKey, seq)
	} else {
		issueKey = fmt.Sprintf("PROJ-%d", seq)
	}

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
		_ = s.producer.PublishIssueCreated(ctx, issue)
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

func (s *issueService) UpdateIssueStatus(ctx context.Context, id string, newStatus domain.Status) (*domain.Issue, error) {
	issue, err := s.repo.GetByIDOrKey(ctx, id)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, domain.ErrNotFound
	}

	prevStatus := issue.Status
	if err := issue.TransitionTo(newStatus); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, issue); err != nil {
		return nil, err
	}

	if s.producer != nil {
		_ = s.producer.PublishIssueStatusChanged(ctx, issue, prevStatus)
	}

	return issue, nil
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
			_ = s.producer.PublishIssueSprintAssigned(ctx, issue, nil)
		}
	}

	return updated, count, nil
}
