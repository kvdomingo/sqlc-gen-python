-- name: GetUser :one
SELECT name FROM things WHERE id = $1;

-- name: List :many
SELECT name FROM things;
