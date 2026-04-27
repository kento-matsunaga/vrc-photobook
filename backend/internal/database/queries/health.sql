-- 基盤確認用の最小 query（sqlc generate が通ることの確認のみが目的で、業務的な意味はない）。
-- 集約別の query は sqlc.yaml の他 set（auth/session / photobook / image / uploadverification /
-- outbox）に分離されている。本 set は health 単独。

-- name: PingHealthCheck :one
SELECT 1::int AS ok;
