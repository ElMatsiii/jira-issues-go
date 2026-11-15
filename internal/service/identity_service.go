package service

import (
	"context"
	"errors"
	"fmt"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
	"jira-issues-go/internal/security"
)

// ProjectAccess is the answer to "can this user open this project?".
type ProjectAccess struct {
	Allowed     bool
	Role        domain.WorkspaceRole
	WorkspaceID string // empty when not allowed
}

// IdentityService manages accounts, workspaces, members and workspace projects. Every operation that
// acts on a workspace checks the requester's role here, in MS1.
type IdentityService interface {
	RegisterUser(ctx context.Context, name, email, password string) (*domain.User, *domain.Workspace, error)
	AuthenticateUser(ctx context.Context, email, password string) (*domain.User, error)
	GetUser(ctx context.Context, id string) (*domain.User, error)
	ListUserWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error)
	AddWorkspaceMember(ctx context.Context, requesterID, workspaceID, memberEmail string) (*domain.User, domain.WorkspaceRole, error)
	GetMemberRole(ctx context.Context, workspaceID, userID string) (domain.WorkspaceRole, error)
	CreateProject(ctx context.Context, requesterID, workspaceID, key, name, description string) (*domain.Project, error)
	ListProjects(ctx context.Context, requesterID, workspaceID string) ([]*domain.Project, error)
	CheckProjectAccess(ctx context.Context, userID, projectID string) (ProjectAccess, error)
}

// timingHash is verified when the email does not exist, so an unknown email costs the same scrypt
// work as a wrong password and response times do not reveal which accounts exist.
const timingHash = "scrypt$00000000000000000000000000000000$" +
	"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

type identityService struct {
	repo repository.IdentityRepository
}

// NewIdentityService constructs the IdentityService.
func NewIdentityService(repo repository.IdentityRepository) IdentityService {
	return &identityService{repo: repo}
}

func (s *identityService) RegisterUser(ctx context.Context, name, email, password string) (*domain.User, *domain.Workspace, error) {
	user, err := domain.NewUser(name, email, password)
	if err != nil {
		return nil, nil, err
	}
	user.PasswordHash, err = security.HashPassword(password)
	if err != nil {
		return nil, nil, err
	}
	workspace, err := s.repo.CreateUserWithWorkspace(ctx, user, domain.PersonalWorkspaceName(user.Name))
	if err != nil {
		return nil, nil, err
	}
	return user, workspace, nil
}

func (s *identityService) AuthenticateUser(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := s.repo.GetUserByEmail(ctx, domain.NormalizeEmail(email))
	if errors.Is(err, domain.ErrUserNotFound) {
		security.VerifyPassword(password, timingHash)
		return nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !security.VerifyPassword(password, user.PasswordHash) {
		return nil, domain.ErrInvalidCredentials
	}
	return user, nil
}

func (s *identityService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.GetUserByID(ctx, id)
}

func (s *identityService) ListUserWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error) {
	return s.repo.ListUserWorkspaces(ctx, userID)
}

func (s *identityService) GetMemberRole(ctx context.Context, workspaceID, userID string) (domain.WorkspaceRole, error) {
	return s.repo.GetMemberRole(ctx, workspaceID, userID)
}

// requireRole returns domain.ErrWorkspaceForbidden unless the user's role passes allowed. Not being a
// member and not having enough permissions look the same, so workspace ids cannot be probed.
func (s *identityService) requireRole(ctx context.Context, workspaceID, userID string, allowed func(domain.WorkspaceRole) bool) (domain.WorkspaceRole, error) {
	role, err := s.repo.GetMemberRole(ctx, workspaceID, userID)
	if err != nil {
		return domain.RoleNone, err
	}
	if !allowed(role) {
		return domain.RoleNone, domain.ErrWorkspaceForbidden
	}
	return role, nil
}

func (s *identityService) AddWorkspaceMember(ctx context.Context, requesterID, workspaceID, memberEmail string) (*domain.User, domain.WorkspaceRole, error) {
	if _, err := s.requireRole(ctx, workspaceID, requesterID, domain.WorkspaceRole.CanManage); err != nil {
		return nil, domain.RoleNone, err
	}
	member, err := s.repo.GetUserByEmail(ctx, domain.NormalizeEmail(memberEmail))
	if err != nil {
		return nil, domain.RoleNone, err
	}
	role, err := s.repo.AddMember(ctx, workspaceID, member.ID, domain.RoleMember)
	if err != nil {
		return nil, domain.RoleNone, err
	}
	return member, role, nil
}

func (s *identityService) CreateProject(ctx context.Context, requesterID, workspaceID, key, name, description string) (*domain.Project, error) {
	if _, err := s.requireRole(ctx, workspaceID, requesterID, domain.WorkspaceRole.CanManage); err != nil {
		return nil, err
	}
	project, err := domain.NewWorkspaceProject(workspaceID, key, name, description)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateWorkspaceProject(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}

func (s *identityService) ListProjects(ctx context.Context, requesterID, workspaceID string) ([]*domain.Project, error) {
	if _, err := s.requireRole(ctx, workspaceID, requesterID, domain.WorkspaceRole.IsMember); err != nil {
		return nil, err
	}
	return s.repo.ListWorkspaceProjects(ctx, workspaceID)
}

func (s *identityService) CheckProjectAccess(ctx context.Context, userID, projectID string) (ProjectAccess, error) {
	workspaceID, err := s.repo.GetProjectWorkspaceID(ctx, projectID)
	if errors.Is(err, domain.ErrProjectNotFound) {
		// Unknown project or one not linked to any workspace: no access, not an error.
		return ProjectAccess{}, nil
	}
	if err != nil {
		return ProjectAccess{}, fmt.Errorf("check project access: %w", err)
	}
	role, err := s.repo.GetMemberRole(ctx, workspaceID, userID)
	if err != nil {
		return ProjectAccess{}, fmt.Errorf("check project access: %w", err)
	}
	if !role.IsMember() {
		return ProjectAccess{}, nil
	}
	return ProjectAccess{Allowed: true, Role: role, WorkspaceID: workspaceID}, nil
}
