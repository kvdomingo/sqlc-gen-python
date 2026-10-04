-- name: ListRulesByOp :many
SELECT * FROM rules
WHERE op = $1;
