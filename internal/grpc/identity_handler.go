package grpc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/service"
)

// IdentityServer implements issuesv1.IdentityServiceServer.
type IdentityServer struct {
	issuesv1.UnimplementedIdentityServiceServer
	svc service.IdentityService
}

// ProjectServer implements issuesv1.ProjectServiceServer.
type ProjectServer struct {
	issuesv1.UnimplementedProjectServiceServer
	svc service.IdentityService
}

func NewIdentityServer(svc service.IdentityService) *IdentityServer {
	return &IdentityServer{svc: svc}
}

func NewProjectServer(svc service.IdentityService) *ProjectServer {
	return &ProjectServer{svc: svc}
}

// requireUUID trims value and returns InvalidArgument when it is empty or not a UUID.
func requireUUID(field, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", status.Errorf(codes.InvalidArgument, "%s is required", field)
	}
	if _, err := uuid.Parse(trimmed); err != nil {
		return "", status.Errorf(codes.InvalidArgument, "%s must be a valid UUID, got %q", field, trimmed)
	}
	return trimmed, nil
}

// identityError maps service errors to gRPC codes; anything unexpected is Internal.
func identityError(operation string, err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidUserInput), errors.Is(err, domain.ErrInvalidProjectInput):
		return status.Errorf(codes.InvalidArgument, "validation error: %v", err)
	case errors.Is(err, domain.ErrEmailTaken), errors.Is(err, domain.ErrProjectKeyTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrWorkspaceForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Errorf(codes.Internal, "failed to %s: %v", operation, err)
	}
}

func (s *IdentityServer) RegisterUser(ctx context.Context, req *issuesv1.RegisterUserRequest) (*issuesv1.RegisterUserResponse, error) {
	user, workspace, err := s.svc.RegisterUser(ctx, req.GetName(), req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, identityError("register user", err)
	}
	return &issuesv1.RegisterUserResponse{User: domain.UserToProto(user), Workspace: domain.WorkspaceToProto(workspace)}, nil
}

func (s *IdentityServer) AuthenticateUser(ctx context.Context, req *issuesv1.AuthenticateUserRequest) (*issuesv1.AuthenticateUserResponse, error) {
	user, err := s.svc.AuthenticateUser(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, identityError("authenticate user", err)
	}
	return &issuesv1.AuthenticateUserResponse{User: domain.UserToProto(user)}, nil
}

func (s *IdentityServer) GetUser(ctx context.Context, req *issuesv1.GetUserRequest) (*issuesv1.GetUserResponse, error) {
	id, err := requireUUID("id", req.GetId())
	if err != nil {
		return nil, err
	}
	user, err := s.svc.GetUser(ctx, id)
	if err != nil {
		return nil, identityError("get user", err)
	}
	return &issuesv1.GetUserResponse{User: domain.UserToProto(user)}, nil
}

func (s *IdentityServer) ListUserWorkspaces(ctx context.Context, req *issuesv1.ListUserWorkspacesRequest) (*issuesv1.ListUserWorkspacesResponse, error) {
	userID, err := requireUUID("user_id", req.GetUserId())
	if err != nil {
		return nil, err
	}
	workspaces, err := s.svc.ListUserWorkspaces(ctx, userID)
	if err != nil {
		return nil, identityError("list workspaces", err)
	}
	result := make([]*issuesv1.Workspace, len(workspaces))
	for i, workspace := range workspaces {
		result[i] = domain.WorkspaceToProto(workspace)
	}
	return &issuesv1.ListUserWorkspacesResponse{Workspaces: result}, nil
}

func (s *IdentityServer) AddWorkspaceMember(ctx context.Context, req *issuesv1.AddWorkspaceMemberRequest) (*issuesv1.AddWorkspaceMemberResponse, error) {
	requesterID, err := requireUUID("requester_id", req.GetRequesterId())
	if err != nil {
		return nil, err
	}
	workspaceID, err := requireUUID("workspace_id", req.GetWorkspaceId())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetMemberEmail()) == "" {
		return nil, status.Error(codes.InvalidArgument, "member_email is required")
	}
	member, role, err := s.svc.AddWorkspaceMember(ctx, requesterID, workspaceID, req.GetMemberEmail())
	if err != nil {
		return nil, identityError("add workspace member", err)
	}
	return &issuesv1.AddWorkspaceMemberResponse{Member: domain.UserToProto(member), Role: domain.RoleToProto(role)}, nil
}

func (s *IdentityServer) GetMemberRole(ctx context.Context, req *issuesv1.GetMemberRoleRequest) (*issuesv1.GetMemberRoleResponse, error) {
	workspaceID, err := requireUUID("workspace_id", req.GetWorkspaceId())
	if err != nil {
		return nil, err
	}
	userID, err := requireUUID("user_id", req.GetUserId())
	if err != nil {
		return nil, err
	}
	role, err := s.svc.GetMemberRole(ctx, workspaceID, userID)
	if err != nil {
		return nil, identityError("get member role", err)
	}
	return &issuesv1.GetMemberRoleResponse{Role: domain.RoleToProto(role)}, nil
}

func (s *ProjectServer) CreateProject(ctx context.Context, req *issuesv1.CreateProjectRequest) (*issuesv1.CreateProjectResponse, error) {
	requesterID, err := requireUUID("requester_id", req.GetRequesterId())
	if err != nil {
		return nil, err
	}
	workspaceID, err := requireUUID("workspace_id", req.GetWorkspaceId())
	if err != nil {
		return nil, err
	}
	project, err := s.svc.CreateProject(ctx, requesterID, workspaceID, req.GetKey(), req.GetName(), req.GetDescription())
	if err != nil {
		return nil, identityError("create project", err)
	}
	return &issuesv1.CreateProjectResponse{Project: domain.ProjectToProto(project)}, nil
}

func (s *ProjectServer) ListProjects(ctx context.Context, req *issuesv1.ListProjectsRequest) (*issuesv1.ListProjectsResponse, error) {
	requesterID, err := requireUUID("requester_id", req.GetRequesterId())
	if err != nil {
		return nil, err
	}
	workspaceID, err := requireUUID("workspace_id", req.GetWorkspaceId())
	if err != nil {
		return nil, err
	}
	projects, err := s.svc.ListProjects(ctx, requesterID, workspaceID)
	if err != nil {
		return nil, identityError("list projects", err)
	}
	result := make([]*issuesv1.Project, len(projects))
	for i, project := range projects {
		result[i] = domain.ProjectToProto(project)
	}
	return &issuesv1.ListProjectsResponse{Projects: result}, nil
}

func (s *ProjectServer) CheckProjectAccess(ctx context.Context, req *issuesv1.CheckProjectAccessRequest) (*issuesv1.CheckProjectAccessResponse, error) {
	userID, err := requireUUID("user_id", req.GetUserId())
	if err != nil {
		return nil, err
	}
	projectID, err := requireUUID("project_id", req.GetProjectId())
	if err != nil {
		return nil, err
	}
	access, err := s.svc.CheckProjectAccess(ctx, userID, projectID)
	if err != nil {
		return nil, identityError("check project access", err)
	}
	return &issuesv1.CheckProjectAccessResponse{
		Allowed:     access.Allowed,
		Role:        domain.RoleToProto(access.Role),
		WorkspaceId: access.WorkspaceID,
	}, nil
}
