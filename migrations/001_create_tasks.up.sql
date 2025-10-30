CREATE TABLE IF NOT EXISTS tasks (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' 
        CHECK (status IN ('pending', 'processing', 'completed')),
    priority INT NOT NULL DEFAULT 5 
        CHECK (priority BETWEEN 1 AND 10),
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tasks_status_priority
    ON tasks(status, priority DESC, created_at);

CREATE INDEX idx_tasks_keyset
    ON tasks(created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key TEXT PRIMARY KEY,
    resource_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)
