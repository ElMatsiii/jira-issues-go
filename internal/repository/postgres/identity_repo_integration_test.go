package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository/postgres"
	"jira-issues-go/internal/service"
)

// bffIdentitySchema is what jira-bff/src/identity/database.service.ts ran on issues_db at start-up.
const bffIdentitySchema = `
CREATE TABLE IF NOT EXISTS app_users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(120) NOT NULL,
  email VARCHAR(320) NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS workspaces (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(160) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS workspace_members (
  workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  role VARCHAR(20) NOT NULL CHECK (role IN ('OWNER', 'ADMIN', 'MEMBER')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (workspace_id, user_id)
);
CREATE TABLE IF NOT EXISTS app_projects (
  project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
  workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_workspace_members_user ON workspace_members(user_id, workspace_id);
CREATE INDEX IF NOT EXISTS idx_app_projects_workspace ON app_projects(workspace_id);
CREATE TABLE IF NOT EXISTS work_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  sprint_id VARCHAR(255),
  hours NUMERIC(6,2) NOT NULL CHECK (hours > 0 AND hours <= 24),
  note VARCHAR(500) NOT NULL DEFAULT '',
  logged_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// Hash stored by the BFF (Node scryptSync) for legacyPassword.
const (
	legacyPassword = "contraseña-segura-2026"
	legacyHash     = "scrypt$9f2c4e6a8b0d1f3e5a7c9e1b3d5f7a90$" +
		"cc918220639e699ced6548bfa7fd92479f9816f89e0ec3259ac4a6b63e456dc6f761159d10ce371ad8ea3cc2fd8b2ec028b2697cd7ae97e992dd646fed11ffdd"
)

// openIsolatedSchema returns a connection whose search_path is a new, empty schema, dropped afterwards,
// so the test can rebuild issues_db from scratch without touching other data.
func openIsolatedSchema(t *testing.T) *sql.DB {
	t.Helper()
	admin := openIntegrationDB(t)
	schema := "it_identity_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	_, err := admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })

	separator := "?"
	if strings.Contains(os.Getenv("JIRA_ISSUES_TEST_DATABASE_URL"), "?") {
		separator = "&"
	}
	db, err := sql.Open("postgres", os.Getenv("JIRA_ISSUES_TEST_DATABASE_URL")+separator+"search_path="+schema)
	require.NoError(t, err)
	db.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// copyMigrations copies the given migration files to a temporary directory.
func copyMigrations(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "migrations", name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0o600))
	}
	return dir
}

func TestIntegration_IdentityMigration_KeepsWhatTheBFFCreated(t *testing.T) {
	db := openIsolatedSchema(t)
	ctx := context.Background()

	// 1. issues_db as it is today: MS1's first migrations, then the BFF's tables and its data.
	require.NoError(t, postgres.RunMigrations(db, copyMigrations(t, "000001_init_schema.up.sql", "000002_issue_parent.up.sql")))
	_, err := db.Exec(bffIdentitySchema)
	require.NoError(t, err)
	var userID, workspaceID string
	require.NoError(t, db.QueryRow(`INSERT INTO app_users (name, email, password_hash) VALUES ('Ana', 'ana@example.com', $1) RETURNING id`, legacyHash).Scan(&userID))
	require.NoError(t, db.QueryRow(`INSERT INTO workspaces (name) VALUES ('Ana''s workspace') RETURNING id`).Scan(&workspaceID))
	_, err = db.Exec(`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'OWNER')`, workspaceID, userID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO app_projects (project_id, workspace_id) VALUES ('00000000-0000-0000-0000-000000000001', $1)`, workspaceID)
	require.NoError(t, err)

	// 2. MS1 takes over: every migration (twice, as on two start-ups) runs over the existing tables.
	migrations := filepath.Join("..", "..", "..", "db", "migrations")
	require.NoError(t, postgres.RunMigrations(db, migrations))
	require.NoError(t, postgres.RunMigrations(db, migrations))

	// 3. Existing data is intact and the BFF's account logs in through MS1.
	svc := service.NewIdentityService(postgres.NewIdentityRepository(db))
	user, err := svc.AuthenticateUser(ctx, "ANA@example.com", legacyPassword)
	require.NoError(t, err)
	assert.Equal(t, userID, user.ID)
	_, err = svc.AuthenticateUser(ctx, "ana@example.com", "otra-contraseña")
	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)

	workspaces, err := svc.ListUserWorkspaces(ctx, userID)
	require.NoError(t, err)
	require.Len(t, workspaces, 1)
	assert.Equal(t, domain.RoleOwner, workspaces[0].Role)

	projects, err := svc.ListProjects(ctx, userID, workspaceID)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "PROJ", projects[0].Key)
	assert.Equal(t, workspaceID, projects[0].WorkspaceID)
}

