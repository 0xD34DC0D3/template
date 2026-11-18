-- EXAMPLE queries against the example notes table; a new app replaces them.

-- name: CreateNote :one
INSERT INTO notes (body) VALUES (?) RETURNING *;

-- name: ListNotes :many
SELECT * FROM notes ORDER BY id DESC LIMIT ?;

-- name: CountNotes :one
SELECT count(*) FROM notes;
