package service

import (
	"context"
	"testing"

	"github.com/BuzzLyutic/task-manager-api/internal/model"
	"github.com/BuzzLyutic/task-manager-api/internal/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockTaskRepository - мок репозитория
type MockTaskRepository struct {
	mock.Mock
}

func (m *MockTaskRepository) Create(ctx context.Context, t model.Task) (model.Task, error) {
	args := m.Called(ctx, t)
	return args.Get(0).(model.Task), args.Error(1)
}

func (m *MockTaskRepository) Get(ctx context.Context, id int64) (model.Task, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.Task), args.Error(1)
}

func (m *MockTaskRepository) List(ctx context.Context, filter model.TaskFilter, limit int) ([]model.Task, error) {
	args := m.Called(ctx, filter, limit)
	return args.Get(0).([]model.Task), args.Error(1)
}

func (m *MockTaskRepository) Update(ctx context.Context, t model.Task) (model.Task, error) {
	args := m.Called(ctx, t)
	return args.Get(0).(model.Task), args.Error(1)
}

func (m *MockTaskRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskRepository) SaveIdempotencyKey(ctx context.Context, key string, resourceID int64) error {
	args := m.Called(ctx, key, resourceID)
	return args.Error(0)
}

func (m *MockTaskRepository) GetIdempotencyKey(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskRepository) GetStats(ctx context.Context) (repo.Stats, error) {
	args := m.Called(ctx)
	return args.Get(0).(repo.Stats), args.Error(1)
}

func TestTaskService_Create(t *testing.T) {
	tests := []struct {
		name      string
		task      model.Task
		idempKey  string
		setupMock func(*MockTaskRepository)
		wantErr   error
	}{
		{
			name: "successful creation without idempotency key",
			task: model.Task{
				Title:    "Test Task",
				Priority: 5,
			},
			idempKey: "",
			setupMock: func(m *MockTaskRepository) {
				m.On("Create", mock.Anything, mock.MatchedBy(func(t model.Task) bool {
					return t.Title == "Test Task" && t.Priority == 5
				})).Return(model.Task{
					ID:       1,
					Title:    "Test Task",
					Priority: 5,
					Status:   "pending",
				}, nil)
			},
			wantErr: nil,
		},
		{
			name: "validation error - empty title",
			task: model.Task{
				Title:    "",
				Priority: 5,
			},
			setupMock: func(m *MockTaskRepository) {},
			wantErr:   ErrValidation,
		},
		{
			name: "validation error - invalid priority",
			task: model.Task{
				Title:    "Test",
				Priority: 15,
			},
			setupMock: func(m *MockTaskRepository) {},
			wantErr:   ErrValidation,
		},
		{
			name: "idempotency - key exists",
			task: model.Task{
				Title:    "Test Task",
				Priority: 5,
			},
			idempKey: "key-123",
			setupMock: func(m *MockTaskRepository) {
				m.On("GetIdempotencyKey", mock.Anything, "key-123").Return(int64(42), nil)
				m.On("Get", mock.Anything, int64(42)).Return(model.Task{
					ID:       42,
					Title:    "Test Task",
					Priority: 5,
				}, nil)
			},
			wantErr: nil,
		},
		{
			name: "idempotency - new key",
			task: model.Task{
				Title:    "Test Task",
				Priority: 5,
			},
			idempKey: "key-456",
			setupMock: func(m *MockTaskRepository) {
				m.On("GetIdempotencyKey", mock.Anything, "key-456").Return(int64(0), repo.ErrorNotFound)
				m.On("Create", mock.Anything, mock.Anything).Return(model.Task{
					ID:       1,
					Title:    "Test Task",
					Priority: 5,
				}, nil)
				m.On("SaveIdempotencyKey", mock.Anything, "key-456", int64(1)).Return(nil)
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockTaskRepository)
			tt.setupMock(mockRepo)

			service := NewTaskService(mockRepo)
			result, err := service.Create(context.Background(), tt.task, tt.idempKey)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.NotZero(t, result.ID)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTaskService_List(t *testing.T) {
	tests := []struct {
		name      string
		filter    model.TaskFilter
		limit     int
		wantLimit int
		setupMock func(*MockTaskRepository)
	}{
		{
			name:      "default limit",
			filter:    model.TaskFilter{},
			limit:     0,
			wantLimit: 20,
			setupMock: func(m *MockTaskRepository) {
				m.On("List", mock.Anything, mock.Anything, 20).Return([]model.Task{}, nil)
			},
		},
		{
			name:      "custom limit",
			filter:    model.TaskFilter{},
			limit:     50,
			wantLimit: 50,
			setupMock: func(m *MockTaskRepository) {
				m.On("List", mock.Anything, mock.Anything, 50).Return([]model.Task{}, nil)
			},
		},
		{
			name:      "limit too high",
			filter:    model.TaskFilter{},
			limit:     200,
			wantLimit: 20,
			setupMock: func(m *MockTaskRepository) {
				m.On("List", mock.Anything, mock.Anything, 20).Return([]model.Task{}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockTaskRepository)
			tt.setupMock(mockRepo)

			service := NewTaskService(mockRepo)
			_, err := service.List(context.Background(), tt.filter, tt.limit)

			require.NoError(t, err)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTaskService_Update(t *testing.T) {
	mockRepo := new(MockTaskRepository)
	mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(t model.Task) bool {
		return t.ID == 1 && t.Title == "Updated"
	})).Return(model.Task{ID: 1, Title: "Updated", Priority: 5, Version: 2}, nil)

	service := NewTaskService(mockRepo)
	result, err := service.Update(context.Background(), model.Task{
		ID:       1,
		Title:    "Updated",
		Priority: 5,
		Version:  1,
	})

	require.NoError(t, err)
	assert.Equal(t, "Updated", result.Title)
	assert.Equal(t, 2, result.Version)
	mockRepo.AssertExpectations(t)
}

func TestTaskService_GetStats(t *testing.T) {
	mockRepo := new(MockTaskRepository)
	expectedStats := repo.Stats{
		ByStatus: map[string]int{
			"pending":    5,
			"processing": 2,
			"completed":  10,
		},
		AvgProcessing: 3.5,
		TotalTasks:    17,
	}

	mockRepo.On("GetStats", mock.Anything).Return(expectedStats, nil)

	service := NewTaskService(mockRepo)
	stats, err := service.GetStats(context.Background())

	require.NoError(t, err)
	assert.Equal(t, expectedStats, stats)
	mockRepo.AssertExpectations(t)
}

func TestTaskService_Validate(t *testing.T) {
	service := &TaskService{}

	tests := []struct {
		name    string
		task    model.Task
		wantErr bool
	}{
		{
			name:    "valid task",
			task:    model.Task{Title: "Valid", Priority: 5},
			wantErr: false,
		},
		{
			name:    "empty title",
			task:    model.Task{Title: "", Priority: 5},
			wantErr: true,
		},
		{
			name:    "whitespace title",
			task:    model.Task{Title: "   ", Priority: 5},
			wantErr: true,
		},
		{
			name:    "priority too low",
			task:    model.Task{Title: "Task", Priority: 0},
			wantErr: true,
		},
		{
			name:    "priority too high",
			task:    model.Task{Title: "Task", Priority: 11},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validate(tt.task)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrValidation)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
