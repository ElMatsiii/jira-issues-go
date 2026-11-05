package kafka_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	kafkapkg "jira-issues-go/internal/kafka"
)

// capturingWriter captures kafka.Message structs for empirical verification of message keys.
type capturingWriter struct {
	messages []kafka.Message
}

func (c *capturingWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	c.messages = append(c.messages, msgs...)
	return nil
}

func (c *capturingWriter) Close() error {
	return nil
}

// TestChallenger_Kafka_PartitionKeyFidelity rigorously verifies that:
// 1. kafka.Message.Key is ALWAYS set to []byte(issue.ID) across all event methods
// 2. The key is NOT set to issue.Key, ProjectID, or left empty
// 3. Partition affinity is mathematically deterministic using kafka.Hash
func TestChallenger_Kafka_PartitionKeyFidelity(t *testing.T) {
	writer := &capturingWriter{}
	producer := kafkapkg.NewEventProducerWithWriter(writer, "issues.events")
	ctx := context.Background()

	testIssue := &domain.Issue{
		ID:          "4b1b3b24-9189-497d-bb6d-92a0e28f3a8b",
		Key:         "PROJ-42",
		ProjectID:   "3a74bca2-8db3-4318-a621-e3a19fc26207",
		Title:       "Partition Key Affinity Test",
		Description: "Testing deterministic keying",
		Status:      domain.StatusTodo,
		Priority:    domain.PriorityHigh,
		Type:        domain.TypeStory,
		StoryPoints: 5,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	t.Run("PublishIssueCreated sets Key to issue.ID", func(t *testing.T) {
		writer.messages = nil
		err := producer.PublishIssueCreated(ctx, testIssue)
		require.NoError(t, err)
		require.Len(t, writer.messages, 1)

		msg := writer.messages[0]
		assert.Equal(t, []byte(testIssue.ID), msg.Key)
		assert.NotEqual(t, []byte(testIssue.Key), msg.Key, "Key must not be human-readable issue key")
		assert.NotEqual(t, []byte(testIssue.ProjectID), msg.Key, "Key must not be project ID")
	})

	t.Run("PublishIssueUpdated sets Key to issue.ID", func(t *testing.T) {
		writer.messages = nil
		err := producer.PublishIssueUpdated(ctx, testIssue)
		require.NoError(t, err)
		require.Len(t, writer.messages, 1)

		msg := writer.messages[0]
		assert.Equal(t, []byte(testIssue.ID), msg.Key)
	})

	t.Run("PublishIssueStatusChanged sets Key to issue.ID", func(t *testing.T) {
		writer.messages = nil
		err := producer.PublishIssueStatusChanged(ctx, testIssue, domain.StatusTodo)
		require.NoError(t, err)
		require.Len(t, writer.messages, 1)

		msg := writer.messages[0]
		assert.Equal(t, []byte(testIssue.ID), msg.Key)
	})

	t.Run("PublishIssueSprintAssigned sets Key to issue.ID", func(t *testing.T) {
		writer.messages = nil
		sprintID := "sprint-uuid-10"
		err := producer.PublishIssueSprintAssigned(ctx, testIssue, &sprintID)
		require.NoError(t, err)
		require.Len(t, writer.messages, 1)

		msg := writer.messages[0]
		assert.Equal(t, []byte(testIssue.ID), msg.Key)
	})

	t.Run("PublishIssueDeleted sets Key to issue.ID", func(t *testing.T) {
		writer.messages = nil
		err := producer.PublishIssueDeleted(ctx, testIssue)
		require.NoError(t, err)
		require.Len(t, writer.messages, 1)

		msg := writer.messages[0]
		assert.Equal(t, []byte(testIssue.ID), msg.Key)
	})

	t.Run("Partition affinity determinism across 1000 generated issues", func(t *testing.T) {
		balancer := &kafka.Hash{}
		const partitionCount = 12
		partitions := make([]int, partitionCount)
		for i := 0; i < partitionCount; i++ {
			partitions[i] = i
		}

		writer.messages = nil
		const numIssues = 1000
		for i := 0; i < numIssues; i++ {
			iss := &domain.Issue{
				ID:        uuid.New().String(),
				Key:       fmt.Sprintf("AFF-%d", i),
				ProjectID: uuid.New().String(),
				Title:     fmt.Sprintf("Issue %d", i),
				Status:    domain.StatusTodo,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}

			err := producer.PublishIssueCreated(ctx, iss)
			require.NoError(t, err)

			// Emit a status change for the same issue
			err = producer.PublishIssueStatusChanged(ctx, iss, domain.StatusTodo)
			require.NoError(t, err)
		}

		require.Len(t, writer.messages, numIssues*2)

		// For each pair of messages for the same issue, verify:
		// 1. Both messages have identical Key
		// 2. Both messages route to the EXACT same partition using kafka.Hash
		partitionDistribution := make(map[int]int)
		for i := 0; i < numIssues; i++ {
			msgCreated := writer.messages[i*2]
			msgStatus := writer.messages[i*2+1]

			assert.Equal(t, msgCreated.Key, msgStatus.Key)

			partCreated := balancer.Balance(msgCreated, partitions...)
			partStatus := balancer.Balance(msgStatus, partitions...)

			assert.Equal(t, partCreated, partStatus,
				"Partition affinity mismatch: all events for issue %s must land on the same partition",
				string(msgCreated.Key),
			)

			partitionDistribution[partCreated]++
		}

		// Verify partition distribution: with 1000 issues across 12 partitions,
		// every partition should receive traffic (hash uniformity).
		assert.Len(t, partitionDistribution, partitionCount,
			"All 12 Kafka partitions must receive traffic under uniform hashing",
		)
		for p, count := range partitionDistribution {
			assert.Greater(t, count, 20, "Partition %d should have reasonable traffic share, got %d", p, count)
		}
	})

	t.Run("Edge Case - Empty AggregateID warning", func(t *testing.T) {
		writer.messages = nil
		envelope := &kafkapkg.IssueEventEnvelope{
			EventID:     uuid.New().String(),
			EventType:   kafkapkg.EventTypeIssueCreated,
			AggregateID: "", // Missing AggregateID
			Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		}

		err := producer.PublishIssueEvent(ctx, envelope)
		require.NoError(t, err)
		require.Len(t, writer.messages, 1)

		// When AggregateID is empty, msg.Key is empty byte slice
		msg := writer.messages[0]
		assert.Empty(t, msg.Key, "Empty AggregateID results in empty Key, breaking partition affinity")
	})
}
