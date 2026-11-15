package grpc_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	grpchandler "jira-issues-go/internal/grpc"
	"jira-issues-go/internal/service"
)

type identityFixture struct {
	identity issuesv1.IdentityServiceClient
	projects issuesv1.ProjectServiceClient
	repo     *service.MockIdentityRepository
}

func setupIdentityFixture(t *testing.T) *identityFixture {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	repo := service.NewMockIdentityRepository()
	svc := service.NewIdentityService(repo)

	server := grpc.NewServer()
	issuesv1.RegisterIdentityServiceServer(server, grpchandler.NewIdentityServer(svc))
	issuesv1.RegisterProjectServiceServer(server, grpchandler.NewProjectServer(svc))
	go func() { _ = server.Serve(lis) }()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
		server.Stop()
		_ = lis.Close()
	})
	return &identityFixture{
		identity: issuesv1.NewIdentityServiceClient(conn),
		projects: issuesv1.NewProjectServiceClient(conn),
		repo:     repo,
	}
}

func (f *identityFixture) register(t *testing.T, name, email string) *issuesv1.RegisterUserResponse {
	t.Helper()
	resp, err := f.identity.RegisterUser(context.Background(), &issuesv1.RegisterUserRequest{Name: name, Email: email, Password: "contraseña-larga"})
	require.NoError(t, err)
	return resp
}

func TestIdentityGRPC_RegisterAuthenticateAndGetUser(t *testing.T) {
	f := setupIdentityFixture(t)
	ctx := context.Background()

	registered := f.register(t, "Ana", "Ana@Example.com")
	assert.Equal(t, "ana@example.com", registered.GetUser().GetEmail())
	assert.NotZero(t, registered.GetUser().GetCreatedAt())
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_OWNER, registered.GetWorkspace().GetRole())

	auth, err := f.identity.AuthenticateUser(ctx, &issuesv1.AuthenticateUserRequest{Email: "ana@example.com", Password: "contraseña-larga"})
	require.NoError(t, err)
	assert.Equal(t, registered.GetUser().GetId(), auth.GetUser().GetId())

	got, err := f.identity.GetUser(ctx, &issuesv1.GetUserRequest{Id: registered.GetUser().GetId()})
	require.NoError(t, err)
	assert.Equal(t, "Ana", got.GetUser().GetName())

	workspaces, err := f.identity.ListUserWorkspaces(ctx, &issuesv1.ListUserWorkspacesRequest{UserId: registered.GetUser().GetId()})
	require.NoError(t, err)
	require.Len(t, workspaces.GetWorkspaces(), 1)
	assert.Equal(t, "Ana's workspace", workspaces.GetWorkspaces()[0].GetName())
}

func TestIdentityGRPC_ErrorCodes(t *testing.T) {
	f := setupIdentityFixture(t)
	ctx := context.Background()
	f.register(t, "Ana", "ana@example.com")

	_, err := f.identity.RegisterUser(ctx, &issuesv1.RegisterUserRequest{Name: "Ana", Email: "ana@example.com", Password: "contraseña-larga"})
	requireCode(t, err, codes.AlreadyExists, "already exists")

	_, err = f.identity.RegisterUser(ctx, &issuesv1.RegisterUserRequest{Name: "Ana", Email: "ana@example.com", Password: "corta"})
	requireCode(t, err, codes.InvalidArgument, "password")

	_, err = f.identity.AuthenticateUser(ctx, &issuesv1.AuthenticateUserRequest{Email: "ana@example.com", Password: "incorrecta-123"})
	requireCode(t, err, codes.Unauthenticated, "invalid email or password")

	_, err = f.identity.GetUser(ctx, &issuesv1.GetUserRequest{Id: "11111111-1111-4111-8111-111111111111"})
	requireCode(t, err, codes.NotFound, "user not found")

	_, err = f.identity.GetUser(ctx, &issuesv1.GetUserRequest{Id: "ana"})
	requireCode(t, err, codes.InvalidArgument, "id must be a valid UUID")

	_, err = f.identity.ListUserWorkspaces(ctx, &issuesv1.ListUserWorkspacesRequest{})
	requireCode(t, err, codes.InvalidArgument, "user_id is required")

	f.repo.Err = errors.New("connection reset")
	_, err = f.identity.ListUserWorkspaces(ctx, &issuesv1.ListUserWorkspacesRequest{UserId: "11111111-1111-4111-8111-111111111111"})
	requireCode(t, err, codes.Internal, "failed to list workspaces")
}

