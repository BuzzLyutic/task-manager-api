package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BuzzLyutic/task-manager-api/internal/handler"
	"github.com/BuzzLyutic/task-manager-api/internal/model"
	"github.com/BuzzLyutic/task-manager-api/internal/repo"
	"github.com/BuzzLyutic/task-manager-api/internal/service"
	"github.com/BuzzLyutic/task-manager-api/internal/worker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupE2EServer(t *testing.T) (*httptest.Server, func()) {
	pool, cleanup := SetupTestDB(t)
	TruncateTables(t, pool)

	taskRepo := repo.NewTaskRepo(pool)
	taskService := service.NewTaskService(taskRepo)
	logger := zap.NewNop()
	taskHandler := handler.NewTaskHandler(taskService, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	r.Route("/api/tasks", func(r chi.Router) {
		r.Post("/", taskHandler.Create)
		r.Get("/", taskHandler.List)
		r.Get("/{id}", taskHandler.Get)
		r.Patch("/{id}", taskHandler.Update)
		r.Delete("/{id}", taskHandler.Delete)
	})

	r.Get("/api/stats", taskHandler.Stats)

	// Start worker pool
	workerPool := worker.NewPool(pool, logger, 2)
	workerPool.Start(context.Background())

	server := httptest.NewServer(r)

	cleanupFunc := func() {
		workerPool.Stop()
		server.Close()
		cleanup()
	}

	return server, cleanupFunc
}

func TestE2E_FullWorkflow(t *testing.T) {
	server, cleanup := setupE2EServer(t)
	defer cleanup()

	t.Run("complete CRUD workflow", func(t *testing.T) {
		// 1. Create task
		createBody := model.Task{
			Title:    "E2E Test Task",
			Priority: 7,
		}
		body, _ := json.Marshal(createBody)
		
		resp, err := http.Post(server.URL+"/api/tasks", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var created model.Task
		json.NewDecoder(resp.Body).Decode(&created)
		resp.Body.Close()
		
		require.NotZero(t, created.ID)
		assert.Equal(t, "E2E Test Task", created.Title)
		assert.Equal(t, "pending", created.Status)

		// 2. Get task
		resp, err = http.Get(fmt.Sprintf("%s/api/tasks/%d", server.URL, created.ID))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var fetched model.Task
		json.NewDecoder(resp.Body).Decode(&fetched)
		resp.Body.Close()
		assert.Equal(t, created.ID, fetched.ID)

		// 3. Update task
		updateBody := model.Task{
			Title:    "Updated E2E Task",
			Priority: 9,
			Version:  created.Version,
		}
		body, _ = json.Marshal(updateBody)
		
		req, _ := http.NewRequest(http.MethodPatch, 
			fmt.Sprintf("%s/api/tasks/%d", server.URL, created.ID), 
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var updated model.Task
		json.NewDecoder(resp.Body).Decode(&updated)
		resp.Body.Close()
		assert.Equal(t, "Updated E2E Task", updated.Title)
		assert.Equal(t, 9, updated.Priority)

		// 4. List tasks
		resp, err = http.Get(server.URL + "/api/tasks")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var tasks []model.Task
		json.NewDecoder(resp.Body).Decode(&tasks)
		resp.Body.Close()
		assert.GreaterOrEqual(t, len(tasks), 1)

		// 5. Delete task
		req, _ = http.NewRequest(http.MethodDelete, 
			fmt.Sprintf("%s/api/tasks/%d", server.URL, created.ID), nil)
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp.Body.Close()

		// 6. Verify deletion
		resp, err = http.Get(fmt.Sprintf("%s/api/tasks/%d", server.URL, created.ID))
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		resp.Body.Close()
	})
}

func TestE2E_WorkerProcessing(t *testing.T) {
	server, cleanup := setupE2EServer(t)
	defer cleanup()

	// Create several tasks
	for i := 0; i < 5; i++ {
		task := model.Task{
			Title:    fmt.Sprintf("Worker Test %d", i),
			Priority: i + 1,
		}
		body, _ := json.Marshal(task)
		http.Post(server.URL+"/api/tasks", "application/json", bytes.NewReader(body))
	}

	// Wait for workers to process
	time.Sleep(12 * time.Second)

	// Check stats
	resp, err := http.Get(server.URL + "/api/stats")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var stats repo.Stats
	json.NewDecoder(resp.Body).Decode(&stats)
	resp.Body.Close()

	// Most should be completed by now
	assert.GreaterOrEqual(t, stats.ByStatus["completed"], 3, "at least 3 tasks should be completed")
	assert.Equal(t, 5, stats.TotalTasks)
}

func TestE2E_IdempotencyAcrossRequests(t *testing.T) {
	server, cleanup := setupE2EServer(t)
	defer cleanup()

	idempKey := "e2e-idem-test"
	task := model.Task{
		Title:    "Idempotent Task",
		Priority: 5,
	}
	body, _ := json.Marshal(task)

	// First request
	req1, _ := http.NewRequest(http.MethodPost, server.URL+"/api/tasks", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idempKey)

	resp1, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp1.StatusCode)

	var task1 model.Task
	json.NewDecoder(resp1.Body).Decode(&task1)
	resp1.Body.Close()

	// Second request with same key
	body, _ = json.Marshal(task) // Re-marshal
	req2, _ := http.NewRequest(http.MethodPost, server.URL+"/api/tasks", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idempKey)

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp2.StatusCode)

	var task2 model.Task
	json.NewDecoder(resp2.Body).Decode(&task2)
	resp2.Body.Close()

	// Should return same task
	assert.Equal(t, task1.ID, task2.ID)
}

func TestE2E_FilteringAndPagination(t *testing.T) {
	server, cleanup := setupE2EServer(t)
	defer cleanup()

	// Create tasks with different priorities
	for i := 0; i < 15; i++ {
		task := model.Task{
			Title:    fmt.Sprintf("Task %d", i),
			Priority: (i % 10) + 1,
		}
		body, _ := json.Marshal(task)
		http.Post(server.URL+"/api/tasks", "application/json", bytes.NewReader(body))
	}

	t.Run("filter by status", func(t *testing.T) {
		resp, _ := http.Get(server.URL + "/api/tasks?status=pending")
		var tasks []model.Task
		json.NewDecoder(resp.Body).Decode(&tasks)
		resp.Body.Close()

		for _, task := range tasks {
			assert.Equal(t, "pending", task.Status)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		resp, _ := http.Get(server.URL + "/api/tasks?limit=5")
		var tasks []model.Task
		json.NewDecoder(resp.Body).Decode(&tasks)
		resp.Body.Close()

		assert.LessOrEqual(t, len(tasks), 5)
	})
}

func TestE2E_HealthCheck(t *testing.T) {
	server, cleanup := setupE2EServer(t)
	defer cleanup()

	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health map[string]string
	json.NewDecoder(resp.Body).Decode(&health)
	resp.Body.Close()

	assert.Equal(t, "ok", health["status"])
}
