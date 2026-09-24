-- name: GetMemberByEmail :one
SELECT * FROM members WHERE email = $1;
