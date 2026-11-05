package domain_test

import (
	"strings"
	"testing"
	"time"

	"jira-issues-go/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIssue_Success(t *testing.T) {
	issue, err := domain.NewIssue(
		"proj-uuid-1",
		"PROJ-101",
		"Implement gRPC authentication",
		"Add interceptor for token validation",
		domain.IssuePriorityHigh,
		domain.IssueTypeTask,
		5,
		"Acceptance: tokens validated on :50051",
	)

	require.NoError(t, err)
	assert.NotEmpty(t, issue.ID)
	assert.Equal(t, "proj-uuid-1", issue.ProjectID)
	assert.Equal(t, "PROJ-101", issue.Key)
	assert.Equal(t, "Implement gRPC authentication", issue.Title)
	assert.Equal(t, domain.IssueStatusTodo, issue.Status)
	assert.Equal(t, domain.IssuePriorityHigh, issue.Priority)
	assert.Equal(t, domain.IssueTypeTask, issue.Type)
	assert.Equal(t, int32(5), issue.StoryPoints)
	assert.Nil(t, issue.SprintID)
	assert.False(t, issue.CreatedAt.IsZero())
	assert.False(t, issue.UpdatedAt.IsZero())
}

func TestNewIssue_Error(t *testing.T) {
	_, err := domain.NewIssue("proj-1", "PROJ-1", "", "Desc", domain.IssuePriorityHigh, domain.IssueTypeStory, 1, "")
	assert.ErrorIs(t, err, domain.ErrEmptyTitle)
}

