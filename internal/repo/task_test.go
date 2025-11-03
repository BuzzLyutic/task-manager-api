// internal/repo/task_test.go
package repo

import (
    "context"
    "os"
    "testing"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/BuzzLyutic/task-manager-api/internal/model"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
    dbURL := os.Getenv("TEST_DATABASE_URL")
    if dbURL == "" {
        t.Skip("TEST_DATABASE_URL not set")
    }

    pool, err := pgxpool.New(context.Background(), dbURL)
    if err != nil {
        t.Fatal(err)
    }

    // Очистка
    pool.Exec(context.Background(), "TRUNCATE tasks, idempotency_keys CASCADE")

    return pool
}

func TestTaskRepo_Create(t *testing.T) {
    pool := setupTestDB(t)
    defer pool.Close()

    repo := NewTaskRepo(pool)
    task := model.Task{Title: "Test", Priority: 5}

    created, err := repo.Create(context.Background(), task)
    if err != nil {
        t.Fatal(err)
    }

    if created.ID == 0 {
        t.Error("expected non-zero ID")
    }
    if created.Status != "pending" {
        t.Errorf("expected status=pending, got %s", created.Status)
    }
}
