-- M1 spike: sqlc 生成元クエリの最小サンプル。本実装では使わない。

-- name: GetLatestAlive :one
SELECT id, note, created_at
FROM _test_alive
ORDER BY id DESC
LIMIT 1;

-- name: InsertAlive :one
INSERT INTO _test_alive (note)
VALUES ($1)
RETURNING id, note, created_at;
