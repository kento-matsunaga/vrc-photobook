// Package db は PostgreSQL 接続プールの最小ラッパー。
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool は DSN から pgx の接続プールを作る。
// dsn が空文字の場合は nil を返す（PoC では DB 未設定でもサーバ起動を続行する想定）。
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, nil
	}
	return pgxpool.New(ctx, dsn)
}
