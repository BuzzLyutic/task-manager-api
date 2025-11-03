package worker

import (
    "context"
    "math/rand"
    "sync"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "go.uber.org/zap"

    "github.com/BuzzLyutic/task-manager-api/internal/model"
)

type Pool struct {
    pool   *pgxpool.Pool
    logger *zap.Logger
    count  int
    wg     sync.WaitGroup
    stop   chan struct{}
}

func NewPool(pool *pgxpool.Pool, logger *zap.Logger, count int) *Pool {
    return &Pool{
        pool:   pool,
        logger: logger,
        count:  count,
        stop:   make(chan struct{}),
    }
}

func (p *Pool) Start(ctx context.Context) {
    p.logger.Info("Starting worker pool", zap.Int("workers", p.count))
    
    for i := 0; i < p.count; i++ {
        p.wg.Add(1)
        go p.worker(ctx, i)
    }
}

func (p *Pool) Stop() {
    p.logger.Info("Stopping worker pool...")
    close(p.stop)
    p.wg.Wait()
    p.logger.Info("Worker pool stopped")
}

func (p *Pool) worker(ctx context.Context, id int) {
    defer p.wg.Done()
    
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-p.stop:
            return
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := p.processNext(ctx, id); err != nil && err != pgx.ErrNoRows {
                p.logger.Error("worker error", zap.Int("worker", id), zap.Error(err))
            }
        }
    }
}

func (p *Pool) processNext(ctx context.Context, workerID int) error {
    // Забрать задачу
    task, err := p.claimTask(ctx)
    if err != nil {
        return err
    }

    p.logger.Info("Processing task",
        zap.Int("worker", workerID),
        zap.Int64("task_id", task.ID),
        zap.String("title", task.Title),
    )

    // Эмуляция работы
    processingTime := time.Duration(2+rand.Intn(3)) * time.Second
    select {
    case <-time.After(processingTime):
        // Успех
        if err := p.completeTask(ctx, task.ID); err != nil {
            return err
        }
        p.logger.Info("Task completed",
            zap.Int("worker", workerID),
            zap.Int64("task_id", task.ID),
            zap.Duration("took", processingTime),
        )
    case <-ctx.Done():
        // Отмена — вернуть задачу в pending
        p.pool.Exec(ctx, "UPDATE tasks SET status='pending' WHERE id=$1", task.ID)
        return ctx.Err()
    }

    return nil
}

func (p *Pool) claimTask(ctx context.Context) (model.Task, error) {
    var t model.Task
    
    err := p.pool.QueryRow(ctx, `
        WITH claimed AS (
            SELECT id
            FROM tasks
            WHERE status = 'pending'
            ORDER BY priority DESC, created_at
            FOR UPDATE SKIP LOCKED
            LIMIT 1
        )
        UPDATE tasks
        SET status = 'processing', updated_at = now()
        FROM claimed
        WHERE tasks.id = claimed.id
        RETURNING tasks.id, tasks.title, tasks.status, tasks.priority, 
                  tasks.version, tasks.created_at, tasks.updated_at
    `).Scan(&t.ID, &t.Title, &t.Status, &t.Priority, &t.Version, &t.CreatedAt, &t.UpdatedAt)

    return t, err
}

func (p *Pool) completeTask(ctx context.Context, id int64) error {
    _, err := p.pool.Exec(ctx, `
        UPDATE tasks SET status = 'completed', updated_at = now() WHERE id = $1
    `, id)
    return err
}
