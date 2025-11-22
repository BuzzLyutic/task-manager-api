package service

import (
	"context"
	"errors"
	"strings"

	"github.com/BuzzLyutic/task-manager-api/internal/model"
	"github.com/BuzzLyutic/task-manager-api/internal/repo"
)

var (
	ErrValidation = errors.New("validation error")
)

type TaskService struct {
	repo repo.TaskRepository
}

func NewTaskService(repo repo.TaskRepository) *TaskService {
	return &TaskService{repo: repo}
}

func (s *TaskService) Create(ctx context.Context, t model.Task, idempKey string) (model.Task, error) {
	if err := s.validate(t); err != nil { // Валидация модели на корректность введенных данных
		return t, err
	}

	if idempKey != "" { // Обеспечение идемпотентности - если ключ с ресурсом уже существует, мы не создаем его еще раз
		if existingID, err := s.repo.GetIdempotencyKey(ctx, idempKey); err == nil {
			return s.repo.Get(ctx, existingID)
		}
	}

	// Создание новой задачи
	resource, err := s.repo.Create(ctx, t)
	if err != nil {
		return resource, err
	}

	// Сохранение нового ключа
	if idempKey != "" {
		s.repo.SaveIdempotencyKey(ctx, idempKey, resource.ID)
	}

	return resource, nil
}

func (s *TaskService) Get(ctx context.Context, id int64) (model.Task, error) {
	return s.repo.Get(ctx, id)
}

func (s *TaskService) List(ctx context.Context, filter model.TaskFilter, limit int) ([]model.Task, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, filter, limit)
}

func (s *TaskService) Update(ctx context.Context, t model.Task) (model.Task, error) {
	if err := s.validate(t); err != nil {
		return t, err
	}
	return s.repo.Update(ctx, t)
}

func (s *TaskService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *TaskService) GetStats(ctx context.Context) (repo.Stats, error) {
    return s.repo.GetStats(ctx)
}

func (s *TaskService) validate(t model.Task) error {
	if strings.TrimSpace(t.Title) == "" {
		return ErrValidation
	}
	if t.Priority < 1 || t.Priority > 10 {
		return ErrValidation
	}
	return nil
}
