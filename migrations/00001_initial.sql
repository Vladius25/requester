-- +goose Up
-- +goose StatementBegin
CREATE TYPE task_status AS ENUM ('new', 'in_process', 'done', 'error');
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status task_status NOT NULL DEFAULT 'new',
    method TEXT NOT NULL,
    url TEXT NOT NULL,
    headers JSONB,
    body JSONB,
    response_status_code INTEGER,
    response_headers JSONB,
    response_content_length BIGINT
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE tasks;
DROP TYPE task_status;
-- +goose StatementEnd
