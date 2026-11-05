package grpc

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
	"jira-issues-go/internal/service"
)

// CreateIssue handles creation of a new work item.
func (s *Server) CreateIssue(ctx context.Context, req *issuesv1.CreateIssueRequest) (*issuesv1.CreateIssueResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	title := strings.TrimSpace(req.GetTitle())
	if title == "" {
		return nil, status.Error(codes.InvalidArgument, "title cannot be empty")
	}

	projectID := strings.TrimSpace(req.GetProjectId())
	if projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id is required")
	}

	if req.GetStoryPoints() < 0 {
		return nil, status.Error(codes.InvalidArgument, "story points cannot be negative")
	}

	var sprintID, assigneeID, reporterID *string
	if req.GetSprintId() != "" {
		sid := req.GetSprintId()
		sprintID = &sid
	}
	if req.GetAssigneeId() != "" {
		aid := req.GetAssigneeId()
		assigneeID = &aid
	}
	if req.GetReporterId() != "" {
		rid := req.GetReporterId()
		reporterID = &rid
	}

	dto := service.CreateIssueDTO{
		ProjectID:          projectID,
		Title:              title,
		Description:        req.GetDescription(),
		Priority:           domain.PriorityFromProto(req.GetPriority()),
		Type:               domain.TypeFromProto(req.GetType()),
		StoryPoints:        req.GetStoryPoints(),
		SprintID:           sprintID,
		AssigneeID:         assigneeID,
		ReporterID:         reporterID,
		AcceptanceCriteria: req.GetAcceptanceCriteria(),
	}

	issue, err := s.svc.CreateIssue(ctx, dto)
	if err != nil {
		if errors.Is(err, domain.ErrEmptyTitle) ||
			errors.Is(err, domain.ErrEmptyProjectID) ||
			errors.Is(err, domain.ErrNegativeStoryPoints) ||
			errors.Is(err, domain.ErrTitleTooLong) ||
			errors.Is(err, domain.ErrInvalidStatus) ||
			errors.Is(err, domain.ErrInvalidPriority) ||
			errors.Is(err, domain.ErrInvalidType) {
			return nil, status.Errorf(codes.InvalidArgument, "validation error: %v", err)
		}
		if errors.Is(err, domain.ErrProjectNotFound) {
			return nil, status.Errorf(codes.NotFound, "project not found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to create issue: %v", err)
	}
	if issue == nil {
		return nil, status.Errorf(codes.Internal, "failed to create issue: nil issue returned")
	}

	return &issuesv1.CreateIssueResponse{
		Issue: domain.IssueToProto(issue),
	}, nil
}

// GetIssue retrieves an issue by its UUID ID or human-readable Key.
func (s *Server) GetIssue(ctx context.Context, req *issuesv1.GetIssueRequest) (*issuesv1.GetIssueResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	idOrKey := strings.TrimSpace(req.GetId())
	if idOrKey == "" {
		return nil, status.Error(codes.InvalidArgument, "issue id or key is required")
	}

	issue, err := s.svc.GetIssue(ctx, idOrKey)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "issue %q not found", idOrKey)
		}
		return nil, status.Errorf(codes.Internal, "failed to retrieve issue: %v", err)
	}
	if issue == nil {
		return nil, status.Errorf(codes.NotFound, "issue %q not found", idOrKey)
	}

	return &issuesv1.GetIssueResponse{
		Issue: domain.IssueToProto(issue),
	}, nil
}

// ListIssues returns a paginated list of issues matching the filter criteria.
func (s *Server) ListIssues(ctx context.Context, req *issuesv1.ListIssuesRequest) (*issuesv1.ListIssuesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	limit := req.GetLimit()
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}

	offset := req.GetOffset()
	if offset < 0 {
		offset = 0
	}

	var sprintID *string
	if strings.TrimSpace(req.GetSprintId()) != "" {
		sid := strings.TrimSpace(req.GetSprintId())
		sprintID = &sid
	}

	filter := repository.ListFilter{
		ProjectID: strings.TrimSpace(req.GetProjectId()),
		SprintID:  sprintID,
		Status:    domain.StatusFromProto(req.GetStatus()),
		Limit:     limit,
		Offset:    offset,
	}

	issues, total, err := s.svc.ListIssues(ctx, filter)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "issues not found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to list issues: %v", err)
	}

	protoIssues := make([]*issuesv1.Issue, len(issues))
	for i, iss := range issues {
		protoIssues[i] = domain.IssueToProto(iss)
	}

	return &issuesv1.ListIssuesResponse{
		Issues:     protoIssues,
		TotalCount: total,
	}, nil
}

// UpdateIssueStatus transitions an issue's status and publishes ISSUE_STATUS_CHANGED.
func (s *Server) UpdateIssueStatus(ctx context.Context, req *issuesv1.UpdateIssueStatusRequest) (*issuesv1.UpdateIssueStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "issue id is required")
	}

	if req.GetStatus() == issuesv1.IssueStatus_ISSUE_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "status must be specified")
	}

	newStatus := domain.StatusFromProto(req.GetStatus())
	if newStatus == domain.StatusUnspecified {
		return nil, status.Error(codes.InvalidArgument, "invalid status value")
	}

	updated, err := s.svc.UpdateIssueStatus(ctx, id, newStatus)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "issue %q not found", id)
		}
		if errors.Is(err, domain.ErrInvalidStatusTransition) {
			return nil, status.Errorf(codes.FailedPrecondition, "invalid status transition: %v", err)
		}
		if errors.Is(err, domain.ErrInvalidStatus) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid status: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to update issue status: %v", err)
	}
	if updated == nil {
		return nil, status.Errorf(codes.NotFound, "issue %q not found", id)
	}

	return &issuesv1.UpdateIssueStatusResponse{
		Issue: domain.IssueToProto(updated),
	}, nil
}

// BatchUpdateIssueSprint updates sprint assignment across multiple issues.
func (s *Server) BatchUpdateIssueSprint(ctx context.Context, req *issuesv1.BatchUpdateIssueSprintRequest) (*issuesv1.BatchUpdateIssueSprintResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	issueIDs := req.GetIssueIds()
	if len(issueIDs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "issue_ids cannot be empty")
	}

	sprintID := strings.TrimSpace(req.GetSprintId())

	updatedIssues, count, err := s.svc.BatchUpdateIssueSprint(ctx, issueIDs, sprintID)
	if err != nil {
		if errors.Is(err, domain.ErrEmptyIssueIDs) {
			return nil, status.Errorf(codes.InvalidArgument, "issue_ids cannot be empty: %v", err)
		}
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "sprint or issues not found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to batch update issues sprint: %v", err)
	}

	protoIssues := make([]*issuesv1.Issue, len(updatedIssues))
	for i, iss := range updatedIssues {
		protoIssues[i] = domain.IssueToProto(iss)
	}

	return &issuesv1.BatchUpdateIssueSprintResponse{
		UpdatedCount: count,
		Issues:       protoIssues,
	}, nil
}
