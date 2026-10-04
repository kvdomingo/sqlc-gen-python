-- Insert an author.
-- name: CreateAuthor :one
INSERT INTO authors (name, bio) VALUES ($1, $2) RETURNING *;

-- name: ListAuthors :many
SELECT * FROM authors ORDER BY name;

-- name: GetAuthorsByID :batchone
SELECT * FROM authors WHERE id = $1;

-- name: DeleteAuthor :exec
DELETE FROM authors WHERE id = $1;

-- name: AddJob :one
INSERT INTO jobs (errors, "cast") VALUES ($1, $2) RETURNING *;

-- name: ListJobsInRange :many
SELECT * FROM jobs WHERE id > $1 AND id < $2;
