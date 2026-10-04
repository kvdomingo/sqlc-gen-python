-- name: ListEventsSince :many
SELECT * FROM events WHERE a > $1;

-- name: GetLatest :one
SELECT max(a)::timestamptz AS latest FROM events;
