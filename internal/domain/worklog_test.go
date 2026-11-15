package domain_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"
)

func TestNewWorkLog_RoundsHoursLikeTheColumn(t *testing.T) {
	log, err := domain.NewWorkLog("u1", "p1", "i1", 1.256, "  Revisión del PR  ")

	require.NoError(t, err)
	assert.Equal(t, 1.26, log.Hours)
	assert.Equal(t, "Revisión del PR", log.Note)
	assert.Equal(t, "u1", log.UserID)
	assert.Equal(t, "p1", log.ProjectID)
	assert.Equal(t, "i1", log.IssueID)

	for _, hours := range []float64{0.01, 0.005, 24} {
		_, err := domain.NewWorkLog("u1", "p1", "i1", hours, "")
		assert.NoError(t, err, "%v hours", hours)
	}
}

func TestNewWorkLog_RejectsInvalidEntries(t *testing.T) {
	for name, hours := range map[string]float64{
		"zero":            0,
		"negative":        -1,
		"rounds to zero":  0.004,
		"more than a day": 24.01,
		"not a number":    math.NaN(),
		"infinite":        math.Inf(1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewWorkLog("u1", "p1", "i1", hours, "")
			assert.ErrorIs(t, err, domain.ErrInvalidWorkLog)
		})
	}

	_, err := domain.NewWorkLog("u1", "p1", "i1", 1, strings.Repeat("ñ", 501))
	assert.ErrorIs(t, err, domain.ErrInvalidWorkLog)
	_, err = domain.NewWorkLog("u1", "p1", "i1", 1, strings.Repeat("ñ", 500))
	assert.NoError(t, err)
}

func TestWorkLogToProto(t *testing.T) {
	logged := time.Date(2026, 11, 15, 9, 30, 0, 0, time.UTC)
	got := domain.WorkLogToProto(&domain.WorkLog{
		ID: "w1", ProjectID: "p1", IssueID: "i1", IssueKey: "WEB-3", IssueTitle: "Login", UserID: "u1",
		UserName: "Ana", SprintID: "s1", Hours: 2.5, Note: "pair", LoggedAt: logged,
	})

	assert.Equal(t, &issuesv1.WorkLog{
		Id: "w1", ProjectId: "p1", IssueId: "i1", IssueKey: "WEB-3", IssueTitle: "Login", UserId: "u1",
		UserName: "Ana", SprintId: "s1", Hours: 2.5, Note: "pair", LoggedAt: logged.UnixMilli(),
	}, got)
	assert.Nil(t, domain.WorkLogToProto(nil))
}
