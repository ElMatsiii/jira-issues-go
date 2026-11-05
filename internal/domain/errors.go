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
)
