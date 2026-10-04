-- Create one author.
-- name: CreateAuthor :one
INSERT INTO authors (name, bio) VALUES ($1, $2) RETURNING *;

-- name: CreateAuthorAt :one
INSERT INTO authors (name, created_at) VALUES ($1, $2) RETURNING *;

-- name: ListAuthors :many
SELECT * FROM authors ORDER BY name;

-- name: CreateAuthors :copyfrom
INSERT INTO authors (name, bio) VALUES ($1, $2);

-- name: CreateBooks :batchexec
INSERT INTO books (author_id, title, status) VALUES ($1, $2, $3);

-- name: CreateBook :one
INSERT INTO books (author_id, title) VALUES ($1, $2) RETURNING *;

-- name: SetBookTitle :exec
UPDATE books SET title = $2 WHERE id = $1;

-- name: GetAuthorsByID :batchone
SELECT * FROM authors WHERE id = $1;

-- name: ListBooksByAuthor :batchmany
SELECT * FROM books WHERE author_id = $1 ORDER BY id;

-- name: GetBookWithAuthor :one
SELECT sqlc.embed(books), sqlc.embed(authors)
FROM books JOIN authors ON authors.id = books.author_id
WHERE books.id = $1;

-- name: InsertAuthorsReturning :batchone
INSERT INTO authors (name) VALUES ($1) RETURNING *;