func TestIssue_Validate(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(i *domain.Issue)
		expectedErr error
	}{
		{
			name:        "Valid issue passes",
			modify:      func(i *domain.Issue) {},
			expectedErr: nil,
		},
		{
			name: "Zero story points is valid",
			modify: func(i *domain.Issue) {
				i.StoryPoints = 0
			},
			expectedErr: nil,
		},
		{
			name: "Max length title (255 chars) is valid",
			modify: func(i *domain.Issue) {
				i.Title = strings.Repeat("A", 255)
			},
			expectedErr: nil,
		},
		{
			name: "Empty title returns error",
			modify: func(i *domain.Issue) {
				i.Title = ""
			},
			expectedErr: domain.ErrEmptyTitle,
		},
		{
			name: "Whitespace-only title returns error",
			modify: func(i *domain.Issue) {
				i.Title = "   "
			},
			expectedErr: domain.ErrEmptyTitle,
		},
		{
			name: "Title exceeding 255 chars returns error",
			modify: func(i *domain.Issue) {
				i.Title = strings.Repeat("A", 256)
			},
			expectedErr: domain.ErrTitleTooLong,
		},
		{
			name: "Empty ProjectID returns error",
			modify: func(i *domain.Issue) {
				i.ProjectID = ""
			},
			expectedErr: domain.ErrEmptyProjectID,
		},
		{
			name: "Negative story points returns error",
			modify: func(i *domain.Issue) {
				i.StoryPoints = -1
			},
			expectedErr: domain.ErrNegativeStoryPoints,
		},
		{
			name: "Invalid status enum returns error",
			modify: func(i *domain.Issue) {
				i.Status = domain.IssueStatus(99)
			},
			expectedErr: domain.ErrInvalidStatus,
		},
		{
			name: "Negative status enum returns error",
			modify: func(i *domain.Issue) {
				i.Status = domain.IssueStatus(-1)
			},
			expectedErr: domain.ErrInvalidStatus,
		},
		{
			name: "Invalid priority enum returns error",
			modify: func(i *domain.Issue) {
				i.Priority = domain.IssuePriority(99)
			},
			expectedErr: domain.ErrInvalidPriority,
		},
		{
			name: "Negative priority enum returns error",
			modify: func(i *domain.Issue) {
				i.Priority = domain.IssuePriority(-1)
			},
			expectedErr: domain.ErrInvalidPriority,
		},
		{
			name: "Invalid type enum returns error",
			modify: func(i *domain.Issue) {
				i.Type = domain.IssueType(99)
			},
			expectedErr: domain.ErrInvalidType,
		},
		{
			name: "Negative type enum returns error",
			modify: func(i *domain.Issue) {
				i.Type = domain.IssueType(-1)
			},
			expectedErr: domain.ErrInvalidType,
		},
		{
			name: "Status unspecified defaults to Todo",
			modify: func(i *domain.Issue) {
				i.Status = domain.StatusUnspecified
			},
			expectedErr: nil,
		},
		{
			name: "Priority unspecified defaults to Medium",
			modify: func(i *domain.Issue) {
				i.Priority = domain.PriorityUnspecified
			},
			expectedErr: nil,
		},
		{
			name: "Type unspecified defaults to Story",
			modify: func(i *domain.Issue) {
				i.Type = domain.TypeUnspecified
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issue, err := domain.NewIssue(
				"proj-uuid-1", "PROJ-1", "Valid Title", "Desc",
				domain.IssuePriorityMedium, domain.IssueTypeStory, 3, "",
			)
			require.NoError(t, err)

			tc.modify(issue)
			err = issue.Validate()
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIssue_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from     domain.IssueStatus
		to       domain.IssueStatus
		expected bool
	}{
		// Same status
		{domain.IssueStatusTodo, domain.IssueStatusTodo, true},
		{domain.IssueStatusInProgress, domain.IssueStatusInProgress, true},

		// TODO transitions
		{domain.IssueStatusTodo, domain.IssueStatusInProgress, true},
		{domain.IssueStatusTodo, domain.IssueStatusInReview, false},
		{domain.IssueStatusTodo, domain.IssueStatusDone, false},

		// IN_PROGRESS transitions
		{domain.IssueStatusInProgress, domain.IssueStatusInReview, true},
		{domain.IssueStatusInProgress, domain.IssueStatusTodo, true},
		{domain.IssueStatusInProgress, domain.IssueStatusDone, false},

		// IN_REVIEW transitions
		{domain.IssueStatusInReview, domain.IssueStatusDone, true},
		{domain.IssueStatusInReview, domain.IssueStatusInProgress, true},
		{domain.IssueStatusInReview, domain.IssueStatusTodo, false},

		// DONE transitions
		{domain.IssueStatusDone, domain.IssueStatusTodo, true}, // Reopened
		{domain.IssueStatusDone, domain.IssueStatusInProgress, false},
		{domain.IssueStatusDone, domain.IssueStatusInReview, false},

		// Invalid zero and out-of-range statuses
		{domain.IssueStatusTodo, domain.IssueStatus(0), false},
		{domain.IssueStatus(0), domain.IssueStatusTodo, false},
		{domain.IssueStatus(99), domain.IssueStatusTodo, false},
		{domain.IssueStatus(0), domain.IssueStatus(0), false},
		{domain.IssueStatus(-1), domain.IssueStatus(-1), false},
		{domain.IssueStatus(99), domain.IssueStatus(99), false},
		{domain.IssueStatusTodo, domain.IssueStatus(-1), false},
		{domain.IssueStatusTodo, domain.IssueStatus(99), false},
		{domain.IssueStatus(-1), domain.IssueStatusTodo, false},
		{domain.IssueStatus(5), domain.IssueStatus(5), false},
		{domain.IssueStatus(5), domain.IssueStatusDone, false},
	}

	for _, tc := range tests {
		t.Run(tc.from.String()+"->"+tc.to.String(), func(t *testing.T) {
			allowed := domain.CanTransition(tc.from, tc.to)
			assert.Equal(t, tc.expected, allowed)
		})
	}
}

func TestIssue_TransitionTo(t *testing.T) {
	issue, err := domain.NewIssue(
		"proj-1", "PROJ-1", "Title", "Desc",
		domain.IssuePriorityLow, domain.IssueTypeTask, 1, "",
	)
	require.NoError(t, err)
	initialUpdated := issue.UpdatedAt

	// Valid transition
	time.Sleep(2 * time.Millisecond)
	err = issue.TransitionTo(domain.IssueStatusInProgress)
	assert.NoError(t, err)
	assert.Equal(t, domain.IssueStatusInProgress, issue.Status)
	assert.True(t, issue.UpdatedAt.After(initialUpdated))
	assert.Equal(t, 2, issue.Version)

	// Invalid transition
	err = issue.TransitionTo(domain.IssueStatusDone)
	assert.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
	assert.Equal(t, domain.IssueStatusInProgress, issue.Status) // preserved

	// Out of bounds status
	err = issue.TransitionTo(domain.IssueStatus(99))
	assert.ErrorIs(t, err, domain.ErrInvalidStatus)

	// Out of bounds status transitions directly
	assert.ErrorIs(t, domain.ValidateStatusTransition(domain.IssueStatus(-1), domain.IssueStatus(-1)), domain.ErrInvalidStatus)
	assert.ErrorIs(t, domain.ValidateStatusTransition(domain.IssueStatus(99), domain.IssueStatus(99)), domain.ErrInvalidStatus)
	assert.ErrorIs(t, domain.ValidateStatusTransition(domain.IssueStatus(-1), domain.IssueStatusTodo), domain.ErrInvalidStatus)
	assert.ErrorIs(t, domain.ValidateStatusTransition(domain.IssueStatus(0), domain.IssueStatusTodo), domain.ErrInvalidStatus)
	assert.ErrorIs(t, domain.ValidateStatusTransition(domain.IssueStatusTodo, domain.IssueStatus(0)), domain.ErrInvalidStatus)
}

func TestProject_NewProject(t *testing.T) {
	p, err := domain.NewProject("PROJ", "Platform", "Project description")
	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "PROJ", p.Key)
	assert.Equal(t, "Platform", p.Name)
	assert.Equal(t, 0, p.IssueCounter)

	// Empty key
	_, err = domain.NewProject("", "Name", "Desc")
	assert.ErrorIs(t, err, domain.ErrEmptyTitle)

	// Empty name
	_, err = domain.NewProject("KEY", "   ", "Desc")
	assert.ErrorIs(t, err, domain.ErrEmptyTitle)
}

