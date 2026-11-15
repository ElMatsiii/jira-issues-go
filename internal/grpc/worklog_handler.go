package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/service"
)

// WorkLogServer implements issuesv1.WorkLogServiceServer.
type WorkLogServer struct {
	issuesv1.UnimplementedWorkLogServiceServer
	svc service.WorkLogService
}

func NewWorkLogServer(svc service.WorkLogService) *WorkLogServer {
	return &WorkLogServer{svc: svc}
}

func workLogError(operation string, err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidWorkLog):
		return status.Errorf(codes.InvalidArgument, "validation error: %v", err)
	case errors.Is(err, domain.ErrProjectForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrIssueNotInProject):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Errorf(codes.Internal, "failed to %s: %v", operation, err)
	}
}

func (s *WorkLogServer) LogWork(ctx context.Context, req *issuesv1.LogWorkRequest) (*issuesv1.LogWorkResponse, error) {
	userID, err := requireUUID("user_id", req.GetUserId())
	if err != nil {
		return nil, err
	}
	projectID, err := requireUUID("project_id", req.GetProjectId())
	if err != nil {
		return nil, err
	}
	issueID, err := requireUUID("issue_id", req.GetIssueId())
	if err != nil {
		return nil, err
	}
	log, err := s.svc.LogWork(ctx, userID, projectID, issueID, req.GetHours(), req.GetNote())
	if err != nil {
		return nil, workLogError("log work", err)
	}
	return &issuesv1.LogWorkResponse{WorkLog: domain.WorkLogToProto(log)}, nil
}

func (s *WorkLogServer) ListWorkLogs(ctx context.Context, req *issuesv1.ListWorkLogsRequest) (*issuesv1.ListWorkLogsResponse, error) {
	requesterID, err := requireUUID("requester_id", req.GetRequesterId())
	if err != nil {
		return nil, err
	}
	projectID, err := requireUUID("project_id", req.GetProjectId())
	if err != nil {
		return nil, err
	}
	logs, total, err := s.svc.ListWorkLogs(ctx, requesterID, projectID, int(req.GetLimit()))
	if err != nil {
		return nil, workLogError("list work logs", err)
	}
	entries := make([]*issuesv1.WorkLog, len(logs))
	for i, log := range logs {
		entries[i] = domain.WorkLogToProto(log)
	}
	return &issuesv1.ListWorkLogsResponse{Entries: entries, TotalHours: total}, nil
}

func (s *WorkLogServer) SumSprintHours(ctx context.Context, req *issuesv1.SumSprintHoursRequest) (*issuesv1.SumSprintHoursResponse, error) {
	projectID, err := requireUUID("project_id", req.GetProjectId())
	if err != nil {
		return nil, err
	}
	// Sprint ids are UUIDs (issues.sprint_id is a UUID column).
	sprintID, err := requireUUID("sprint_id", req.GetSprintId())
	if err != nil {
		return nil, err
	}
	total, err := s.svc.SumSprintHours(ctx, projectID, sprintID)
	if err != nil {
		return nil, workLogError("sum sprint hours", err)
	}
	return &issuesv1.SumSprintHoursResponse{TotalHours: total}, nil
}
