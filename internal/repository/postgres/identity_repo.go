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

func (r *identityRepo) ListWorkspaceProjects(ctx context.Context, workspaceID string) ([]*domain.Project, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, ap.workspace_id, p.key, p.name, p.description, p.issue_counter, p.created_at, p.updated_at
		   FROM app_projects ap
		   JOIN projects p ON p.id = ap.project_id
		  WHERE ap.workspace_id = $1
		  ORDER BY p.created_at DESC, p.id`, workspaceID)
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