func TestIdentityGRPC_MembersAndRoles(t *testing.T) {
	f := setupIdentityFixture(t)
	ctx := context.Background()
	owner := f.register(t, "Owner", "owner@example.com")
	guest := f.register(t, "Guest", "guest@example.com")
	workspaceID := owner.GetWorkspace().GetId()

	added, err := f.identity.AddWorkspaceMember(ctx, &issuesv1.AddWorkspaceMemberRequest{
		RequesterId: owner.GetUser().GetId(), WorkspaceId: workspaceID, MemberEmail: "GUEST@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, guest.GetUser().GetId(), added.GetMember().GetId())
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_MEMBER, added.GetRole())

	role, err := f.identity.GetMemberRole(ctx, &issuesv1.GetMemberRoleRequest{WorkspaceId: workspaceID, UserId: guest.GetUser().GetId()})
	require.NoError(t, err)
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_MEMBER, role.GetRole())

	// A MEMBER cannot add people.
	_, err = f.identity.AddWorkspaceMember(ctx, &issuesv1.AddWorkspaceMemberRequest{
		RequesterId: guest.GetUser().GetId(), WorkspaceId: workspaceID, MemberEmail: "owner@example.com",
	})
	requireCode(t, err, codes.PermissionDenied, "not allowed")

	_, err = f.identity.AddWorkspaceMember(ctx, &issuesv1.AddWorkspaceMemberRequest{
		RequesterId: owner.GetUser().GetId(), WorkspaceId: workspaceID, MemberEmail: "nadie@example.com",
	})
	requireCode(t, err, codes.NotFound, "user not found")

	_, err = f.identity.AddWorkspaceMember(ctx, &issuesv1.AddWorkspaceMemberRequest{
		RequesterId: owner.GetUser().GetId(), WorkspaceId: workspaceID, MemberEmail: " ",
	})
	requireCode(t, err, codes.InvalidArgument, "member_email is required")

	_, err = f.identity.AddWorkspaceMember(ctx, &issuesv1.AddWorkspaceMemberRequest{WorkspaceId: workspaceID, MemberEmail: "x@example.com"})
	requireCode(t, err, codes.InvalidArgument, "requester_id is required")

	_, err = f.identity.AddWorkspaceMember(ctx, &issuesv1.AddWorkspaceMemberRequest{RequesterId: owner.GetUser().GetId(), MemberEmail: "x@example.com"})
	requireCode(t, err, codes.InvalidArgument, "workspace_id is required")

	// Not a member: UNSPECIFIED, not an error.
	stranger := f.register(t, "Stranger", "stranger@example.com")
	role, err = f.identity.GetMemberRole(ctx, &issuesv1.GetMemberRoleRequest{WorkspaceId: workspaceID, UserId: stranger.GetUser().GetId()})
	require.NoError(t, err)
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_UNSPECIFIED, role.GetRole())

	_, err = f.identity.GetMemberRole(ctx, &issuesv1.GetMemberRoleRequest{WorkspaceId: "x", UserId: stranger.GetUser().GetId()})
	requireCode(t, err, codes.InvalidArgument, "workspace_id must be a valid UUID")
	_, err = f.identity.GetMemberRole(ctx, &issuesv1.GetMemberRoleRequest{WorkspaceId: workspaceID})
	requireCode(t, err, codes.InvalidArgument, "user_id is required")
}

