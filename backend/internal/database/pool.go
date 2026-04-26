// Package database は PostgreSQL 接続プールの最小ラッパー。
//
// PR3: pgx/v5 で pool を作成する。DATABASE_URL が空ならば pool nil を返し、
// 呼び出し側（/readyz / 各 Repository）が nil を受けて 503 を返す設計に揃える。
//
// セキュリティ:
//   - DSN（DATABASE_URL の値全体）はログに出さない
//   - 接続失敗時のエラーメッセージはサーバ側 slog でのみ追跡し、クライアントには分類キーのみ返す
package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool は DSN から pgx の接続プールを作る。
// dsn が空文字の場合は (nil, nil) を返す（PR3 段階では DB 未設定でも起動継続を許容）。
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, nil
	}
	return pgxpool.New(ctx, dsn)
}
