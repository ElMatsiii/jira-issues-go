package repository

import (
	"context"
	"jira-issues-go/internal/domain"
)

// ListFilter defines filtering and pagination parameters for listing issues.
type ListFilter struct {
	ProjectID string
	SprintID  *string
	Status    domain.Status
	Limit     int32
	Offset    int32
}

// IssueFilter alias for backwards compatibility.
type IssueFilter = ListFilter

// IssueRepository defines data access methods for Issue aggregates.
type IssueRepository interface {
	Create(ctx context.Context, issue *domain.Issue) error
	GetByID(ctx context.Context, id string) (*domain.Issue, error)
	GetByKey(ctx context.Context, key string) (*domain.Issue, error)
	GetByIDOrKey(ctx context.Context, idOrKey string) (*domain.Issue, error)
	List(ctx context.Context, filter ListFilter) ([]*domain.Issue, int32, error)
	Update(ctx context.Context, issue *domain.Issue) error
	BatchUpdateSprint(ctx context.Context, ids []string, sprintID string) ([]*domain.Issue, error)
	GetNextSequence(ctx context.Context, projectID string) (int64, string, error)
}

// ProjectRepository defines data access methods for Project entities.
type ProjectRepository interface {
	Create(ctx context.Context, project *domain.Project) error
	GetByID(ctx context.Context, id string) (*domain.Project, error)
	GetByKey(ctx context.Context, key string) (*domain.Project, error)
}
