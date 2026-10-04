-- name: GetTrip :one
SELECT * FROM trips
WHERE id = $1 LIMIT 1;

-- name: ListTripsByClass :many
SELECT * FROM trips
WHERE class = $1;

-- name: ListDepartures :many
SELECT id, "from" FROM trips
WHERE class = sqlc.arg(class);

-- name: Import :exec
INSERT INTO trips ("from", "to", class) VALUES ($1, $2, $3);
