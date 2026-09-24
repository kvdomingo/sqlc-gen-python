-- name: GetGrid :one
SELECT * FROM grids WHERE id = $1;

-- name: CreateGrid :exec
INSERT INTO grids (matrix, cube, tags) VALUES ($1, $2, $3);
