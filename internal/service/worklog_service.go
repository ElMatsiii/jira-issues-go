package service

import (
	"context"
	"errors"
	"fmt"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
)

// ProjectAccessChecker answers whether a user can open a project (IdentityService implements it).
type ProjectAccessChecker interface {
	CheckProjectAccess(ctx context.Context, userID, projectID string) (ProjectAccess, error)
}

// WorkLogService logs and reads hours per issue. Logging and listing require access to the project.
type WorkLogService interface {
	LogWork(ctx context.Context, userID, projectID, issueID string, hours float64, note string) (*domain.WorkLog, error)
	// ListWorkLogs returns the newest entries (limit <= 0 or above the maximum means 100) and the
	// total hours of the whole project.
	ListWorkLogs(ctx context.Context, requesterID, projectID string, limit int) ([]*domain.WorkLog, float64, error)
	SumSprintHours(ctx context.Context, projectID, sprintID string) (float64, error)
}

type workLogService struct {
	logs   repository.WorkLogRepository
	issues repository.IssueRepository
	access ProjectAccessChecker
}

// NewWorkLogService constructs the WorkLogService.
func NewWorkLogService(logs repository.WorkLogRepository, issues repository.IssueRepository, access ProjectAccessChecker) WorkLogService {
	return &workLogService{logs: logs, issues: issues, access: access}
}

func (s *workLogService) requireAccess(ctx context.Context, userID, projectID string) error {
	access, err := s.access.CheckProjectAccess(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if !access.Allowed {
		return domain.ErrProjectForbidden
	}
	return nil
}

func (s *workLogService) LogWork(ctx context.Context, userID, projectID, issueID string, hours float64, note string) (*domain.WorkLog, error) {
	log, err := domain.NewWorkLog(userID, projectID, issueID, hours, note)
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, userID, projectID); err != nil {
		return nil, err
	}

	issue, err := s.issues.GetByID(ctx, issueID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && (issue == nil || issue.ProjectID != projectID)) {
		return nil, domain.ErrIssueNotInProject
	}
	if err != nil {
		return nil, fmt.Errorf("load issue %s: %w", issueID, err)
	}
	// The entry remembers the sprint the issue is in right now, so moving the issue later does not
	// move hours between sprints.
	if issue.SprintID != nil {
		log.SprintID = *issue.SprintID
	}

	if err := s.logs.Create(ctx, log); err != nil {
		return nil, err
	}
	return log, nil
}

func (s *workLogService) ListWorkLogs(ctx context.Context, requesterID, projectID string, limit int) ([]*domain.WorkLog, float64, error) {
	if err := s.requireAccess(ctx, requesterID, projectID); err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > domain.DefaultWorkLogLimit {
		limit = domain.DefaultWorkLogLimit
	}
	logs, err := s.logs.ListByProject(ctx, projectID, limit)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.logs.SumProjectHours(ctx, projectID)
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (s *workLogService) SumSprintHours(ctx context.Context, projectID, sprintID string) (float64, error) {
	return s.logs.SumSprintHours(ctx, projectID, sprintID)
}
