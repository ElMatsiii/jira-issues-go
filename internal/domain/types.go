package domain

import (
	"strings"
)

// Type represents the Agile categorization of an Issue.
type Type int32

const (
	TypeUnspecified Type = 0
	TypeStory       Type = 1
	TypeTask        Type = 2
	TypeBug         Type = 3
	TypeEpic        Type = 4
)

// Aliases for explorer_m2_3 test compatibility
const (
	IssueTypeUnspecified = TypeUnspecified
	IssueTypeStory       = TypeStory
	IssueTypeTask        = TypeTask
	IssueTypeBug         = TypeBug
	IssueTypeEpic        = TypeEpic
)

// IsValid returns true if the type is a recognized Agile work item type.
func (t Type) IsValid() bool {
	return t >= TypeStory && t <= TypeEpic
}

func (t Type) String() string {
	switch t {
	case TypeStory:
		return "STORY"
	case TypeTask:
		return "TASK"
	case TypeBug:
		return "BUG"
	case TypeEpic:
		return "EPIC"
	default:
		return "UNSPECIFIED"
	}
}

func ParseType(str string) Type {
	switch strings.ToUpper(strings.TrimSpace(str)) {
	case "STORY":
		return TypeStory
	case "TASK":
		return TypeTask
	case "BUG":
		return TypeBug
	case "EPIC":
		return TypeEpic
	default:
		return TypeUnspecified
	}
}

// Aliases for type aliases to support diverse conventions
type IssueStatus = Status
type IssuePriority = Priority
type IssueType = Type
