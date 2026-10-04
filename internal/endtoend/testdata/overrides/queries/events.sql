-- name: CreateEvent :one
INSERT INTO events (payload, extra) VALUES ($1, $2) RETURNING *;

-- name: ListEventsByRank :many
SELECT * FROM events WHERE rank > $1 AND rank < $2;
