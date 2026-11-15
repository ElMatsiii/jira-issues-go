package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/service"
)

const unknownProject = "ffffffff-ffff-4fff-8fff-ffffffffffff"

func newProject(t *testing.T, repo *service.MockIdentityRepository) (service.IdentityService, *domain.Project) {
	t.Helper()
	svc := service.NewIdentityService(repo)
	project, err := svc.CreateProject(context.Background(), ownerID, workspaceID, "WEB", "Portal", "")
	require.NoError(t, err)
	return svc, project
}

func TestAddProjectMember(t *testing.T) {
	ctx := context.Background()
	repo := teamRepo()
	svc, project := newProject(t, repo)

	// Someone outside the workspace joins it as MEMBER, with access to this project only.
	added, err := svc.AddProjectMember(ctx, adminID, project.ID, " Outsider@Example.com ")
	require.NoError(t, err)
	assert.Equal(t, outsiderID, added.User.ID)
	assert.Equal(t, domain.RoleMember, added.Role)
	assert.True(t, added.Invited)
	assert.Equal(t, domain.RoleMember, repo.Members[workspaceID+"/"+outsiderID])

	again, err := svc.AddProjectMember(ctx, ownerID, project.ID, "outsider@example.com")
	require.NoError(t, err)
	assert.True(t, again.Invited, "adding twice is not an error")

	// The OWNER and ADMINs already reach every project: returned as they are, not invited.
	admin, err := svc.AddProjectMember(ctx, ownerID, project.ID, "admin@example.com")
	require.NoError(t, err)
	assert.Equal(t, domain.RoleAdmin, admin.Role)
	assert.False(t, admin.Invited)
	assert.False(t, repo.ProjectMembers[project.ID+"/"+adminID])
}

func TestAddProjectMember_Rejections(t *testing.T) {
	ctx := context.Background()
	repo := teamRepo()
	svc, project := newProject(t, repo)
	repo.ProjectMembers[project.ID+"/"+memberID] = true

	_, err := svc.AddProjectMember(ctx, memberID, project.ID, "outsider@example.com")
	assert.ErrorIs(t, err, domain.ErrWorkspaceForbidden, "a MEMBER of the project cannot invite")
	_, err = svc.AddProjectMember(ctx, outsiderID, project.ID, "outsider@example.com")
	assert.ErrorIs(t, err, domain.ErrWorkspaceForbidden)
	_, err = svc.AddProjectMember(ctx, ownerID, unknownProject, "outsider@example.com")
	assert.ErrorIs(t, err, domain.ErrProjectForbidden, "an unknown project looks like one without access")
	_, err = svc.AddProjectMember(ctx, ownerID, project.ID, "nadie@example.com")
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
	assert.NotContains(t, repo.Members, workspaceID+"/"+outsiderID)
}

func TestListProjectMembers(t *testing.T) {
	ctx := context.Background()
	svc, project := newProject(t, teamRepo())
	_, err := svc.AddProjectMember(ctx, ownerID, project.ID, "member@example.com")
	require.NoError(t, err)

	members, err := svc.ListProjectMembers(ctx, memberID, project.ID)
	require.NoError(t, err)
	require.Len(t, members, 3)
	assert.Equal(t, []domain.WorkspaceRole{domain.RoleOwner, domain.RoleAdmin, domain.RoleMember},
		[]domain.WorkspaceRole{members[0].Role, members[1].Role, members[2].Role})
	assert.Equal(t, []bool{false, false, true}, []bool{members[0].Invited, members[1].Invited, members[2].Invited})
	assert.Empty(t, members[2].User.PasswordHash)

	_, err = svc.ListProjectMembers(ctx, outsiderID, project.ID)
	assert.ErrorIs(t, err, domain.ErrProjectForbidden)
}

