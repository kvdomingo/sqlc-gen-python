-- name: ListAuthors :many
SELECT * FROM authors;

-- name: CountAuthors :one
SELECT count(*) FROM authors;
