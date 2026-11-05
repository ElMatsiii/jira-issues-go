package service

import (
	"context"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
)

// CreateIssueDTO holds data transfer parameters for creating an issue.
type CreateIssueDTO struct {
	ProjectID          string
	ProjectKey         string
	Title              string
	Description        string
	Priority           domain.Priority
	Type               domain.Type
	StoryPoints        int32
	SprintID           *string
	AssigneeID         *string
	ReporterID         *string
	AcceptanceCriteria string
}

// IssueService defines the business operations for Issue management.
type IssueService interface {
	CreateIssue(ctx context.Context, dto CreateIssueDTO) (*domain.Issue, error)
	GetIssue(ctx context.Context, idOrKey string) (*domain.Issue, error)
	ListIssues(ctx context.Context, filter repository.ListFilter) ([]*domain.Issue, int32, error)
	UpdateIssueStatus(ctx context.Context, id string, newStatus domain.Status) (*domain.Issue, error)
	BatchUpdateIssueSprint(ctx context.Context, ids []string, sprintID string) ([]*domain.Issue, int32, error)
}
