-- name: CreateEvent :one
INSERT INTO events (payload, extra) VALUES ($1, $2) RETURNING *;
