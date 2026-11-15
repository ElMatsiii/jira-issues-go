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
	// UpdateStatus persists only issue.Status (plus version/updated_at) when the stored version
	// still equals expectedVersion; otherwise it returns domain.ErrVersionConflict (or
	// domain.ErrNotFound). On success it sets issue.Version and issue.UpdatedAt from the database.
	UpdateStatus(ctx context.Context, issue *domain.Issue, expectedVersion int) error
	BatchUpdateSprint(ctx context.Context, ids []string, sprintID string) ([]*domain.Issue, error)
	GetNextSequence(ctx context.Context, projectID string) (int64, string, error)
}

// IdentityRepository stores users, workspaces, members and the project-to-workspace link
// (tables app_users, workspaces, workspace_members and app_projects).
type IdentityRepository interface {
	// CreateUserWithWorkspace inserts the user, a workspace and the OWNER membership atomically and
	// fills user.ID/CreatedAt. A duplicate email returns domain.ErrEmailTaken.
	CreateUserWithWorkspace(ctx context.Context, user *domain.User, workspaceName string) (*domain.Workspace, error)
	// GetUserByEmail and GetUserByID return domain.ErrUserNotFound when there is no such user.
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	// ListUserWorkspaces returns the user's workspaces, oldest first, with the user's role.
	ListUserWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error)
	// GetMemberRole returns domain.RoleNone when the user is not a member.
	GetMemberRole(ctx context.Context, workspaceID, userID string) (domain.WorkspaceRole, error)
	// AddMember is idempotent: an existing member keeps its role, which is returned.
	AddMember(ctx context.Context, workspaceID, userID string, role domain.WorkspaceRole) (domain.WorkspaceRole, error)
	// CreateWorkspaceProject inserts the project and its app_projects row atomically and fills
	// project.ID/CreatedAt/UpdatedAt. A duplicate key returns domain.ErrProjectKeyTaken.
	CreateWorkspaceProject(ctx context.Context, project *domain.Project) error
	// ListWorkspaceProjects returns the workspace's projects, newest first.
	ListWorkspaceProjects(ctx context.Context, workspaceID string) ([]*domain.Project, error)
	// GetProjectWorkspaceID returns domain.ErrProjectNotFound when the project is not linked to a workspace.
	GetProjectWorkspaceID(ctx context.Context, projectID string) (string, error)
	// ListMemberProjects returns the workspace's projects the user was added to, newest first.
	ListMemberProjects(ctx context.Context, workspaceID, userID string) ([]*domain.Project, error)
	// IsProjectMember reports whether the user was added to the project (project_members).
	IsProjectMember(ctx context.Context, projectID, userID string) (bool, error)
	// AddProjectMember is idempotent.
	AddProjectMember(ctx context.Context, projectID, userID string) error
	// RemoveProjectMember returns false when the user was not a member of the project.
	RemoveProjectMember(ctx context.Context, projectID, userID string) (bool, error)
	// ListProjectMembers returns the OWNER, the ADMINs and the invited members of the project's
	// workspace, in that order and by name.
	ListProjectMembers(ctx context.Context, projectID, workspaceID string) ([]*domain.ProjectMember, error)
}

// WorkLogRepository stores hours logged per issue (table work_logs).
type WorkLogRepository interface {
	// Create inserts the entry and fills ID, LoggedAt, IssueKey, IssueTitle and UserName.
	Create(ctx context.Context, log *domain.WorkLog) error
	// ListByProject returns the project's newest entries first, at most limit.
	ListByProject(ctx context.Context, projectID string, limit int) ([]*domain.WorkLog, error)
	// SumProjectHours adds every entry of the project (not only a listed page).
	SumProjectHours(ctx context.Context, projectID string) (float64, error)
	// SumSprintHours adds the project's entries logged while their issue was in the sprint.
	SumSprintHours(ctx context.Context, projectID, sprintID string) (float64, error)
}

// ProjectRepository defines data access methods for Project entities.
type ProjectRepository interface {
	Create(ctx context.Context, project *domain.Project) error
	GetByID(ctx context.Context, id string) (*domain.Project, error)
	GetByKey(ctx context.Context, key string) (*domain.Project, error)
}
