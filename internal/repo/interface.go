package repo

import (
	"context"

	"github.com/BuzzLyutic/task-manager-api/internal/model"
)

// TaskRepository определяет интерфейс для работы с задачами
type TaskRepository interface {
	Create(ctx context.Context, t model.Task) (model.Task, error)
	Get(ctx context.Context, id int64) (model.Task, error)
	List(ctx context.Context, filter model.TaskFilter, limit int) ([]model.Task, error)
	Update(ctx context.Context, t model.Task) (model.Task, error)
	Delete(ctx context.Context, id int64) error
	SaveIdempotencyKey(ctx context.Context, key string, resourceID int64) error
	GetIdempotencyKey(ctx context.Context, key string) (int64, error)
	GetStats(ctx context.Context) (Stats, error)
}
