package domain

import "errors"

var (
	ErrNotFound                = errors.New("issue not found")
	ErrEmptyTitle              = errors.New("issue title cannot be empty")
	ErrTitleTooLong            = errors.New("issue title cannot exceed 255 characters")
	ErrEmptyProjectID          = errors.New("project ID cannot be empty")
	ErrInvalidStatus           = errors.New("invalid issue status")
	ErrInvalidPriority         = errors.New("invalid issue priority")
	ErrInvalidType             = errors.New("invalid issue type")
	ErrInvalidStatusTransition = errors.New("invalid issue status transition")
	ErrNegativeStoryPoints     = errors.New("story points cannot be negative")
	ErrProjectNotFound         = errors.New("project not found")
	ErrEmptyIssueIDs           = errors.New("issue IDs cannot be empty")
	ErrParentNotFound          = errors.New("parent epic not found")
	ErrInvalidParent           = errors.New("invalid parent epic")
	// Identity, workspaces and projects.
	ErrInvalidUserInput    = errors.New("invalid user data")
	ErrInvalidProjectInput = errors.New("invalid project data")
	ErrUserNotFound        = errors.New("user not found")
	ErrEmailTaken          = errors.New("an account with that email already exists")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrWorkspaceForbidden  = errors.New("not allowed in this workspace")
	ErrProjectKeyTaken     = errors.New("project key already in use")
	ErrProjectForbidden    = errors.New("no access to this project")
	// ErrMemberNotRemovable: the OWNER and ADMINs reach every project through their workspace role.
	ErrMemberNotRemovable    = errors.New("the OWNER and ADMINs reach every project through their workspace role and cannot be removed from one")
	ErrProjectMemberNotFound = errors.New("user is not a member of this project")
	// Work logs.
	ErrInvalidWorkLog    = errors.New("invalid work log")
	ErrIssueNotInProject = errors.New("issue not found in this project")
	// ErrVersionConflict means the issue changed after it was read (optimistic concurrency).
	ErrVersionConflict = errors.New("issue was modified concurrently, retry the operation")
)
