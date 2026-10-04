-- name: GetAuthor :one
SELECT * FROM authors WHERE id = $1;

-- name: ListTaskTitles :many
SELECT id, title FROM tasks WHERE priority = $1;
