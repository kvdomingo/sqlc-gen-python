-- name: GetAuthor :one
SELECT * FROM authors WHERE id = $1;

-- name: ListAuthors :many
SELECT * FROM authors ORDER BY name;

-- name: DeleteAuthor :execrows
DELETE FROM authors WHERE id = $1;
