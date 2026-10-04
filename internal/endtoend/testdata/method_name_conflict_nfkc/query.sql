-- name: GetIs :one
SELECT name FROM things WHERE id = $1;

-- name: GetIſ :one
SELECT id FROM things WHERE id = $1;