func TestRemoveProjectMember(t *testing.T) {
	ctx := context.Background()
	repo := teamRepo()
	svc, project := newProject(t, repo)
	_, err := svc.AddProjectMember(ctx, ownerID, project.ID, "member@example.com")
	require.NoError(t, err)

	assert.ErrorIs(t, svc.RemoveProjectMember(ctx, memberID, project.ID, memberID), domain.ErrWorkspaceForbidden)
	assert.ErrorIs(t, svc.RemoveProjectMember(ctx, ownerID, unknownProject, memberID), domain.ErrProjectForbidden)
	assert.ErrorIs(t, svc.RemoveProjectMember(ctx, adminID, project.ID, ownerID), domain.ErrMemberNotRemovable)
	assert.ErrorIs(t, svc.RemoveProjectMember(ctx, ownerID, project.ID, adminID), domain.ErrMemberNotRemovable)

	require.NoError(t, svc.RemoveProjectMember(ctx, adminID, project.ID, memberID))
	assert.Equal(t, domain.RoleMember, repo.Members[workspaceID+"/"+memberID], "the user stays in the workspace")
	access, err := svc.CheckProjectAccess(ctx, memberID, project.ID)
	require.NoError(t, err)
	assert.False(t, access.Allowed)

	assert.ErrorIs(t, svc.RemoveProjectMember(ctx, ownerID, project.ID, memberID), domain.ErrProjectMemberNotFound)
}

// failingAt makes the repository fail only for the given step, to cover the error paths in the
// middle of an operation.
type failingAt struct {
	*service.MockIdentityRepository
	step string
}

var errStep = errors.New("step failed")

func (f failingAt) GetMemberRole(ctx context.Context, workspaceID, userID string) (domain.WorkspaceRole, error) {
	if f.step == "role:"+userID {
		return domain.RoleNone, errStep
	}
	return f.MockIdentityRepository.GetMemberRole(ctx, workspaceID, userID)
}

func (f failingAt) AddMember(ctx context.Context, workspaceID, userID string, role domain.WorkspaceRole) (domain.WorkspaceRole, error) {
	if f.step == "addMember" {
		return domain.RoleNone, errStep
	}
	return f.MockIdentityRepository.AddMember(ctx, workspaceID, userID, role)
}

func (f failingAt) AddProjectMember(ctx context.Context, projectID, userID string) error {
	if f.step == "addProjectMember" {
		return errStep
	}
	return f.MockIdentityRepository.AddProjectMember(ctx, projectID, userID)
}

func (f failingAt) RemoveProjectMember(ctx context.Context, projectID, userID string) (bool, error) {
	if f.step == "remove" {
		return false, errStep
	}
	return f.MockIdentityRepository.RemoveProjectMember(ctx, projectID, userID)
}

func (f failingAt) IsProjectMember(ctx context.Context, projectID, userID string) (bool, error) {
	if f.step == "isMember" {
		return false, errStep
	}
	return f.MockIdentityRepository.IsProjectMember(ctx, projectID, userID)
}

func TestProjectMembers_FailuresInTheMiddleAreReturned(t *testing.T) {
	ctx := context.Background()
	base := teamRepo()
	_, project := newProject(t, base)
	base.ProjectMembers[project.ID+"/"+memberID] = true
	with := func(step string) service.IdentityService {
		return service.NewIdentityService(failingAt{MockIdentityRepository: base, step: step})
	}

	_, err := with("role:"+outsiderID).AddProjectMember(ctx, ownerID, project.ID, "outsider@example.com")
	assert.ErrorIs(t, err, errStep)
	_, err = with("addMember").AddProjectMember(ctx, ownerID, project.ID, "outsider@example.com")
	assert.ErrorIs(t, err, errStep)
	_, err = with("addProjectMember").AddProjectMember(ctx, ownerID, project.ID, "outsider@example.com")
	assert.ErrorIs(t, err, errStep)
	assert.ErrorIs(t, with("role:"+memberID).RemoveProjectMember(ctx, ownerID, project.ID, memberID), errStep)
	assert.ErrorIs(t, with("remove").RemoveProjectMember(ctx, ownerID, project.ID, memberID), errStep)
	_, err = with("isMember").CheckProjectAccess(ctx, memberID, project.ID)
	assert.ErrorIs(t, err, errStep)
}
