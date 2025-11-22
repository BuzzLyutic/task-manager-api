package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BuzzLyutic/task-manager-api/internal/model"
	"github.com/BuzzLyutic/task-manager-api/internal/repo"
	"github.com/BuzzLyutic/task-manager-api/internal/service"
	"github.com/BuzzLyutic/task-manager-api/tests"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupHandler(t *testing.T) (*TaskHandler, func()) {
	pool, cleanup := tests.SetupTestDB(t)
	
	taskRepo := repo.NewTaskRepo(pool)
	taskService := service.NewTaskService(taskRepo)
	logger := zap.NewNop()
	handler := NewTaskHandler(taskService, logger)

	return handler, cleanup
}

func TestTaskHandler_Create(t *testing.T) {
	handler, cleanup := setupHandler(t)
	defer cleanup()

	tests := []struct {
		name           string
		body           interface{}
		idempKey       string
		wantCode       int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful creation",
			body: model.Task{
				Title:    "Test Task",
				Priority: 5,
			},
			idempKey: "",
			wantCode: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var task model.Task
				json.NewDecoder(w.Body).Decode(&task)
				assert.NotZero(t, task.ID)
				assert.Equal(t, "Test Task", task.Title)
				assert.Contains(t, w.Header().Get("Location"), "/api/tasks/")
			},
		},
		{
			name:     "empty body",
			body:     nil,
			wantCode: http.StatusBadRequest,
		},
		{
			name: "validation error",
			body: model.Task{
				Title:    "",
				Priority: 5,
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "with idempotency key",
			body: model.Task{
				Title:    "Idempotent Task",
				Priority: 7,
			},
			idempKey: "test-key-123",
			wantCode: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Send again with same key
				body, _ := json.Marshal(model.Task{Title: "Idempotent Task", Priority: 7})
				req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", "test-key-123")
				
				w2 := httptest.NewRecorder()
				handler.Create(w2, req)
				
				var task1, task2 model.Task
				json.NewDecoder(w.Body).Decode(&task1)
				json.NewDecoder(w2.Body).Decode(&task2)
				
				assert.Equal(t, task1.ID, task2.ID, "should return same task")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.idempKey != "" {
				req.Header.Set("Idempotency-Key", tt.idempKey)
			}

			w := httptest.NewRecorder()
			handler.Create(w, req)

			assert.Equal(t, tt.wantCode, w.Code)
			
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestTaskHandler_Get(t *testing.T) {
	handler, cleanup := setupHandler(t)
	defer cleanup()

	// Create a task first
	createReq := model.Task{Title: "Get Test", Priority: 5}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	handler.Create(w, req)
	
	var created model.Task
	json.NewDecoder(w.Body).Decode(&created)

	t.Run("get existing task", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tasks/%d", created.ID), nil)
		
		// Add chi URL params
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", fmt.Sprintf("%d", created.ID))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var task model.Task
		json.NewDecoder(w.Body).Decode(&task)
		assert.Equal(t, created.ID, task.ID)
	})

	t.Run("get non-existing task", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/tasks/99999", nil)
		
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "99999")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.Get(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestTaskHandler_List(t *testing.T) {
	handler, cleanup := setupHandler(t)
	defer cleanup()

	// Create some tasks
	for i := 0; i < 5; i++ {
		task := model.Task{Title: fmt.Sprintf("Task %d", i), Priority: i + 1}
		body, _ := json.Marshal(task)
		req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.Create(w, req)
	}

	t.Run("list all tasks", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
		w := httptest.NewRecorder()
		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var tasks []model.Task
		json.NewDecoder(w.Body).Decode(&tasks)
		assert.GreaterOrEqual(t, len(tasks), 5)
	})

	t.Run("filter by status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/tasks?status=pending", nil)
		w := httptest.NewRecorder()
		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var tasks []model.Task
		json.NewDecoder(w.Body).Decode(&tasks)
		for _, task := range tasks {
			assert.Equal(t, "pending", task.Status)
		}
	})

	t.Run("with limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/tasks?limit=3", nil)
		w := httptest.NewRecorder()
		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var tasks []model.Task
		json.NewDecoder(w.Body).Decode(&tasks)
		assert.LessOrEqual(t, len(tasks), 3)
	})
}

func TestTaskHandler_Update(t *testing.T) {
	handler, cleanup := setupHandler(t)
	defer cleanup()

	// Create a task
	createReq := model.Task{Title: "Original", Priority: 5}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	handler.Create(w, req)
	
	var created model.Task
	json.NewDecoder(w.Body).Decode(&created)

	t.Run("successful update", func(t *testing.T) {
		updateReq := model.Task{
			Title:    "Updated",
			Priority: 8,
			Version:  created.Version,
		}
		body, _ := json.Marshal(updateReq)
		
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/tasks/%d", created.ID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", fmt.Sprintf("%d", created.ID))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.Update(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var updated model.Task
		json.NewDecoder(w.Body).Decode(&updated)
		assert.Equal(t, "Updated", updated.Title)
		assert.Equal(t, 8, updated.Priority)
		assert.Equal(t, created.Version+1, updated.Version)
	})

	t.Run("version conflict", func(t *testing.T) {
		updateReq := model.Task{
			Title:    "Conflict",
			Priority: 3,
			Version:  999,
		}
		body, _ := json.Marshal(updateReq)
		
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/tasks/%d", created.ID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", fmt.Sprintf("%d", created.ID))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.Update(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})
}

func TestTaskHandler_Delete(t *testing.T) {
	handler, cleanup := setupHandler(t)
	defer cleanup()

	// Create a task
	createReq := model.Task{Title: "To Delete", Priority: 5}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	handler.Create(w, req)
	
	var created model.Task
	json.NewDecoder(w.Body).Decode(&created)

	t.Run("successful delete", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/tasks/%d", created.ID), nil)
		
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", fmt.Sprintf("%d", created.ID))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.Delete(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("delete non-existing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/tasks/99999", nil)
		
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "99999")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.Delete(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestTaskHandler_Stats(t *testing.T) {
	handler, cleanup := setupHandler(t)
	defer cleanup()

	// Create various tasks
	for i := 0; i < 10; i++ {
		task := model.Task{Title: fmt.Sprintf("Task %d", i), Priority: 5}
		body, _ := json.Marshal(task)
		req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.Create(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	handler.Stats(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var stats repo.Stats
	err := json.NewDecoder(w.Body).Decode(&stats)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TotalTasks, 10)
	assert.NotNil(t, stats.ByStatus)
}