func TestProjectGRPC_CreateListAndCheckAccess(t *testing.T) {
	f := setupIdentityFixture(t)
	ctx := context.Background()
	owner := f.register(t, "Owner", "owner@example.com")
	member := f.register(t, "Member", "member@example.com")
	outsider := f.register(t, "Outsider", "outsider@example.com")
	workspaceID := owner.GetWorkspace().GetId()
	_, err := f.identity.AddWorkspaceMember(ctx, &issuesv1.AddWorkspaceMemberRequest{
		RequesterId: owner.GetUser().GetId(), WorkspaceId: workspaceID, MemberEmail: "member@example.com",
	})
	require.NoError(t, err)

	created, err := f.projects.CreateProject(ctx, &issuesv1.CreateProjectRequest{
		RequesterId: owner.GetUser().GetId(), WorkspaceId: workspaceID, Key: "web", Name: "Portal", Description: "Clientes",
	})
	require.NoError(t, err)
	assert.Equal(t, "WEB", created.GetProject().GetKey())
	assert.Equal(t, workspaceID, created.GetProject().GetWorkspaceId())

	_, err = f.projects.CreateProject(ctx, &issuesv1.CreateProjectRequest{
		RequesterId: member.GetUser().GetId(), WorkspaceId: workspaceID, Key: "APP", Name: "App",
	})
	requireCode(t, err, codes.PermissionDenied, "not allowed")

	_, err = f.projects.CreateProject(ctx, &issuesv1.CreateProjectRequest{
		RequesterId: owner.GetUser().GetId(), WorkspaceId: workspaceID, Key: "WEB", Name: "Otro",
	})
	requireCode(t, err, codes.AlreadyExists, "project key already in use")

	_, err = f.projects.CreateProject(ctx, &issuesv1.CreateProjectRequest{
		RequesterId: owner.GetUser().GetId(), WorkspaceId: workspaceID, Key: "W", Name: "Corto",
	})
	requireCode(t, err, codes.InvalidArgument, "key must have 2 to 10")

	_, err = f.projects.CreateProject(ctx, &issuesv1.CreateProjectRequest{WorkspaceId: workspaceID, Key: "APP", Name: "App"})
	requireCode(t, err, codes.InvalidArgument, "requester_id is required")
	_, err = f.projects.CreateProject(ctx, &issuesv1.CreateProjectRequest{RequesterId: owner.GetUser().GetId(), Key: "APP", Name: "App"})
	requireCode(t, err, codes.InvalidArgument, "workspace_id is required")

	// Being in the workspace is not enough: a MEMBER sees only the projects it was added to.
	listed, err := f.projects.ListProjects(ctx, &issuesv1.ListProjectsRequest{RequesterId: member.GetUser().GetId(), WorkspaceId: workspaceID})
	require.NoError(t, err)
	assert.Empty(t, listed.GetProjects())
	_, err = f.projects.AddProjectMember(ctx, &issuesv1.AddProjectMemberRequest{
		RequesterId: owner.GetUser().GetId(), ProjectId: created.GetProject().GetId(), MemberEmail: "member@example.com",
	})
	require.NoError(t, err)

	listed, err = f.projects.ListProjects(ctx, &issuesv1.ListProjectsRequest{RequesterId: member.GetUser().GetId(), WorkspaceId: workspaceID})
	require.NoError(t, err)
	require.Len(t, listed.GetProjects(), 1)
	assert.Equal(t, created.GetProject().GetId(), listed.GetProjects()[0].GetId())

	_, err = f.projects.ListProjects(ctx, &issuesv1.ListProjectsRequest{RequesterId: outsider.GetUser().GetId(), WorkspaceId: workspaceID})
	requireCode(t, err, codes.PermissionDenied, "not allowed")
	_, err = f.projects.ListProjects(ctx, &issuesv1.ListProjectsRequest{WorkspaceId: workspaceID})
	requireCode(t, err, codes.InvalidArgument, "requester_id is required")
	_, err = f.projects.ListProjects(ctx, &issuesv1.ListProjectsRequest{RequesterId: owner.GetUser().GetId()})
	requireCode(t, err, codes.InvalidArgument, "workspace_id is required")

	access, err := f.projects.CheckProjectAccess(ctx, &issuesv1.CheckProjectAccessRequest{UserId: member.GetUser().GetId(), ProjectId: created.GetProject().GetId()})
	require.NoError(t, err)
	assert.True(t, access.GetAllowed())
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_MEMBER, access.GetRole())
	assert.Equal(t, workspaceID, access.GetWorkspaceId())

	denied, err := f.projects.CheckProjectAccess(ctx, &issuesv1.CheckProjectAccessRequest{UserId: outsider.GetUser().GetId(), ProjectId: created.GetProject().GetId()})
	require.NoError(t, err)
	assert.False(t, denied.GetAllowed())
	assert.Empty(t, denied.GetWorkspaceId())

	_, err = f.projects.CheckProjectAccess(ctx, &issuesv1.CheckProjectAccessRequest{UserId: outsider.GetUser().GetId(), ProjectId: "PROJ-1"})
	requireCode(t, err, codes.InvalidArgument, "project_id must be a valid UUID")
	_, err = f.projects.CheckProjectAccess(ctx, &issuesv1.CheckProjectAccessRequest{ProjectId: created.GetProject().GetId()})
	requireCode(t, err, codes.InvalidArgument, "user_id is required")

	f.repo.Err = errors.New("connection reset")
	_, err = f.projects.CheckProjectAccess(ctx, &issuesv1.CheckProjectAccessRequest{UserId: member.GetUser().GetId(), ProjectId: created.GetProject().GetId()})
	requireCode(t, err, codes.Internal, "failed to check project access")
}

