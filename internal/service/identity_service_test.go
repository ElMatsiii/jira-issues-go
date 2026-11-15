package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/security"
	"jira-issues-go/internal/service"
)

const (
	workspaceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	ownerID     = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	adminID     = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	memberID    = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	outsiderID  = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"

	// Hash created by the BFF (Node scryptSync) for legacyPassword.
	legacyPassword = "contraseña-segura-2026"
	legacyHash     = "scrypt$9f2c4e6a8b0d1f3e5a7c9e1b3d5f7a90$" +
		"cc918220639e699ced6548bfa7fd92479f9816f89e0ec3259ac4a6b63e456dc6f761159d10ce371ad8ea3cc2fd8b2ec028b2697cd7ae97e992dd646fed11ffdd"
)

// teamRepo has one workspace with an owner, an admin and a member, plus a registered outsider.
func teamRepo() *service.MockIdentityRepository {
	repo := service.NewMockIdentityRepository()
	for id, email := range map[string]string{ownerID: "owner@example.com", adminID: "admin@example.com", memberID: "member@example.com", outsiderID: "outsider@example.com"} {
		repo.Users[id] = &domain.User{ID: id, Name: email[:5], Email: email, PasswordHash: legacyHash}
	}
	repo.AddWorkspace(workspaceID, ownerID, domain.RoleOwner)
	repo.AddWorkspace(workspaceID, adminID, domain.RoleAdmin)
	repo.AddWorkspace(workspaceID, memberID, domain.RoleMember)
	return repo
}

func TestRegisterUser_CreatesOwnerWorkspaceAndBFFCompatibleHash(t *testing.T) {
	repo := service.NewMockIdentityRepository()
	svc := service.NewIdentityService(repo)

	user, workspace, err := svc.RegisterUser(context.Background(), " Ana ", "ANA@example.com", legacyPassword)

	require.NoError(t, err)
	assert.Equal(t, "ana@example.com", user.Email)
	assert.Equal(t, "Ana's workspace", workspace.Name)
	assert.Equal(t, domain.RoleOwner, workspace.Role)
	stored := repo.Users[user.ID]
	assert.Regexp(t, `^scrypt\$[0-9a-f]{32}\$[0-9a-f]{128}$`, stored.PasswordHash)
	assert.True(t, security.VerifyPassword(legacyPassword, stored.PasswordHash))
	assert.Equal(t, domain.RoleOwner, repo.Members[workspace.ID+"/"+user.ID])
}

func TestRegisterUser_Errors(t *testing.T) {
	repo := teamRepo()
	svc := service.NewIdentityService(repo)
	ctx := context.Background()

	_, _, err := svc.RegisterUser(ctx, "Otra", " Owner@Example.com", "0123456789")
	assert.ErrorIs(t, err, domain.ErrEmailTaken)

	before := len(repo.Users)
	_, _, err = svc.RegisterUser(ctx, "Ana", "ana@example.com", "corta")
	assert.ErrorIs(t, err, domain.ErrInvalidUserInput)
	assert.Len(t, repo.Users, before, "invalid data never reaches the repository")
}

func TestAuthenticateUser(t *testing.T) {
	svc := service.NewIdentityService(teamRepo())
	ctx := context.Background()

	user, err := svc.AuthenticateUser(ctx, "  OWNER@example.com ", legacyPassword)
	require.NoError(t, err, "accounts created by the BFF keep working")
	assert.Equal(t, ownerID, user.ID)

	_, err = svc.AuthenticateUser(ctx, "owner@example.com", "otra-contraseña")
	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)

	_, err = svc.AuthenticateUser(ctx, "nadie@example.com", legacyPassword)
	assert.ErrorIs(t, err, domain.ErrInvalidCredentials, "an unknown email looks like a wrong password")
}

func TestAddWorkspaceMember_RequiresOwnerOrAdmin(t *testing.T) {
	ctx := context.Background()

	for _, requester := range []string{ownerID, adminID} {
		repo := teamRepo()
		member, role, err := service.NewIdentityService(repo).AddWorkspaceMember(ctx, requester, workspaceID, " Outsider@Example.com ")
		require.NoError(t, err)
		assert.Equal(t, outsiderID, member.ID)
		assert.Equal(t, domain.RoleMember, role)
		assert.Equal(t, domain.RoleMember, repo.Members[workspaceID+"/"+outsiderID])
	}

	for _, requester := range []string{memberID, outsiderID} {
		repo := teamRepo()
		_, _, err := service.NewIdentityService(repo).AddWorkspaceMember(ctx, requester, workspaceID, "outsider@example.com")
		assert.ErrorIs(t, err, domain.ErrWorkspaceForbidden)
		assert.NotContains(t, repo.Members, workspaceID+"/"+outsiderID)
	}
}

func TestAddWorkspaceMember_UnknownEmailAndExistingMember(t *testing.T) {
	svc := service.NewIdentityService(teamRepo())
	ctx := context.Background()

	_, _, err := svc.AddWorkspaceMember(ctx, ownerID, workspaceID, "nadie@example.com")
	assert.ErrorIs(t, err, domain.ErrUserNotFound)

	_, role, err := svc.AddWorkspaceMember(ctx, ownerID, workspaceID, "admin@example.com")
	require.NoError(t, err)
	assert.Equal(t, domain.RoleAdmin, role, "an existing member keeps its role")
}

