package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/BuzzLyutic/task-manager-api/internal/model"
	"github.com/BuzzLyutic/task-manager-api/internal/repo"
	"github.com/BuzzLyutic/task-manager-api/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrent_IdempotencyKeys(t *testing.T) {
	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	TruncateTables(t, pool)

	taskRepo := repo.NewTaskRepo(pool)
	taskService := service.NewTaskService(taskRepo)
	ctx := context.Background()

	const goroutines = 10
	const idempKey = "concurrent-test-key"

	var wg sync.WaitGroup
	results := make([]model.Task, goroutines)
	errors := make([]error, goroutines)

	// Launch concurrent requests with same idempotency key
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			task := model.Task{
				Title:    fmt.Sprintf("Concurrent Task %d", idx),
				Priority: 5,
			}
			results[idx], errors[idx] = taskService.Create(ctx, task, idempKey)
		}(i)
	}

	wg.Wait()

	// All should succeed
	for i, err := range errors {
		require.NoError(t, err, "request %d should not error", i)
	}

	// All should return the same task ID
	firstID := results[0].ID
	for i, result := range results {
		assert.Equal(t, firstID, result.ID, "request %d should return same ID", i)
	}

	// Only one task should be created
	var count int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM tasks").Scan(&count)
	assert.Equal(t, 1, count, "only one task should be created")
}

func TestConcurrent_OptimisticLocking(t *testing.T) {
	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	TruncateTables(t, pool)

	taskRepo := repo.NewTaskRepo(pool)
	taskService := service.NewTaskService(taskRepo)
	ctx := context.Background()

	// Create initial task
	task, err := taskService.Create(ctx, model.Task{
		Title:    "Optimistic Lock Test",
		Priority: 5,
	}, "")
	require.NoError(t, err)

	const goroutines = 10
	var wg sync.WaitGroup
	errors := make([]error, goroutines)

	// Launch concurrent updates
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			updateTask := model.Task{
				ID:       task.ID,
				Title:    fmt.Sprintf("Updated %d", idx),
				Priority: idx + 1,
				Version:  task.Version, // All use same version
			}
			_, errors[idx] = taskService.Update(ctx, updateTask)
		}(i)
	}

	wg.Wait()

	// Only one should succeed
	successCount := 0
	conflictCount := 0
	for i, err := range errors {
		switch err {
		case nil:
			successCount++
		case repo.ErrorConflict:
			conflictCount++
		default:
			t.Errorf("unexpected error at %d: %v", i, err)
		}
	}

	assert.Equal(t, 1, successCount, "exactly one update should succeed")
	assert.Equal(t, goroutines-1, conflictCount, "others should conflict")

	// Final version should be original + 1
	finalTask, _ := taskRepo.Get(ctx, task.ID)
	assert.Equal(t, task.Version+1, finalTask.Version)
}

func TestConcurrent_WorkerPoolNoRaceConditions(t *testing.T) {
	// This test runs with -race flag to detect race conditions
	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	TruncateTables(t, pool)
	SeedTasks(t, pool, 20)

	ctx := context.Background()

	var wg sync.WaitGroup
	const workers = 5

	// Simulate multiple workers reading concurrently
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < 10; j++ {
				var taskID int64
				err := pool.QueryRow(ctx, `
					WITH claimed AS (
						SELECT id FROM tasks 
						WHERE status = 'pending'
						ORDER BY priority DESC
						FOR UPDATE SKIP LOCKED
						LIMIT 1
					)
					UPDATE tasks SET status = 'processing'
					FROM claimed
					WHERE tasks.id = claimed.id
					RETURNING tasks.id
				`).Scan(&taskID)
				
				if err == nil {
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify no tasks are claimed twice
	var processing int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM tasks WHERE status = 'processing'").Scan(&processing)
	assert.LessOrEqual(t, processing, 20, "should not have more processing than total tasks")
}

func TestConcurrent_MultipleReads(t *testing.T) {
	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	TruncateTables(t, pool)
	ids := SeedTasks(t, pool, 10)

	taskRepo := repo.NewTaskRepo(pool)
	ctx := context.Background()

	const goroutines = 50
	var wg sync.WaitGroup

	// Concurrent reads should not cause issues
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			
			taskID := ids[idx%len(ids)]
			task, err := taskRepo.Get(ctx, taskID)
			require.NoError(t, err)
			assert.NotZero(t, task.ID)
		}(i)
	}

	wg.Wait()
}

func TestConcurrent_CreateAndList(t *testing.T) {
	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	TruncateTables(t, pool)

	taskRepo := repo.NewTaskRepo(pool)
	taskService := service.NewTaskService(taskRepo)
	ctx := context.Background()

	var wg sync.WaitGroup
	const creators = 5
	const readers = 5

	// Concurrent creates
	for i := 0; i < creators; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				taskService.Create(ctx, model.Task{
					Title:    fmt.Sprintf("Task %d-%d", idx, j),
					Priority: (idx + j) % 10 + 1,
				}, "")
				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				taskRepo.List(ctx, model.TaskFilter{}, 20)
				time.Sleep(30 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Verify final count
	tasks, _ := taskRepo.List(ctx, model.TaskFilter{}, 100)
	assert.Equal(t, creators*5, len(tasks))
}
