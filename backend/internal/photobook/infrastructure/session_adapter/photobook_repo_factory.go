package session_adapter

import (
	"github.com/jackc/pgx/v5"

	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// NewPhotobookTxRepositoryFactory は pgx.Tx 起点で usecase.PhotobookTxRepository を作る factory を返す。
//
// 本ファクトリは Publish / Reissue UseCase の WithTx 内で使い、Photobook の状態変更と
// Session revoke を同じ tx で実行するための入口になる。
func NewPhotobookTxRepositoryFactory() usecase.PhotobookTxRepositoryFactory {
	return func(tx pgx.Tx) usecase.PhotobookTxRepository {
		return rdb.NewPhotobookRepository(tx)
	}
}
