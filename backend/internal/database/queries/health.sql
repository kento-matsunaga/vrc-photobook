-- PR3 基盤確認用の最小 query。
-- sqlc generate が通ることの確認のみが目的で、業務的な意味はない。
-- 後続 PR で各集約ごとに query を分離する。

-- name: PingHealthCheck :one
SELECT 1::int AS ok;