func TestIntegration_IdentityRepository_FullFlow(t *testing.T) {
	db := openIsolatedSchema(t)
	require.NoError(t, postgres.RunMigrations(db, filepath.Join("..", "..", "..", "db", "migrations")))
	repo := postgres.NewIdentityRepository(db)
	svc := service.NewIdentityService(repo)
	ctx := context.Background()

	owner, workspace, err := svc.RegisterUser(ctx, "Owner", "owner@example.com", "contraseña-larga")
	require.NoError(t, err)
	assert.Equal(t, domain.RoleOwner, workspace.Role)
	assert.False(t, owner.CreatedAt.IsZero())
	_, _, err = svc.RegisterUser(ctx, "Otro", "OWNER@example.com", "contraseña-larga")
	assert.ErrorIs(t, err, domain.ErrEmailTaken)

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&count))
	assert.Equal(t, 1, count, "a rejected registration leaves no orphan workspace")

	guest, _, err := svc.RegisterUser(ctx, "Guest", "guest@example.com", "contraseña-larga")
	require.NoError(t, err)
	stored, err := repo.GetUserByID(ctx, guest.ID)
	require.NoError(t, err)
	assert.Equal(t, "guest@example.com", stored.Email)
	_, err = repo.GetUserByID(ctx, uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrUserNotFound)

	_, role, err := svc.AddWorkspaceMember(ctx, owner.ID, workspace.ID, "guest@example.com")
	require.NoError(t, err)
	assert.Equal(t, domain.RoleMember, role)
	_, role, err = svc.AddWorkspaceMember(ctx, owner.ID, workspace.ID, "owner@example.com")
	require.NoError(t, err)
	assert.Equal(t, domain.RoleOwner, role, "adding an existing member keeps its role")
	_, _, err = svc.AddWorkspaceMember(ctx, guest.ID, workspace.ID, "owner@example.com")
	assert.ErrorIs(t, err, domain.ErrWorkspaceForbidden)

	guestWorkspaces, err := svc.ListUserWorkspaces(ctx, guest.ID)
	require.NoError(t, err)
	require.Len(t, guestWorkspaces, 2, "oldest first: the owner's workspace predates the guest's own one")
	assert.Equal(t, workspace.ID, guestWorkspaces[0].ID)
	assert.Equal(t, domain.RoleMember, guestWorkspaces[0].Role)
	assert.Equal(t, domain.RoleOwner, guestWorkspaces[1].Role)

	first, err := svc.CreateProject(ctx, owner.ID, workspace.ID, "web", "Portal", " Clientes ")
	require.NoError(t, err)
	assert.Equal(t, "WEB", first.Key)
	second, err := svc.CreateProject(ctx, owner.ID, workspace.ID, "APP", "App", "")
	require.NoError(t, err)
	_, err = svc.CreateProject(ctx, owner.ID, workspace.ID, "WEB", "Duplicado", "")
	assert.ErrorIs(t, err, domain.ErrProjectKeyTaken)
	_, err = svc.CreateProject(ctx, guest.ID, workspace.ID, "GST", "Invitado", "")
	assert.ErrorIs(t, err, domain.ErrWorkspaceForbidden)

	projects, err := svc.ListProjects(ctx, guest.ID, workspace.ID)
	require.NoError(t, err)
	require.Len(t, projects, 2)
	assert.Equal(t, second.ID, projects[0].ID, "newest first")
	assert.Equal(t, "Clientes", projects[1].Description)

	// The new project issues keys like any other MS1 project.
	seq, prefix, err := postgres.NewIssueRepository(db).GetNextSequence(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), seq)
	assert.Equal(t, "WEB", prefix)

	access, err := svc.CheckProjectAccess(ctx, guest.ID, first.ID)
	require.NoError(t, err)
	assert.Equal(t, service.ProjectAccess{Allowed: true, Role: domain.RoleMember, WorkspaceID: workspace.ID}, access)
	stranger, _, err := svc.RegisterUser(ctx, "Stranger", "stranger@example.com", "contraseña-larga")
	require.NoError(t, err)
	access, err = svc.CheckProjectAccess(ctx, stranger.ID, first.ID)
	require.NoError(t, err)
	assert.False(t, access.Allowed)
	access, err = svc.CheckProjectAccess(ctx, owner.ID, uuid.NewString())
	require.NoError(t, err)
	assert.False(t, access.Allowed)
}