func TestDomain_ParsersAndStrings(t *testing.T) {
	// Status
	assert.Equal(t, "TODO", domain.StatusTodo.String())
	assert.Equal(t, "IN_PROGRESS", domain.StatusInProgress.String())
	assert.Equal(t, "IN_REVIEW", domain.StatusInReview.String())
	assert.Equal(t, "DONE", domain.StatusDone.String())
	assert.Equal(t, "UNSPECIFIED", domain.StatusUnspecified.String())

	assert.Equal(t, domain.StatusTodo, domain.ParseStatus("TODO"))
	assert.Equal(t, domain.StatusInProgress, domain.ParseStatus("in_progress"))
	assert.Equal(t, domain.StatusInReview, domain.ParseStatus("IN_REVIEW"))
	assert.Equal(t, domain.StatusDone, domain.ParseStatus("Done"))
	assert.Equal(t, domain.StatusUnspecified, domain.ParseStatus("OTHER"))

	// Priority
	assert.Equal(t, "LOW", domain.PriorityLow.String())
	assert.Equal(t, "MEDIUM", domain.PriorityMedium.String())
	assert.Equal(t, "HIGH", domain.PriorityHigh.String())
	assert.Equal(t, "CRITICAL", domain.PriorityCritical.String())
	assert.Equal(t, "UNSPECIFIED", domain.PriorityUnspecified.String())

	assert.Equal(t, domain.PriorityLow, domain.ParsePriority("low"))
	assert.Equal(t, domain.PriorityMedium, domain.ParsePriority("MEDIUM"))
	assert.Equal(t, domain.PriorityHigh, domain.ParsePriority("High"))
	assert.Equal(t, domain.PriorityCritical, domain.ParsePriority("CRITICAL"))
	assert.Equal(t, domain.PriorityUnspecified, domain.ParsePriority("NONE"))

	// Type
	assert.Equal(t, "STORY", domain.TypeStory.String())
	assert.Equal(t, "TASK", domain.TypeTask.String())
	assert.Equal(t, "BUG", domain.TypeBug.String())
	assert.Equal(t, "EPIC", domain.TypeEpic.String())
	assert.Equal(t, "UNSPECIFIED", domain.TypeUnspecified.String())

	assert.Equal(t, domain.TypeStory, domain.ParseType("story"))
	assert.Equal(t, domain.TypeTask, domain.ParseType("TASK"))
	assert.Equal(t, domain.TypeBug, domain.ParseType("Bug"))
	assert.Equal(t, domain.TypeEpic, domain.ParseType("EPIC"))
	assert.Equal(t, domain.TypeUnspecified, domain.ParseType("INVALID"))
}