func TestCreateProject_RolesAndValidation(t *testing.T) {
	ctx := context.Background()
	repo := teamRepo()
	svc := service.NewIdentityService(repo)

	project, err := svc.CreateProject(ctx, adminID, workspaceID, " web ", "Portal", "")
	require.NoError(t, err)
	assert.Equal(t, "WEB", project.Key)
	assert.Equal(t, workspaceID, project.WorkspaceID)
	assert.NotEmpty(t, project.ID)

	_, err = svc.CreateProject(ctx, ownerID, workspaceID, "WEB", "Otro portal", "")
	assert.ErrorIs(t, err, domain.ErrProjectKeyTaken)

	_, err = svc.CreateProject(ctx, ownerID, workspaceID, "W", "Portal", "")
	assert.ErrorIs(t, err, domain.ErrInvalidProjectInput)

	for _, requester := range []string{memberID, outsiderID} {
		_, err = svc.CreateProject(ctx, requester, workspaceID, "APP", "App", "")
		assert.ErrorIs(t, err, domain.ErrWorkspaceForbidden)
	}
	assert.Len(t, repo.Projects, 1)
}

func TestListProjects_OwnerSeesAllAndMemberOnlyItsProjects(t *testing.T) {
	ctx := context.Background()
	svc := service.NewIdentityService(teamRepo())
	old, err := svc.CreateProject(ctx, ownerID, workspaceID, "OLD", "Antiguo", "")
	require.NoError(t, err)
	_, err = svc.CreateProject(ctx, ownerID, workspaceID, "NEW", "Nuevo", "")
	require.NoError(t, err)

	for _, manager := range []string{ownerID, adminID} {
		projects, err := svc.ListProjects(ctx, manager, workspaceID)
		require.NoError(t, err)
		require.Len(t, projects, 2)
		assert.Equal(t, "NEW", projects[0].Key, "newest first")
	}

	projects, err := svc.ListProjects(ctx, memberID, workspaceID)
	require.NoError(t, err)
	assert.Empty(t, projects, "a MEMBER sees no project until it is added to one")

	_, err = svc.AddProjectMember(ctx, ownerID, old.ID, "member@example.com")
	require.NoError(t, err)
	projects, err = svc.ListProjects(ctx, memberID, workspaceID)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "OLD", projects[0].Key)

	_, err = svc.ListProjects(ctx, outsiderID, workspaceID)
	assert.ErrorIs(t, err, domain.ErrWorkspaceForbidden)
}

func TestCheckProjectAccess(t *testing.T) {
	ctx := context.Background()
	svc := service.NewIdentityService(teamRepo())
	project, err := svc.CreateProject(ctx, ownerID, workspaceID, "WEB", "Portal", "")
	require.NoError(t, err)

	access, err := svc.CheckProjectAccess(ctx, adminID, project.ID)
	require.NoError(t, err)
	assert.Equal(t, service.ProjectAccess{Allowed: true, Role: domain.RoleAdmin, WorkspaceID: workspaceID}, access,
		"the OWNER and ADMINs reach every project")

	access, err = svc.CheckProjectAccess(ctx, memberID, project.ID)
	require.NoError(t, err)
	assert.Equal(t, service.ProjectAccess{}, access, "a MEMBER not added to the project has no access")

	_, err = svc.AddProjectMember(ctx, ownerID, project.ID, "member@example.com")
	require.NoError(t, err)
	access, err = svc.CheckProjectAccess(ctx, memberID, project.ID)
	require.NoError(t, err)
	assert.Equal(t, service.ProjectAccess{Allowed: true, Role: domain.RoleMember, WorkspaceID: workspaceID}, access)

	access, err = svc.CheckProjectAccess(ctx, outsiderID, project.ID)
	require.NoError(t, err)
	assert.Equal(t, service.ProjectAccess{}, access, "a non-member gets allowed=false and no workspace id")

	access, err = svc.CheckProjectAccess(ctx, ownerID, "ffffffff-ffff-4fff-8fff-ffffffffffff")
	require.NoError(t, err)
	assert.False(t, access.Allowed, "unknown projects are simply not accessible")
}

func TestIdentityService_RepositoryFailuresAreReturned(t *testing.T) {
	ctx := context.Background()
	repo := teamRepo()
	repo.Err = errors.New("connection reset")
	svc := service.NewIdentityService(repo)

	_, _, err := svc.RegisterUser(ctx, "Ana", "ana@example.com", legacyPassword)
	assert.ErrorIs(t, err, repo.Err)
	_, err = svc.AuthenticateUser(ctx, "owner@example.com", legacyPassword)
	assert.ErrorIs(t, err, repo.Err)
	assert.NotErrorIs(t, err, domain.ErrInvalidCredentials, "a database failure is not reported as bad credentials")
	_, err = svc.GetUser(ctx, ownerID)
	assert.ErrorIs(t, err, repo.Err)
	_, err = svc.ListUserWorkspaces(ctx, ownerID)
	assert.ErrorIs(t, err, repo.Err)
	_, err = svc.GetMemberRole(ctx, workspaceID, ownerID)
	assert.ErrorIs(t, err, repo.Err)
	_, _, err = svc.AddWorkspaceMember(ctx, ownerID, workspaceID, "outsider@example.com")
	assert.ErrorIs(t, err, repo.Err)
	_, err = svc.CreateProject(ctx, ownerID, workspaceID, "WEB", "Portal", "")
	assert.ErrorIs(t, err, repo.Err)
	_, err = svc.ListProjects(ctx, ownerID, workspaceID)
	assert.ErrorIs(t, err, repo.Err)
	_, err = svc.CheckProjectAccess(ctx, ownerID, workspaceID)
	assert.ErrorIs(t, err, repo.Err)
	_, err = svc.AddProjectMember(ctx, ownerID, workspaceID, "outsider@example.com")
	assert.ErrorIs(t, err, repo.Err)
	_, err = svc.ListProjectMembers(ctx, ownerID, workspaceID)
	assert.ErrorIs(t, err, repo.Err)
	assert.ErrorIs(t, svc.RemoveProjectMember(ctx, ownerID, workspaceID, memberID), repo.Err)
}
