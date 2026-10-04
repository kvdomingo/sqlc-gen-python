-- name: getUser :one
SELECT id FROM things WHERE id = $1;

-- name: List_ :many
SELECT id FROM things;
