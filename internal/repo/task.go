package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BuzzLyutic/task-manager-api/internal/model"
)

var (
	ErrorNotFound = errors.New("not found")
	ErrorConflict = errors.New("conflict")
)

type TaskRepo struct { // Репозиторий для работы непосредственно с БД
	pool *pgxpool.Pool
}

func NewTaskRepo(pool *pgxpool.Pool) *TaskRepo { // Конструктор
	return &TaskRepo{
		pool: pool,
	}
}

func (r *TaskRepo) Create(ctx context.Context, t model.Task) (model.Task, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO tasks (title, priority, status)
		VALUES ($1, $2, 'pending')
		RETURNING id, title, status, priority, version, created_at, updated_at
	`, t.Title, t.Priority).Scan(
		&t.ID, &t.Title, &t.Status, &t.Priority, &t.Version, &t.CreatedAt, &t.UpdatedAt,
	)
	return t, r.mapError(err)
}

func (r *TaskRepo) Get(ctx context.Context, id int64) (model.Task, error) {
	var t model.Task
	err := r.pool.QueryRow(ctx, `
		SELECT id, title, status, priority, version, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`, id).Scan(
		&t.ID, &t.Title, &t.Status, &t.Priority, &t.Version, &t.CreatedAt, &t.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return t, ErrorNotFound
	}
	return t, err
}

func (r *TaskRepo) List(ctx context.Context, filter model.TaskFilter, limit int) ([]model.Task, error) {
	query := `
		SELECT id, title, status, priority, version, created_at, updated_at
		FROM tasks
		WHERE ($1::text IS NULL OR status = $1)
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, filter.Status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]model.Task, 0, limit)
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.Priority, &t.Version, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *TaskRepo) Update(ctx context.Context, t model.Task) (model.Task, error) {
	err := r.pool.QueryRow(ctx, `
		UPDATE tasks
		SET title = $2, priority = $3, version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $4
		RETURNING id, title, status, priority, version, created_at, updated_at
	`, t.ID, t.Title, t.Priority, t.Version).Scan(
		&t.ID, &t.Title, &t.Status, &t.Priority, &t.Version, &t.CreatedAt, &t.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return t, ErrorConflict
	}
	return t, err
}

func (r *TaskRepo) Delete(ctx context.Context, id int64) error {
	cmd, err := r.pool.Exec(ctx, "DELETE FROM tasks WHERE id = $1", id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrorNotFound
	}
	return nil
}

func (r *TaskRepo) SaveIdempotencyKey(ctx context.Context, key string, resourceID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO idempotency_keys (key, resource_id) VALUES ($1, $2)
		ON CONFLICT (key) DO NOTHING
	`, key, resourceID)
	return err
}

func (r *TaskRepo) GetIdempotencyKey(ctx context.Context, key string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		SELECT resource_id from idempotency_keys WHERE key = $1
	`, key).Scan(&id)

	if err == pgx.ErrNoRows {
		return 0, ErrorNotFound
	}
	return id, err
}

func (r *TaskRepo) mapError(err error) error { // ЗАЧЕМ
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return ErrorConflict
		}
	}
	return err
}
