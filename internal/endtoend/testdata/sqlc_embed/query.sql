-- name: GetBookWithAuthor :one
SELECT sqlc.embed(authors), books.title
FROM authors JOIN books ON books.author_id = authors.id
WHERE books.id = $1;

-- name: ListAuthorsAndBooks :many
SELECT sqlc.embed(authors), sqlc.embed(books)
FROM authors JOIN books ON books.author_id = authors.id;

-- name: GetAuthorOnly :one
SELECT sqlc.embed(authors) FROM authors WHERE id = $1;

-- name: ListTitlesWithAuthor :many
SELECT books.id, sqlc.embed(authors), books.title
FROM books JOIN authors ON books.author_id = authors.id;
