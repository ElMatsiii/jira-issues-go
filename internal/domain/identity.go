package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// WorkspaceRole is the role of a user inside a workspace. RoleNone means "not a member".
type WorkspaceRole string

const (
	RoleNone   WorkspaceRole = ""
	RoleOwner  WorkspaceRole = "OWNER"
	RoleAdmin  WorkspaceRole = "ADMIN"
	RoleMember WorkspaceRole = "MEMBER"
)

// ParseWorkspaceRole maps the value stored in workspace_members.role; unknown values are RoleNone.
func ParseWorkspaceRole(value string) WorkspaceRole {
	switch WorkspaceRole(value) {
	case RoleOwner, RoleAdmin, RoleMember:
		return WorkspaceRole(value)
	default:
		return RoleNone
	}
}

// IsMember reports whether the role grants read access to the workspace.
func (r WorkspaceRole) IsMember() bool {
	return r == RoleOwner || r == RoleAdmin || r == RoleMember
}

// CanManage reports whether the role may create projects and add members (OWNER or ADMIN).
func (r WorkspaceRole) CanManage() bool {
	return r == RoleOwner || r == RoleAdmin
}

// User is an account. PasswordHash never leaves MS1.
type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// Workspace groups members and projects. Role is the role of the user it was listed for.
type Workspace struct {
	ID        string
	Name      string
	Role      WorkspaceRole
	CreatedAt time.Time
}

// Limits shared with the tables (app_users, projects) and with the rules the BFF applied before.
const (
	MaxUserNameLength    = 120
	MaxEmailLength       = 320
	MinPasswordLength    = 10
	MaxPasswordLength    = 200
	MaxProjectNameLength = 255
)

var (
	emailPattern      = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	projectKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)
)

// NormalizeEmail is how emails are stored and looked up (the BFF stored them the same way).
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// PersonalWorkspaceName is the name of the workspace created on registration (same text the BFF used).
func PersonalWorkspaceName(userName string) string {
	return userName + "'s workspace"
}

// NewUser validates and normalizes a registration. The password is only checked here; hashing is
// done by the caller so the domain does not depend on the crypto parameters.
func NewUser(name, email, password string) (*User, error) {
	normalizedName := strings.TrimSpace(name)
	normalizedEmail := NormalizeEmail(email)
	if normalizedName == "" || utf8.RuneCountInString(normalizedName) > MaxUserNameLength {
		return nil, fmt.Errorf("%w: name is required and must have at most %d characters", ErrInvalidUserInput, MaxUserNameLength)
	}
	if len(normalizedEmail) > MaxEmailLength || !emailPattern.MatchString(normalizedEmail) {
		return nil, fmt.Errorf("%w: email is not valid", ErrInvalidUserInput)
	}
	// Counted in UTF-16 units, exactly like the BFF's password.length check.
	if length := len(utf16.Encode([]rune(password))); length < MinPasswordLength || length > MaxPasswordLength {
		return nil, fmt.Errorf("%w: password must have between %d and %d characters", ErrInvalidUserInput, MinPasswordLength, MaxPasswordLength)
	}
	return &User{Name: normalizedName, Email: normalizedEmail}, nil
}

// NewWorkspaceProject validates a project created inside a workspace: key of 2 to 10 uppercase
// alphanumerics starting with a letter, and a name of at most 255 characters.
func NewWorkspaceProject(workspaceID, key, name, description string) (*Project, error) {
	normalizedKey := strings.ToUpper(strings.TrimSpace(key))
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" || utf8.RuneCountInString(normalizedName) > MaxProjectNameLength {
		return nil, fmt.Errorf("%w: name is required and must have at most %d characters", ErrInvalidProjectInput, MaxProjectNameLength)
	}
	if !projectKeyPattern.MatchString(normalizedKey) {
		return nil, fmt.Errorf("%w: key must have 2 to 10 letters or digits and start with a letter", ErrInvalidProjectInput)
	}
	return &Project{
		WorkspaceID: workspaceID,
		Key:         normalizedKey,
		Name:        normalizedName,
		Description: strings.TrimSpace(description),
	}, nil
}
