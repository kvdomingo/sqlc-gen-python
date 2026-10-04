-- Create one author.
-- name: CreateAuthor :one
INSERT INTO authors (name, bio) VALUES ($1, $2) RETURNING *;

-- name: GetAuthor :one
SELECT * FROM authors WHERE id = $1;

-- name: ListAuthors :many
SELECT * FROM authors ORDER BY name;

-- name: DeleteAuthor :execrows
DELETE FROM authors WHERE id = $1;

-- name: CreateAuthors :copyfrom
INSERT INTO authors (name, bio) VALUES ($1, $2);

-- name: CreateBooks :batchexec
INSERT INTO books (author_id, title, status, "from") VALUES ($1, $2, $3, $4);

-- Look up several authors at once.
-- name: GetAuthorsByID :batchone
SELECT * FROM authors WHERE id = $1;

-- name: ListBooksByAuthor :batchmany
SELECT * FROM books WHERE author_id = $1 ORDER BY id;

-- name: GetBookWithAuthor :one
SELECT sqlc.embed(books), sqlc.embed(authors)
FROM books JOIN authors ON authors.id = books.author_id
WHERE books.id = $1;

-- name: ListBooksByStatus :many
SELECT books.id, books.title, books."from", authors.name AS author_name
FROM books JOIN authors ON authors.id = books.author_id
WHERE books.status = $1 AND books.author_id = sqlc.arg(class)::bigint;

-- name: CountAuthors :one
SELECT count(*) FROM authors;

-- name: List :many
SELECT tags FROM books;

-- name: Int :execrows
DELETE FROM books WHERE id = $1;

-- name: Models :one
SELECT * FROM authors WHERE id = $1;

-- name: ListTags :many
SELECT tags FROM books WHERE id = $1;

-- name: PickAuthor :one
SELECT * FROM authors WHERE name = $1;
