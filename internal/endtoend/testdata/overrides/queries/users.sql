-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: ListUsersByOrg :many
SELECT * FROM users WHERE org_id = $1;

-- name: GetAuthor :one
SELECT * FROM authors WHERE id = $1;
