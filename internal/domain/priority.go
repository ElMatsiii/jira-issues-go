package domain

import (
	"strings"
)

// Priority represents the urgency and impact of an Issue.
type Priority int32

const (
	PriorityUnspecified Priority = 0
	PriorityLow         Priority = 1
	PriorityMedium      Priority = 2
	PriorityHigh        Priority = 3
	PriorityCritical    Priority = 4
)

// Aliases for explorer_m2_3 test compatibility
const (
	IssuePriorityUnspecified = PriorityUnspecified
	IssuePriorityLow         = PriorityLow
	IssuePriorityMedium      = PriorityMedium
	IssuePriorityHigh        = PriorityHigh
	IssuePriorityCritical    = PriorityCritical
)

// IsValid returns true if the priority is a recognized urgency level.
func (p Priority) IsValid() bool {
	return p >= PriorityLow && p <= PriorityCritical
}

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "LOW"
	case PriorityMedium:
		return "MEDIUM"
	case PriorityHigh:
		return "HIGH"
	case PriorityCritical:
		return "CRITICAL"
	default:
		return "UNSPECIFIED"
	}
}

func ParsePriority(str string) Priority {
	switch strings.ToUpper(strings.TrimSpace(str)) {
	case "LOW":
		return PriorityLow
	case "MEDIUM":
		return PriorityMedium
	case "HIGH":
		return PriorityHigh
	case "CRITICAL":
		return PriorityCritical
	default:
		return PriorityUnspecified
	}
}
