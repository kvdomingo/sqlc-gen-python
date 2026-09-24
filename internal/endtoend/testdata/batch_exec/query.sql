-- name: CreateAuthors :batchexec
INSERT INTO authors (name, bio) VALUES ($1, $2);

-- name: DeleteAuthors :batchexec
DELETE FROM authors WHERE id = $1;
