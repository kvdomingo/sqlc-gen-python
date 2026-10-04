-- name: GetPerson :one
SELECT * FROM people WHERE id = $1;

-- name: GetArtist :one
SELECT * FROM artists WHERE spotify_url = $1;

-- name: ListArtistLinks :many
SELECT id, spotify_url FROM artists WHERE kind = $1;
