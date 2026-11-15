package kafka_test

import (
	"encoding/json"
	"testing"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/kafka"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueEvent_ParentID(t *testing.T) {
	parent := "33333333-3333-4333-8333-333333333333"
	child := &domain.Issue{ID: "44444444-4444-4444-8444-444444444444", Key: "PROJ-2", Type: domain.TypeStory, ParentID: &parent}

	withParent, err := json.Marshal(kafka.NewIssueCreatedEvent(child))
	require.NoError(t, err)
	var decoded struct {
		Payload map[string]any `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(withParent, &decoded))
	assert.Equal(t, parent, decoded.Payload["parentId"])

	// Issues without a parent keep the payload MS3 already consumes: no parentId key at all.
	child.ParentID = nil
	withoutParent, err := json.Marshal(kafka.NewIssueCreatedEvent(child))
	require.NoError(t, err)
	assert.NotContains(t, string(withoutParent), "parentId")
}
