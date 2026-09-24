-- Insert an author.
-- name: CreateAuthor :one
INSERT INTO authors (name, bio) VALUES ($1, $2) RETURNING *;

-- name: ListAuthors :many
SELECT * FROM authors ORDER BY name;

-- name: GetAuthorsByID :batchone
SELECT * FROM authors WHERE id = $1;

-- name: DeleteAuthor :exec
DELETE FROM authors WHERE id = $1;
