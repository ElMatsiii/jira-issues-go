package service_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
	"jira-issues-go/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChallenger_Concurrency_MockIssueRepository stress-tests MockIssueRepository
// under heavy concurrent read/write load to verify absence of data races and panics.
func TestChallenger_Concurrency_MockIssueRepository(t *testing.T) {
	repo := service.NewMockIssueRepository()
	ctx := context.Background()

	const numGoroutines = 50
	const opsPerGoroutine = 20

	var wg sync.WaitGroup

	// Phase 1: Concurrent Creation and Sequence Generation
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for op := 0; op < opsPerGoroutine; op++ {
				projID := fmt.Sprintf("proj-%d", goroutineID%5)
				seq, prefix, err := repo.GetNextSequence(ctx, projID)
				assert.NoError(t, err)
				assert.NotEmpty(t, prefix)
				assert.Greater(t, seq, int64(0))

				issue := &domain.Issue{
					ID:          fmt.Sprintf("issue-%d-%d", goroutineID, op),
					Key:         fmt.Sprintf("%s-%d", prefix, seq),
					ProjectID:   projID,
					Title:       fmt.Sprintf("Concurrent Issue %d-%d", goroutineID, op),
					Status:      domain.StatusTodo,
					Priority:    domain.PriorityMedium,
					Type:        domain.TypeStory,
					StoryPoints: int32(op % 8),
					Version:     1,
					CreatedAt:   time.Now().UTC(),
					UpdatedAt:   time.Now().UTC(),
				}

				err = repo.Create(ctx, issue)
				assert.NoError(t, err)
			}
		}(g)
	}
	wg.Wait()

	// Verify all items were created
	expectedCount := numGoroutines * opsPerGoroutine
	listRes, count, err := repo.List(ctx, repository.ListFilter{Limit: int32(expectedCount * 2)})
	require.NoError(t, err)
	assert.Equal(t, int32(expectedCount), count)
	assert.Len(t, listRes, expectedCount)

	// Phase 2: Simultaneous Concurrent Reads, Writes, Updates, BatchUpdates, and Lists
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for op := 0; op < opsPerGoroutine; op++ {
				targetID := fmt.Sprintf("issue-%d-%d", (goroutineID+op)%numGoroutines, op%opsPerGoroutine)

				// 1. Concurrent GetByID
				iss, getErr := repo.GetByID(ctx, targetID)
				if getErr == nil && iss != nil {
					assert.NotEmpty(t, iss.ID)
				}

				// 2. Concurrent GetByIDOrKey
				issOrKey, getOrKeyErr := repo.GetByIDOrKey(ctx, targetID)
				if getOrKeyErr == nil && issOrKey != nil {
					assert.NotEmpty(t, issOrKey.ID)
				}

				// 3. Concurrent List with filters
				_, _, listErr := repo.List(ctx, repository.ListFilter{
					ProjectID: fmt.Sprintf("proj-%d", (goroutineID+op)%5),
					Limit:     10,
				})
				assert.NoError(t, listErr)

				// 4. Concurrent Update
				if iss != nil {
					issCopy := *iss
					issCopy.Title = fmt.Sprintf("Updated %d-%d", goroutineID, op)
					_ = repo.Update(ctx, &issCopy)
				}

				// 5. Concurrent BatchUpdateSprint
				sprintID := fmt.Sprintf("sprint-%d", goroutineID%3)
				_, _ = repo.BatchUpdateSprint(ctx, []string{targetID}, sprintID)
			}
		}(g)
	}
	wg.Wait()
}

// TestChallenger_Concurrency_MockEventProducer stress-tests MockEventProducer
// under concurrent event publishing from 100 goroutines.
func TestChallenger_Concurrency_MockEventProducer(t *testing.T) {
	producer := service.NewMockEventProducer()
	ctx := context.Background()

	const numGoroutines = 50
	const eventsPerGoroutine = 40
	totalEvents := numGoroutines * eventsPerGoroutine

	var wg sync.WaitGroup
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for e := 0; e < eventsPerGoroutine; e++ {
				issue := &domain.Issue{
					ID:        fmt.Sprintf("issue-%d-%d", goroutineID, e),
					Key:       fmt.Sprintf("PROJ-%d", e),
					ProjectID: "proj-1",
					Title:     "Concurrent Event Test",
					Status:    domain.StatusTodo,
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
				}

				switch e % 5 {
				case 0:
					_ = producer.PublishIssueCreated(ctx, issue)
				case 1:
					_ = producer.PublishIssueUpdated(ctx, issue)
				case 2:
					_ = producer.PublishIssueStatusChanged(ctx, issue, domain.StatusTodo)
				case 3:
					sprint := "sprint-1"
					_ = producer.PublishIssueSprintAssigned(ctx, issue, &sprint)
				case 4:
					_ = producer.PublishIssueDeleted(ctx, issue)
				}
			}
		}(g)
	}
	wg.Wait()

	// Verify thread-safe slice recording without data loss or race corruption
	assert.Len(t, producer.Events, totalEvents)
	assert.NoError(t, producer.Close())
}

// TestChallenger_Concurrency_IssueServicePipeline stress-tests the entire
// business logic pipeline under concurrent requests for creates, status transitions,
// and batch sprint assignments.
func TestChallenger_Concurrency_IssueServicePipeline(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)
	ctx := context.Background()

	const numWorkers = 25
	const tasksPerWorker = 20

	var createdIDs sync.Map
	var wg sync.WaitGroup

	// Phase 1: Concurrent Issue Creation via Service
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for tsk := 0; tsk < tasksPerWorker; tsk++ {
				dto := service.CreateIssueDTO{
					ProjectID:   fmt.Sprintf("proj-%d", workerID%4),
					Title:       fmt.Sprintf("Worker-%d-Task-%d", workerID, tsk),
					Description: "Stress test service pipeline",
					Priority:    domain.PriorityHigh,
					Type:        domain.TypeStory,
					StoryPoints: int32(tsk + 1),
				}
				issue, err := svc.CreateIssue(ctx, dto)
				assert.NoError(t, err)
				if issue != nil {
					createdIDs.Store(issue.ID, issue)
				}
			}
		}(w)
	}
	wg.Wait()

	// Verify all created issues emitted events
	expectedCount := numWorkers * tasksPerWorker
	assert.Len(t, producer.Events, expectedCount)

	// Collect IDs
	var allIDs []string
	createdIDs.Range(func(key, val interface{}) bool {
		allIDs = append(allIDs, key.(string))
		return true
	})
	require.Len(t, allIDs, expectedCount)

	// Phase 2: Concurrent Status Transitions and Sprint Assignments
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for tsk := 0; tsk < tasksPerWorker; tsk++ {
				idx := (workerID*tasksPerWorker + tsk) % len(allIDs)
				id := allIDs[idx]

				// Transition TODO -> IN_PROGRESS
				updated, err := svc.UpdateIssueStatus(ctx, id, domain.StatusInProgress)
				if err == nil && updated != nil {
					assert.Equal(t, domain.StatusInProgress, updated.Status)
				}

				// Batch sprint assignment
				sprint := fmt.Sprintf("sprint-%d", workerID%5)
				_, count, batchErr := svc.BatchUpdateIssueSprint(ctx, []string{id}, sprint)
				assert.NoError(t, batchErr)
				assert.Equal(t, int32(1), count)

				// List issues concurrently
				_, _, listErr := svc.ListIssues(ctx, repository.ListFilter{
					ProjectID: fmt.Sprintf("proj-%d", workerID%4),
					Limit:     5,
				})
				assert.NoError(t, listErr)
			}
		}(w)
	}
	wg.Wait()
}
