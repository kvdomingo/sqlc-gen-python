-- name: GetPost :one
SELECT * FROM posts WHERE id = $1;

-- name: ListPosts :many
SELECT * FROM posts ORDER BY id;

-- name: ListTitles :many
SELECT id, title FROM posts WHERE status = $1;

-- name: CreatePost :one
INSERT INTO posts (title, tags, status, created_at) VALUES ($1, $2, $3, $4)
RETURNING *;
