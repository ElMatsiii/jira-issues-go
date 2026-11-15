package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/kafka"
)

func TestReporterName_TravelsInProtoButNotInEvents(t *testing.T) {
	reporter := "11111111-1111-4111-8111-111111111111"
	issue := &domain.Issue{ID: "i1", Key: "WEB-1", ReporterID: &reporter, ReporterName: "Ana"}

	pb := domain.IssueToProto(issue)
	assert.Equal(t, "Ana", pb.GetReporterName())
	assert.Equal(t, "Ana", domain.ProtoToIssue(pb).ReporterName)

	// issues.events has additionalProperties: false, so the name never reaches Kafka.
	event, err := json.Marshal(kafka.NewIssueCreatedEvent(issue))
	require.NoError(t, err)
	assert.NotContains(t, string(event), "reporterName")
	assert.NotContains(t, string(event), "Ana")
}
