package domain

import (
	issuesv1 "jira-contracts/gen/go/issues/v1"
)

// RoleToProto maps a workspace role; RoleNone is WORKSPACE_ROLE_UNSPECIFIED ("not a member").
func RoleToProto(role WorkspaceRole) issuesv1.WorkspaceRole {
	switch role {
	case RoleOwner:
		return issuesv1.WorkspaceRole_WORKSPACE_ROLE_OWNER
	case RoleAdmin:
		return issuesv1.WorkspaceRole_WORKSPACE_ROLE_ADMIN
	case RoleMember:
		return issuesv1.WorkspaceRole_WORKSPACE_ROLE_MEMBER
	default:
		return issuesv1.WorkspaceRole_WORKSPACE_ROLE_UNSPECIFIED
	}
}

// UserToProto never includes the password hash.
func UserToProto(user *User) *issuesv1.User {
	if user == nil {
		return nil
	}
	return &issuesv1.User{
		Id:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt.UnixMilli(),
	}
}

func WorkspaceToProto(workspace *Workspace) *issuesv1.Workspace {
	if workspace == nil {
		return nil
	}
	return &issuesv1.Workspace{
		Id:        workspace.ID,
		Name:      workspace.Name,
		Role:      RoleToProto(workspace.Role),
		CreatedAt: workspace.CreatedAt.UnixMilli(),
	}
}

func ProjectMemberToProto(member *ProjectMember) *issuesv1.ProjectMember {
	if member == nil {
		return nil
	}
	return &issuesv1.ProjectMember{
		User:    UserToProto(member.User),
		Role:    RoleToProto(member.Role),
		Invited: member.Invited,
	}
}

func ProjectToProto(project *Project) *issuesv1.Project {
	if project == nil {
		return nil
	}
	return &issuesv1.Project{
		Id:          project.ID,
		WorkspaceId: project.WorkspaceID,
		Key:         project.Key,
		Name:        project.Name,
		Description: project.Description,
		CreatedAt:   project.CreatedAt.UnixMilli(),
	}
}