func TestDomain_EnumIsValid(t *testing.T) {
	// Status
	assert.False(t, domain.StatusUnspecified.IsValid())
	assert.True(t, domain.StatusTodo.IsValid())
	assert.True(t, domain.StatusInProgress.IsValid())
	assert.True(t, domain.StatusInReview.IsValid())
	assert.True(t, domain.StatusDone.IsValid())
	assert.False(t, domain.Status(-1).IsValid())
	assert.False(t, domain.Status(5).IsValid())
	assert.False(t, domain.Status(99).IsValid())

	// Priority
	assert.False(t, domain.PriorityUnspecified.IsValid())
	assert.True(t, domain.PriorityLow.IsValid())
	assert.True(t, domain.PriorityMedium.IsValid())
	assert.True(t, domain.PriorityHigh.IsValid())
	assert.True(t, domain.PriorityCritical.IsValid())
	assert.False(t, domain.Priority(-1).IsValid())
	assert.False(t, domain.Priority(5).IsValid())
	assert.False(t, domain.Priority(99).IsValid())

	// Type
	assert.False(t, domain.TypeUnspecified.IsValid())
	assert.True(t, domain.TypeStory.IsValid())
	assert.True(t, domain.TypeTask.IsValid())
	assert.True(t, domain.TypeBug.IsValid())
	assert.True(t, domain.TypeEpic.IsValid())
	assert.False(t, domain.Type(-1).IsValid())
	assert.False(t, domain.Type(5).IsValid())
	assert.False(t, domain.Type(99).IsValid())
}

func TestIssue_CanTransition_OutOfRangeStatuses(t *testing.T) {
	tests := []struct {
		name     string
		from     domain.Status
		to       domain.Status
		expected bool
	}{
		{"Negative from and to (-1 -> -1)", domain.Status(-1), domain.Status(-1), false},
		{"Large positive from and to (99 -> 99)", domain.Status(99), domain.Status(99), false},
		{"Negative from to valid to (-1 -> TODO)", domain.Status(-1), domain.StatusTodo, false},
		{"Valid from to negative to (TODO -> -1)", domain.StatusTodo, domain.Status(-1), false},
		{"Large positive from to valid to (99 -> TODO)", domain.Status(99), domain.StatusTodo, false},
		{"Valid from to large positive to (TODO -> 99)", domain.StatusTodo, domain.Status(99), false},
		{"Out of range (0 -> 0)", domain.StatusUnspecified, domain.StatusUnspecified, false},
		{"Boundary below (0 -> TODO)", domain.Status(0), domain.StatusTodo, false},
		{"Boundary above (5 -> 5)", domain.Status(5), domain.Status(5), false},
		{"Boundary above to valid (5 -> DONE)", domain.Status(5), domain.StatusDone, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			allowed := domain.CanTransition(tc.from, tc.to)
			assert.Equal(t, tc.expected, allowed)
		})
	}
}

func TestValidateStatusTransition_OutOfRangeStatuses(t *testing.T) {
	// Out of range current status
	err := domain.ValidateStatusTransition(domain.Status(99), domain.StatusTodo)
	assert.ErrorIs(t, err, domain.ErrInvalidStatus)

	err = domain.ValidateStatusTransition(domain.Status(-1), domain.StatusTodo)
	assert.ErrorIs(t, err, domain.ErrInvalidStatus)

	// Out of range next status
	err = domain.ValidateStatusTransition(domain.StatusTodo, domain.Status(99))
	assert.ErrorIs(t, err, domain.ErrInvalidStatus)

	err = domain.ValidateStatusTransition(domain.StatusTodo, domain.Status(-1))
	assert.ErrorIs(t, err, domain.ErrInvalidStatus)

	// Both out of range
	err = domain.ValidateStatusTransition(domain.Status(-1), domain.Status(-1))
	assert.ErrorIs(t, err, domain.ErrInvalidStatus)

	err = domain.ValidateStatusTransition(domain.Status(99), domain.Status(99))
	assert.ErrorIs(t, err, domain.ErrInvalidStatus)

	// Valid Scrum transitions
	err = domain.ValidateStatusTransition(domain.StatusTodo, domain.StatusInProgress)
	assert.NoError(t, err)

	err = domain.ValidateStatusTransition(domain.StatusInProgress, domain.StatusInReview)
	assert.NoError(t, err)

	err = domain.ValidateStatusTransition(domain.StatusInReview, domain.StatusDone)
	assert.NoError(t, err)

	err = domain.ValidateStatusTransition(domain.StatusDone, domain.StatusTodo)
	assert.NoError(t, err)

	// Invalid Scrum transition within range
	err = domain.ValidateStatusTransition(domain.StatusTodo, domain.StatusDone)
	assert.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
}
