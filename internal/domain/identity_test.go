package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"
)

func TestNewUser_NormalizesLikeTheBFF(t *testing.T) {
	user, err := domain.NewUser("  Ana Pérez ", "  Ana@Example.COM ", "0123456789")

	require.NoError(t, err)
	assert.Equal(t, "Ana Pérez", user.Name)
	assert.Equal(t, "ana@example.com", user.Email)
	assert.Empty(t, user.PasswordHash, "hashing is the service's job")
	assert.Equal(t, "Ana Pérez's workspace", domain.PersonalWorkspaceName(user.Name))
}

func TestNewUser_RejectsInvalidData(t *testing.T) {
	tests := map[string][3]string{
		"blank name":              {"   ", "ana@example.com", "0123456789"},
		"name too long":           {strings.Repeat("ñ", 121), "ana@example.com", "0123456789"},
		"email without domain":    {"Ana", "ana@example", "0123456789"},
		"email with spaces":       {"Ana", "ana maria@example.com", "0123456789"},
		"email too long":          {"Ana", strings.Repeat("a", 310) + "@example.com", "0123456789"},
		"password too short":      {"Ana", "ana@example.com", "012345678"},
		"password too long":       {"Ana", "ana@example.com", strings.Repeat("x", 201)},
		"emoji counts as 2 units": {"Ana", "ana@example.com", strings.Repeat("😀", 101)},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewUser(input[0], input[1], input[2])
			assert.ErrorIs(t, err, domain.ErrInvalidUserInput)
		})
	}

	_, err := domain.NewUser("Ana", "ana@example.com", strings.Repeat("😀", 5))
	assert.NoError(t, err, "5 emoji are 10 UTF-16 units, the same length the BFF measured")
}

func TestNewWorkspaceProject(t *testing.T) {
	project, err := domain.NewWorkspaceProject("ws-1", " web2 ", "  Portal web ", "  Clientes ")
	require.NoError(t, err)
	assert.Equal(t, "ws-1", project.WorkspaceID)
	assert.Equal(t, "WEB2", project.Key)
	assert.Equal(t, "Portal web", project.Name)
	assert.Equal(t, "Clientes", project.Description)

	for name, input := range map[string][2]string{
		"blank name":          {"WEB", " "},
		"name too long":       {"WEB", strings.Repeat("a", 256)},
		"key too short":       {"W", "Portal"},
		"key too long":        {"ABCDEFGHIJK", "Portal"},
		"key starts by digit": {"1WEB", "Portal"},
		"key with symbols":    {"WE-B", "Portal"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewWorkspaceProject("ws-1", input[0], input[1], "")
			assert.ErrorIs(t, err, domain.ErrInvalidProjectInput)
		})
	}
}

func TestWorkspaceRoles(t *testing.T) {
	assert.Equal(t, domain.RoleOwner, domain.ParseWorkspaceRole("OWNER"))
	assert.Equal(t, domain.RoleAdmin, domain.ParseWorkspaceRole("ADMIN"))
	assert.Equal(t, domain.RoleMember, domain.ParseWorkspaceRole("MEMBER"))
	assert.Equal(t, domain.RoleNone, domain.ParseWorkspaceRole("owner"))

	assert.True(t, domain.RoleOwner.CanManage())
	assert.True(t, domain.RoleAdmin.CanManage())
	assert.False(t, domain.RoleMember.CanManage())
	assert.False(t, domain.RoleNone.CanManage())
	assert.True(t, domain.RoleMember.IsMember())
	assert.False(t, domain.RoleNone.IsMember())

	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_OWNER, domain.RoleToProto(domain.RoleOwner))
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_ADMIN, domain.RoleToProto(domain.RoleAdmin))
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_MEMBER, domain.RoleToProto(domain.RoleMember))
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_UNSPECIFIED, domain.RoleToProto(domain.RoleNone))
}

func TestIdentityConverters(t *testing.T) {
	created := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

	user := domain.UserToProto(&domain.User{ID: "u1", Name: "Ana", Email: "ana@example.com", PasswordHash: "scrypt$a$b", CreatedAt: created})
	assert.Equal(t, &issuesv1.User{Id: "u1", Name: "Ana", Email: "ana@example.com", CreatedAt: created.UnixMilli()}, user)

	workspace := domain.WorkspaceToProto(&domain.Workspace{ID: "w1", Name: "Equipo", Role: domain.RoleAdmin, CreatedAt: created})
	assert.Equal(t, issuesv1.WorkspaceRole_WORKSPACE_ROLE_ADMIN, workspace.GetRole())
	assert.Equal(t, created.UnixMilli(), workspace.GetCreatedAt())

	project := domain.ProjectToProto(&domain.Project{ID: "p1", WorkspaceID: "w1", Key: "WEB", Name: "Portal", Description: "d", CreatedAt: created})
	assert.Equal(t, &issuesv1.Project{Id: "p1", WorkspaceId: "w1", Key: "WEB", Name: "Portal", Description: "d", CreatedAt: created.UnixMilli()}, project)

	assert.Nil(t, domain.UserToProto(nil))
	assert.Nil(t, domain.WorkspaceToProto(nil))
	assert.Nil(t, domain.ProjectToProto(nil))
}
