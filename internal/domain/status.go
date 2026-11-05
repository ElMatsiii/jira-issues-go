package domain

import (
	"strings"
)

// Status represents the Scrum workflow status of an Issue.
type Status int32

const (
	StatusUnspecified Status = 0
	StatusTodo        Status = 1
	StatusInProgress  Status = 2
	StatusInReview    Status = 3
	StatusDone        Status = 4
)

// Alias for explorer_m2_3 test compatibility
const (
	IssueStatusUnspecified = StatusUnspecified
	IssueStatusTodo        = StatusTodo
	IssueStatusInProgress  = StatusInProgress
	IssueStatusInReview    = StatusInReview
	IssueStatusDone        = StatusDone
)

// IsValid returns true if the status is a recognized Scrum lifecycle status.
func (s Status) IsValid() bool {
	return s >= StatusTodo && s <= StatusDone
}

func (s Status) String() string {
	switch s {
	case StatusTodo:
		return "TODO"
	case StatusInProgress:
		return "IN_PROGRESS"
	case StatusInReview:
		return "IN_REVIEW"
	case StatusDone:
		return "DONE"
	default:
		return "UNSPECIFIED"
	}
}

func ParseStatus(str string) Status {
	switch strings.ToUpper(strings.TrimSpace(str)) {
	case "TODO":
		return StatusTodo
	case "IN_PROGRESS":
		return StatusInProgress
	case "IN_REVIEW":
		return StatusInReview
	case "DONE":
		return StatusDone
	default:
		return StatusUnspecified
	}
}

// CanTransition determines whether a status transition is permitted in the Scrum lifecycle.
func CanTransition(from, to Status) bool {
	if !from.IsValid() || !to.IsValid() {
		return false
	}
	if from == to {
		return true
	}

	switch from {
	case StatusTodo:
		return to == StatusInProgress
	case StatusInProgress:
		return to == StatusInReview || to == StatusTodo
	case StatusInReview:
		return to == StatusDone || to == StatusInProgress
	case StatusDone:
		return to == StatusTodo
	default:
		return false
	}
}

// ValidateStatusTransition verifies if transitioning from current to next status is permitted.
func ValidateStatusTransition(current, next Status) error {
	if !current.IsValid() || !next.IsValid() {
		return ErrInvalidStatus
	}
	if !CanTransition(current, next) {
		return ErrInvalidStatusTransition
	}
	return nil
}
