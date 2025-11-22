package repo

import (
	"context"
	"testing"

	"github.com/BuzzLyutic/task-manager-api/internal/model"
	"github.com/BuzzLyutic/task-manager-api/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRepo_Create(t *testing.T) {
	pool, cleanup := tests.SetupTestDB(t)
	defer cleanup()

	repo := NewTaskRepo(pool)
	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		tests.TruncateTables(t, pool)

		task := model.Task{
			Title:    "Test Task",
			Priority: 5,
		}

		result, err := repo.Create(ctx, task)
		require.NoError(t, err)
		assert.NotZero(t, result.ID)
		assert.Equal(t, "Test Task", result.Title)
		assert.Equal(t, 5, result.Priority)
		assert.Equal(t, "pending", result.Status)
		assert.Equal(t, 1, result.Version)
		assert.False(t, result.CreatedAt.IsZero())
		assert.False(t, result.UpdatedAt.IsZero())
	})
}

func TestTaskRepo_Get(t *testing.T) {
	pool, cleanup := tests.SetupTestDB(t)
	defer cleanup()

	repo := NewTaskRepo(pool)
	ctx := context.Background()

	t.Run("existing task", func(t *testing.T) {
		tests.TruncateTables(t, pool)
		ids := tests.SeedTasks(t, pool, 1)

		task, err := repo.Get(ctx, ids[0])
		require.NoError(t, err)
		assert.Equal(t, ids[0], task.ID)
		assert.NotEmpty(t, task.Title)
	})

	t.Run("non-existing task", func(t *testing.T) {
		_, err := repo.Get(ctx, 99999)
		assert.ErrorIs(t, err, ErrorNotFound)
	})
}

func TestTaskRepo_List(t *testing.T) {
	pool, cleanup := tests.SetupTestDB(t)
	defer cleanup()

	repo := NewTaskRepo(pool)
	ctx := context.Background()

	tests.TruncateTables(t, pool)
	tests.SeedTasks(t, pool, 15)

	// Update some tasks to have different statuses
	pool.Exec(ctx, "UPDATE tasks SET status = 'completed' WHERE id <= 5")
	pool.Exec(ctx, "UPDATE tasks SET status = 'processing' WHERE id > 5 AND id <= 10")

	t.Run("list all", func(t *testing.T) {
		tasks, err := repo.List(ctx, model.TaskFilter{}, 20)
		require.NoError(t, err)
		assert.Len(t, tasks, 15)
	})

	t.Run("filter by status", func(t *testing.T) {
		status := "completed"
		tasks, err := repo.List(ctx, model.TaskFilter{Status: &status}, 20)
		require.NoError(t, err)
		assert.Len(t, tasks, 5)
		for _, task := range tasks {
			assert.Equal(t, "completed", task.Status)
		}
	})

	t.Run("with limit", func(t *testing.T) {
		tasks, err := repo.List(ctx, model.TaskFilter{}, 5)
		require.NoError(t, err)
		assert.Len(t, tasks, 5)
	})
}

func TestTaskRepo_Update(t *testing.T) {
	pool, cleanup := tests.SetupTestDB(t)
	defer cleanup()

	repo := NewTaskRepo(pool)
	ctx := context.Background()

	t.Run("successful update", func(t *testing.T) {
		tests.TruncateTables(t, pool)
		ids := tests.SeedTasks(t, pool, 1)

		original, _ := repo.Get(ctx, ids[0])
		original.Title = "Updated Title"
		original.Priority = 8

		updated, err := repo.Update(ctx, original)
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", updated.Title)
		assert.Equal(t, 8, updated.Priority)
		assert.Equal(t, original.Version+1, updated.Version)
	})

	t.Run("optimistic lock conflict", func(t *testing.T) {
		tests.TruncateTables(t, pool)
		ids := tests.SeedTasks(t, pool, 1)

		task, _ := repo.Get(ctx, ids[0])
		task.Title = "Updated"
		task.Version = 999 // Wrong version

		_, err := repo.Update(ctx, task)
		assert.ErrorIs(t, err, ErrorConflict)
	})
}

func TestTaskRepo_Delete(t *testing.T) {
	pool, cleanup := tests.SetupTestDB(t)
	defer cleanup()

	repo := NewTaskRepo(pool)
	ctx := context.Background()

	t.Run("successful delete", func(t *testing.T) {
		tests.TruncateTables(t, pool)
		ids := tests.SeedTasks(t, pool, 1)

		err := repo.Delete(ctx, ids[0])
		require.NoError(t, err)

		_, err = repo.Get(ctx, ids[0])
		assert.ErrorIs(t, err, ErrorNotFound)
	})

	t.Run("delete non-existing", func(t *testing.T) {
		err := repo.Delete(ctx, 99999)
		assert.ErrorIs(t, err, ErrorNotFound)
	})
}

func TestTaskRepo_Idempotency(t *testing.T) {
	pool, cleanup := tests.SetupTestDB(t)
	defer cleanup()

	repo := NewTaskRepo(pool)
	ctx := context.Background()

	tests.TruncateTables(t, pool)

	t.Run("save and get idempotency key", func(t *testing.T) {
		key := "test-key-123"
		resourceID := int64(42)

		// Save
		err := repo.SaveIdempotencyKey(ctx, key, resourceID)
		require.NoError(t, err)

		// Get
		id, err := repo.GetIdempotencyKey(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, resourceID, id)
	})

	t.Run("get non-existing key", func(t *testing.T) {
		_, err := repo.GetIdempotencyKey(ctx, "non-existing")
		assert.ErrorIs(t, err, ErrorNotFound)
	})

	t.Run("duplicate key does not fail", func(t *testing.T) {
		key := "duplicate-key"
		err := repo.SaveIdempotencyKey(ctx, key, 1)
		require.NoError(t, err)

		// Second save with same key should not fail (ON CONFLICT DO NOTHING)
		err = repo.SaveIdempotencyKey(ctx, key, 2)
		assert.NoError(t, err)

		// But should return original value
		id, _ := repo.GetIdempotencyKey(ctx, key)
		assert.Equal(t, int64(1), id)
	})
}

func TestTaskRepo_GetStats(t *testing.T) {
	pool, cleanup := tests.SetupTestDB(t)
	defer cleanup()

	repo := NewTaskRepo(pool)
	ctx := context.Background()

	tests.TruncateTables(t, pool)
	tests.SeedTasks(t, pool, 20)

	// Set different statuses
	pool.Exec(ctx, "UPDATE tasks SET status = 'completed' WHERE id <= 10")
	pool.Exec(ctx, "UPDATE tasks SET status = 'processing' WHERE id > 10 AND id <= 15")

	stats, err := repo.GetStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 10, stats.ByStatus["completed"])
	assert.Equal(t, 5, stats.ByStatus["processing"])
	assert.Equal(t, 5, stats.ByStatus["pending"])
	assert.Equal(t, 20, stats.TotalTasks)
	assert.GreaterOrEqual(t, stats.AvgProcessing, 0.0)
}
