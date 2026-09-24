-- name: GetAuthorsByID :batchone
SELECT * FROM authors WHERE id = $1;

-- name: GetAuthorNames :batchone
SELECT name FROM authors WHERE id = $1;

-- name: ListBooksByAuthor :batchmany
SELECT * FROM books WHERE author_id = $1;

-- name: ListBookTitles :batchmany
SELECT id, title FROM books WHERE author_id = $1 AND title LIKE $2;