func TestProjectGRPC_ProjectMembers(t *testing.T) {
	f := setupIdentityFixture(t)
	ctx := context.Background()
	owner := f.register(t, "Owner", "owner@example.com")
	guest := f.register(t, "Guest", "guest@example.com")
	outsider := f.register(t, "Outsider", "outsider@example.com")
	ownerID := owner.GetUser().GetId()
	created, err := f.projects.CreateProject(ctx, &issuesv1.CreateProjectRequest{
		RequesterId: ownerID, WorkspaceId: owner.GetWorkspace().GetId(), Key: "WEB", Name: "Portal",
	})
	require.NoError(t, err)
	projectID := created.GetProject().GetId()

	added, err := f.projects.AddProjectMember(ctx, &issuesv1.AddProjectMemberRequest{RequesterId: ownerID, ProjectId: projectID, MemberEmail: "Guest@example.com"})
	require.NoError(t, err)
	assert.Equal(t, guest.GetUser().GetId(), added.GetMember().GetUser().GetId())
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_MEMBER, added.GetMember().GetRole())
	assert.True(t, added.GetMember().GetInvited())

	members, err := f.projects.ListProjectMembers(ctx, &issuesv1.ListProjectMembersRequest{RequesterId: guest.GetUser().GetId(), ProjectId: projectID})
	require.NoError(t, err)
	require.Len(t, members.GetMembers(), 2)
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_OWNER, members.GetMembers()[0].GetRole())
	assert.False(t, members.GetMembers()[0].GetInvited())
	assert.Equal(t, "Guest", members.GetMembers()[1].GetUser().GetName())

	// Error codes.
	_, err = f.projects.AddProjectMember(ctx, &issuesv1.AddProjectMemberRequest{RequesterId: guest.GetUser().GetId(), ProjectId: projectID, MemberEmail: "outsider@example.com"})
	requireCode(t, err, codes.PermissionDenied, "not allowed in this workspace")
	_, err = f.projects.AddProjectMember(ctx, &issuesv1.AddProjectMemberRequest{RequesterId: ownerID, ProjectId: projectID, MemberEmail: "nadie@example.com"})
	requireCode(t, err, codes.NotFound, "user not found")
	_, err = f.projects.AddProjectMember(ctx, &issuesv1.AddProjectMemberRequest{RequesterId: ownerID, ProjectId: projectID, MemberEmail: " "})
	requireCode(t, err, codes.InvalidArgument, "member_email is required")
	_, err = f.projects.AddProjectMember(ctx, &issuesv1.AddProjectMemberRequest{RequesterId: ownerID, ProjectId: "WEB", MemberEmail: "x@example.com"})
	requireCode(t, err, codes.InvalidArgument, "project_id must be a valid UUID")
	_, err = f.projects.AddProjectMember(ctx, &issuesv1.AddProjectMemberRequest{ProjectId: projectID, MemberEmail: "x@example.com"})
	requireCode(t, err, codes.InvalidArgument, "requester_id is required")
	_, err = f.projects.ListProjectMembers(ctx, &issuesv1.ListProjectMembersRequest{RequesterId: outsider.GetUser().GetId(), ProjectId: projectID})
	requireCode(t, err, codes.PermissionDenied, "no access to this project")
	_, err = f.projects.ListProjectMembers(ctx, &issuesv1.ListProjectMembersRequest{ProjectId: projectID})
	requireCode(t, err, codes.InvalidArgument, "requester_id is required")
	_, err = f.projects.ListProjectMembers(ctx, &issuesv1.ListProjectMembersRequest{RequesterId: ownerID})
	requireCode(t, err, codes.InvalidArgument, "project_id is required")

	remove := func(userID string) error {
		_, err := f.projects.RemoveProjectMember(ctx, &issuesv1.RemoveProjectMemberRequest{RequesterId: ownerID, ProjectId: projectID, UserId: userID})
		return err
	}
	requireCode(t, remove(ownerID), codes.FailedPrecondition, "cannot be removed")
	requireCode(t, remove("guest"), codes.InvalidArgument, "user_id must be a valid UUID")
	require.NoError(t, remove(guest.GetUser().GetId()))
	requireCode(t, remove(guest.GetUser().GetId()), codes.NotFound, "not a member of this project")
	_, err = f.projects.RemoveProjectMember(ctx, &issuesv1.RemoveProjectMemberRequest{ProjectId: projectID, UserId: ownerID})
	requireCode(t, err, codes.InvalidArgument, "requester_id is required")
	_, err = f.projects.RemoveProjectMember(ctx, &issuesv1.RemoveProjectMemberRequest{RequesterId: ownerID, UserId: ownerID})
	requireCode(t, err, codes.InvalidArgument, "project_id is required")

	f.repo.Err = errors.New("connection reset")
	_, err = f.projects.ListProjectMembers(ctx, &issuesv1.ListProjectMembersRequest{RequesterId: ownerID, ProjectId: projectID})
	requireCode(t, err, codes.Internal, "failed to list project members")
}
