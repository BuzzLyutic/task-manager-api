package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SetupTestDB создает тестовую БД с помощью testcontainers
func SetupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	// Находим путь к миграциям
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filename))
	migrationsPath := filepath.Join(projectRoot, "migrations")

	// Создаем PostgreSQL контейнер
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.WithInitScripts(filepath.Join(migrationsPath, "001_create_tasks.up.sql")),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	cleanup := func() {
		pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Errorf("Failed to terminate container: %v", err)
		}
	}

	return pool, cleanup
}

// TruncateTables очищает все таблицы
func TruncateTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	
	_, err := pool.Exec(ctx, "TRUNCATE tasks, idempotency_keys RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}
}

// SeedTasks создает тестовые задачи
func SeedTasks(t *testing.T, pool *pgxpool.Pool, count int) []int64 {
	t.Helper()
	ctx := context.Background()
	
	ids := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		var id int64
		err := pool.QueryRow(ctx, `
			INSERT INTO tasks (title, priority, status)
			VALUES ($1, $2, $3)
			RETURNING id
		`, fmt.Sprintf("Task %d", i+1), (i%10)+1, "pending").Scan(&id)
		
		if err != nil {
			t.Fatalf("Failed to seed task: %v", err)
		}
		ids = append(ids, id)
	}
	
	return ids
}

// WaitForCondition ждет выполнения условия с таймаутом
func WaitForCondition(t *testing.T, timeout time.Duration, condition func() bool) bool {
	t.Helper()
	
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
