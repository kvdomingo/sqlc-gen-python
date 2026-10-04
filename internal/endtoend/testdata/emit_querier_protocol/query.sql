-- name: GetAuthor :one
SELECT * FROM authors WHERE id = $1;

-- name: ListAuthors :many
SELECT * FROM authors ORDER BY name;

-- name: CreateAuthors :copyfrom
INSERT INTO authors (name, bio) VALUES ($1, $2);

-- name: GetAuthorsByID :batchone
SELECT * FROM authors WHERE id = $1;

-- name: ListBooksByAuthor :batchmany
SELECT * FROM books WHERE author_id = $1;

-- name: DeleteAuthor :exec
DELETE FROM authors WHERE id = $1;
