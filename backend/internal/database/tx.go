// Package database には pgx ベースの DB アクセス共通ユーティリティを置く。
//
// 本ファイルは TX を 1 トランザクション内で実行するための WithTx ヘルパを提供する。
//
// 設計参照:
//   - docs/plan/m2-photobook-session-integration-plan.md §6.2
package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTx は pool から TX を 1 つ開始し、fn に渡して実行する。
//
// fn が nil error を返した場合は Commit、それ以外は Rollback する。
// Commit 失敗もエラーとして返す。defer Rollback は冪等で、Commit 後に呼ばれても無害。
//
// セキュリティ:
//   - 引数 / 返却値・ログには raw token / hash / Cookie / DSN を含めない
//   - Rollback 失敗は err を join して返す（標準 errors.Join）。詳細は呼び出し元に委ねる
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	if pool == nil {
		return errors.New("WithTx: pool is nil")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// fn 成功で Commit、失敗で Rollback。
	// defer Rollback は Commit 後に呼ばれても pgx がエラーを返すだけで副作用はないため安全。
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
