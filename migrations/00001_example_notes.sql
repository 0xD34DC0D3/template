-- +goose Up
-- EXAMPLE migration for the template. Before the first production deploy, a
-- new app deletes it (with its queries) and adds its real schema; after that,
-- drop the table in a new migration instead. Nothing should depend on it.
CREATE TABLE notes (
    id         INTEGER PRIMARY KEY,
    body       TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- +goose Down
DROP TABLE notes;
