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

-- name: ProbeJobs :many
SELECT id, id_2, id, list, dict FROM jobs WHERE list = $1 AND id > $2 AND id < $3 AND id_2 = $4;

-- name: ListJobsByDict :many
SELECT * FROM jobs WHERE dict = $1;
