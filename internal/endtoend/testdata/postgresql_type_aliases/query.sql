-- name: GetEverything :one
SELECT * FROM everything WHERE id = $1;

-- name: CastAliases :one
SELECT
  $1::varchar AS v,
  $2::character varying AS cv,
  $3::bpchar AS bp,
  $4::name AS n,
  $5::time AS t,
  $6::timetz AS ttz,
  $7::timestamp AS ts,
  $8::timestamptz AS tstz,
  $9::pg_catalog.json AS j,
  $10::pg_catalog.jsonb AS jb;
