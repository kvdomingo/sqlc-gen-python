-- Fetch an author by id.
-- name: GetAuthor :one
SELECT * FROM authors WHERE id = $1;

-- List every author.
-- Ordered by "name", with a \ backslash.
-- name: ListAuthors :many
SELECT * FROM authors ORDER BY name;

-- name: DeleteAuthor :exec
DELETE FROM authors WHERE id = $1;
