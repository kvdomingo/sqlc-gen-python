-- name: GetIid :one
SELECT name FROM things WHERE id = $1;

-- name: GetIıd :one
SELECT id FROM things WHERE id = $1;
