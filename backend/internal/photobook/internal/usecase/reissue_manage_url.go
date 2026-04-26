package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// ErrReissueConflict は reissue 楽観ロック失敗（version 不一致 / status≠published・deleted）。
var ErrReissueConflict = errors.New("reissue conflict (version mismatch or not in published/deleted state)")

// ReissueManageUrlInput は manage URL 再発行の入力。
type ReissueManageUrlInput struct {
	PhotobookID     photobook_id.PhotobookID
	ExpectedVersion int
	Now             time.Time
}

// ReissueManageUrlOutput は reissue 結果。RawManageToken はログ禁止。
type ReissueManageUrlOutput struct {
	Photobook      domain.Photobook
	RawManageToken manage_url_token.ManageUrlToken
}

// ReissueManageUrl は manage_url_token を新規発行する UseCase。
//
// **同一 TX 内で**:
//  1. photobook を取得・状態検証（published / deleted）
//  2. oldVersion を保持
//  3. 新 ManageUrlToken / Hash 生成
//  4. photobookRepo.ReissueManageUrl（version=$expected で UPDATE、manage_url_token_version+=1）
//  5. session の RevokeAllManageByTokenVersion(photobook_id, oldVersion)（同 tx）
//
// session revoke 失敗時は photobook 側 UPDATE もロールバックされる（I-S10 整合）。
//
// ModerationAction / ManageUrlDelivery / Outbox INSERT は本 PR では行わない（後続 PR）。
type ReissueManageUrl struct {
	pool                 *pgxpool.Pool
	photobookRepoFactory PhotobookTxRepositoryFactory
	revokerFactory       ManageSessionRevokerFactory
}

// NewReissueManageUrl は UseCase を組み立てる。
func NewReissueManageUrl(
	pool *pgxpool.Pool,
	photobookRepoFactory PhotobookTxRepositoryFactory,
	revokerFactory ManageSessionRevokerFactory,
) *ReissueManageUrl {
	return &ReissueManageUrl{
		pool:                 pool,
		photobookRepoFactory: photobookRepoFactory,
		revokerFactory:       revokerFactory,
	}
}

// Execute は WithTx 内で reissue + RevokeAllManageByTokenVersion を原子的に実行する。
func (u *ReissueManageUrl) Execute(
	ctx context.Context,
	in ReissueManageUrlInput,
) (ReissueManageUrlOutput, error) {
	rawManage, err := manage_url_token.Generate()
	if err != nil {
		return ReissueManageUrlOutput{}, fmt.Errorf("manage token gen: %w", err)
	}
	newHash := manage_url_token_hash.Of(rawManage)

	var resultPB domain.Photobook
	err = database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := u.photobookRepoFactory(tx)
		revoker := u.revokerFactory(tx)

		pb, err := repo.FindByID(ctx, in.PhotobookID)
		if err != nil {
			if errors.Is(err, rdb.ErrNotFound) {
				return ErrReissueConflict
			}
			return err
		}
		// 状態検証は domain 側 ReissueManageUrl が ErrNotPublishedOrDeleted を返す
		if pb.Version() != in.ExpectedVersion {
			return ErrReissueConflict
		}
		next, oldVersionVO, err := pb.ReissueManageUrl(newHash, in.Now)
		if err != nil {
			if errors.Is(err, domain.ErrNotPublishedOrDeleted) {
				return ErrReissueConflict
			}
			return err
		}
		if err := repo.ReissueManageUrl(ctx, pb.ID(), newHash, in.ExpectedVersion); err != nil {
			if errors.Is(err, rdb.ErrOptimisticLockConflict) {
				return ErrReissueConflict
			}
			return err
		}
		if _, err := revoker.RevokeAllManageByTokenVersion(ctx, pb.ID(), oldVersionVO.Int()); err != nil {
			return fmt.Errorf("revoke all manage: %w", err)
		}
		resultPB = next
		return nil
	})
	if err != nil {
		return ReissueManageUrlOutput{}, err
	}
	return ReissueManageUrlOutput{Photobook: resultPB, RawManageToken: rawManage}, nil
}
