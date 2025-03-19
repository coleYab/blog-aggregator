-- +goose Up

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE TABLE users (
    id UUID primary key default gen_random_uuid(),
    created_at TIMESTAMP default CURRENT_TIMESTAMP,
    updated_at TIMESTAMP default CURRENT_TIMESTAMP,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
