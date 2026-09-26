-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name text NOT NULL,
    auth_id text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
