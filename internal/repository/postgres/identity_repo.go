package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
)

type identityRepo struct {
	db *sql.DB
}

// NewIdentityRepository constructs the PostgreSQL IdentityRepository (migration 000003).
func NewIdentityRepository(db *sql.DB) repository.IdentityRepository {
	return &identityRepo{db: db}
}

// inTx runs fn in a transaction and rolls it back when fn fails.
func (r *identityRepo) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *identityRepo) CreateUserWithWorkspace(ctx context.Context, user *domain.User, workspaceName string) (*domain.Workspace, error) {
	workspace := &domain.Workspace{Name: workspaceName, Role: domain.RoleOwner}
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx,
			`INSERT INTO app_users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id, created_at`,
			user.Name, user.Email, user.PasswordHash,
		).Scan(&user.ID, &user.CreatedAt)
		if err != nil {
			if errors.Is(MapPgError(err), ErrDuplicateKey) {
				return domain.ErrEmailTaken
			}
			return fmt.Errorf("insert user: %w", err)
		}
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO workspaces (name) VALUES ($1) RETURNING id, created_at`, workspaceName,
		).Scan(&workspace.ID, &workspace.CreatedAt); err != nil {
			return fmt.Errorf("insert workspace: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
			workspace.ID, user.ID, string(domain.RoleOwner),
		); err != nil {
			return fmt.Errorf("insert workspace owner: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return workspace, nil
}

func (r *identityRepo) getUser(ctx context.Context, where string, arg string) (*domain.User, error) {
	var user domain.User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, created_at FROM app_users WHERE `+where, arg,
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read user: %w", err)
	}
	return &user, nil
}

func (r *identityRepo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.getUser(ctx, "email = $1", email)
}

func (r *identityRepo) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	return r.getUser(ctx, "id = $1", id)
}

func (r *identityRepo) ListUserWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT w.id, w.name, wm.role, w.created_at
		   FROM workspaces w
		   JOIN workspace_members wm ON wm.workspace_id = w.id
		  WHERE wm.user_id = $1
		  ORDER BY w.created_at, w.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()

	workspaces := []*domain.Workspace{}
	for rows.Next() {
		var workspace domain.Workspace
		var role string
		if err := rows.Scan(&workspace.ID, &workspace.Name, &role, &workspace.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		workspace.Role = domain.ParseWorkspaceRole(role)
		workspaces = append(workspaces, &workspace)
	}
	return workspaces, rows.Err()
}

func (r *identityRepo) GetMemberRole(ctx context.Context, workspaceID, userID string) (domain.WorkspaceRole, error) {
	var role string
	err := r.db.QueryRowContext(ctx,
		`SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`, workspaceID, userID,
	).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RoleNone, nil
	}
	if err != nil {
		return domain.RoleNone, fmt.Errorf("read member role: %w", err)
	}
	return domain.ParseWorkspaceRole(role), nil
}

func (r *identityRepo) AddMember(ctx context.Context, workspaceID, userID string, role domain.WorkspaceRole) (domain.WorkspaceRole, error) {
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (workspace_id, user_id) DO NOTHING`,
		workspaceID, userID, string(role),
	); err != nil {
		return domain.RoleNone, fmt.Errorf("add member: %w", err)
	}
	// An existing member keeps the role it already had.
	return r.GetMemberRole(ctx, workspaceID, userID)
}

func (r *identityRepo) CreateWorkspaceProject(ctx context.Context, project *domain.Project) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx,
			`INSERT INTO projects (key, name, description) VALUES ($1, $2, $3)
			 RETURNING id, issue_counter, created_at, updated_at`,
			project.Key, project.Name, project.Description,
		).Scan(&project.ID, &project.IssueCounter, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			if errors.Is(MapPgError(err), ErrDuplicateKey) {
				return domain.ErrProjectKeyTaken
			}
			return fmt.Errorf("insert project: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app_projects (project_id, workspace_id) VALUES ($1, $2)`, project.ID, project.WorkspaceID,
		); err != nil {
			return fmt.Errorf("link project to workspace: %w", err)
		}
		return nil
	})
}

// projectSelect reads a workspace project; queryProjects scans the columns in this order.
const projectSelect = `SELECT p.id, ap.workspace_id, p.key, p.name, p.description, p.issue_counter, p.created_at, p.updated_at
		   FROM app_projects ap
		   JOIN projects p ON p.id = ap.project_id`

func (r *identityRepo) ListWorkspaceProjects(ctx context.Context, workspaceID string) ([]*domain.Project, error) {
	return r.queryProjects(ctx, projectSelect+`
		  WHERE ap.workspace_id = $1
		  ORDER BY p.created_at DESC, p.id`, workspaceID)
}

func (r *identityRepo) ListMemberProjects(ctx context.Context, workspaceID, userID string) ([]*domain.Project, error) {
	return r.queryProjects(ctx, projectSelect+`
		   JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = $2
		  WHERE ap.workspace_id = $1
		  ORDER BY p.created_at DESC, p.id`, workspaceID, userID)
}

func (r *identityRepo) queryProjects(ctx context.Context, query string, args ...any) ([]*domain.Project, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := []*domain.Project{}
	for rows.Next() {
		var project domain.Project
		if err := rows.Scan(&project.ID, &project.WorkspaceID, &project.Key, &project.Name, &project.Description,
			&project.IssueCounter, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, &project)
	}
	return projects, rows.Err()
}

func (r *identityRepo) GetProjectWorkspaceID(ctx context.Context, projectID string) (string, error) {
	var workspaceID string
	err := r.db.QueryRowContext(ctx,
		`SELECT workspace_id FROM app_projects WHERE project_id = $1`, projectID,
	).Scan(&workspaceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrProjectNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read project workspace: %w", err)
	}
	return workspaceID, nil
}

func (r *identityRepo) IsProjectMember(ctx context.Context, projectID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2)`, projectID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("read project member: %w", err)
	}
	return exists, nil
}

func (r *identityRepo) AddProjectMember(ctx context.Context, projectID, userID string) error {
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO project_members (project_id, user_id) VALUES ($1, $2) ON CONFLICT (project_id, user_id) DO NOTHING`,
		projectID, userID,
	); err != nil {
		return fmt.Errorf("add project member: %w", err)
	}
	return nil
}

func (r *identityRepo) RemoveProjectMember(ctx context.Context, projectID, userID string) (bool, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID)
	if err != nil {
		return false, fmt.Errorf("remove project member: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("remove project member: %w", err)
	}
	return removed > 0, nil
}

func (r *identityRepo) ListProjectMembers(ctx context.Context, projectID, workspaceID string) ([]*domain.ProjectMember, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT u.id, u.name, u.email, u.created_at, wm.role
		   FROM workspace_members wm
		   JOIN app_users u ON u.id = wm.user_id
		   LEFT JOIN project_members pm ON pm.project_id = $1 AND pm.user_id = wm.user_id
		  WHERE wm.workspace_id = $2
		    AND (wm.role IN ('OWNER', 'ADMIN') OR pm.user_id IS NOT NULL)
		  ORDER BY CASE wm.role WHEN 'OWNER' THEN 0 WHEN 'ADMIN' THEN 1 ELSE 2 END, u.name, u.id`,
		projectID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}
	defer rows.Close()

	members := []*domain.ProjectMember{}
	for rows.Next() {
		var user domain.User
		var role string
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &role); err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}
		parsed := domain.ParseWorkspaceRole(role)
		members = append(members, &domain.ProjectMember{User: &user, Role: parsed, Invited: !parsed.ReachesEveryProject()})
	}
	return members, rows.Err()
}
