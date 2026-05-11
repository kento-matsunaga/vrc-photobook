// Package usecase は Photobook 集約の UseCase を提供する。
//
// 主な UseCase:
//   - CreateDraftPhotobook
//   - TouchDraft
//   - ExchangeDraftTokenForSession / ExchangeManageTokenForSession（token → Cookie session 交換）
//   - PublishFromDraft（同一 TX 内で Session の RevokeAllDrafts と Outbox INSERT を行う）
//   - ReissueManageUrl（同一 TX 内で Session の RevokeAllManageByTokenVersion を呼ぶ）
//   - GetManagePhotobook / GetPublicPhotobook / GetEditView（read 系）
//   - 編集 UseCase（settings 更新 / page / photo / cover 操作）
//
// セキュリティ:
//   - raw DraftEditToken / ManageUrlToken / SessionToken は戻り値としてのみ取り扱い、
//     ログ・diff・テストログには出さない
//   - photobook 状態不一致 / token 不一致 / 期限切れ等の検証失敗は ErrInvalid* に
//     集約する（cause は error chain で保持）
package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
)

// PhotobookRepository は TX 不要の参照系 / draft INSERT / TouchDraft 用の Photobook 永続化操作。
//
// CreateDraft / FindBy* / TouchDraft は単一 SQL なので、UseCase 側で TX を張らず本 interface 経由で呼ぶ。
type PhotobookRepository interface {
	CreateDraft(ctx context.Context, pb domain.Photobook) error
	FindByID(ctx context.Context, id photobook_id.PhotobookID) (domain.Photobook, error)
	FindByDraftEditTokenHash(ctx context.Context, hash draft_edit_token_hash.DraftEditTokenHash) (domain.Photobook, error)
	FindByManageUrlTokenHash(ctx context.Context, hash manage_url_token_hash.ManageUrlTokenHash) (domain.Photobook, error)
	TouchDraft(ctx context.Context, id photobook_id.PhotobookID, newExpiresAt time.Time, expectedVersion int) error
}

// PhotobookTxRepository は WithTx 内で使う Photobook 永続化操作（UPDATE を含む）。
//
// PublishFromDraft / ReissueManageUrl は session revoke と同一 TX で実行する必要があるため、
// pgx.Tx 起点で生成された repository を引数に取る形にする。
type PhotobookTxRepository interface {
	FindByID(ctx context.Context, id photobook_id.PhotobookID) (domain.Photobook, error)
	PublishFromDraft(
		ctx context.Context,
		id photobook_id.PhotobookID,
		publicSlug slug.Slug,
		manageHash manage_url_token_hash.ManageUrlTokenHash,
		publishedAt time.Time,
		expectedVersion int,
	) error
	ReissueManageUrl(
		ctx context.Context,
		id photobook_id.PhotobookID,
		newManageHash manage_url_token_hash.ManageUrlTokenHash,
		expectedVersion int,
	) error
}

// PhotobookTxRepositoryFactory は pgx.Tx 起点で PhotobookTxRepository を作るファクトリ。
//
// 本番では infrastructure/session_adapter / repository から組み合わせて実装する。
// テストでは fake を返す。
type PhotobookTxRepositoryFactory func(tx pgx.Tx) PhotobookTxRepository

// === Session 連携 ports ===
//
// Photobook UseCase は本 interface 群を経由して Session 機構を呼ぶ。
// 実装は infrastructure/session_adapter/ で sessionintegration を呼ぶ薄い wrapper。

// DraftSessionIssuer は draft session を発行する。
//
// Exchange*ForSession から呼ばれる。Photobook 側の SELECT と session INSERT は
// 厳密な同一 TX を必須としない（photobook 側は変更しない）。
type DraftSessionIssuer interface {
	IssueDraft(
		ctx context.Context,
		photobookID photobook_id.PhotobookID,
		now time.Time,
		expiresAt time.Time,
	) (session_token.SessionToken, error)
}

// ManageSessionIssuer は manage session を発行する。
type ManageSessionIssuer interface {
	IssueManage(
		ctx context.Context,
		photobookID photobook_id.PhotobookID,
		tokenVersion int,
		now time.Time,
		expiresAt time.Time,
	) (session_token.SessionToken, error)
}

// DraftSessionRevoker は publish 時に draft session を一括 revoke する。
//
// pgx.Tx 起点で生成され、Photobook の状態変更と同じ TX で動く前提。
type DraftSessionRevoker interface {
	RevokeAllDrafts(ctx context.Context, photobookID photobook_id.PhotobookID) (int64, error)
}

// DraftSessionRevokerFactory は pgx.Tx 起点で DraftSessionRevoker を作るファクトリ。
type DraftSessionRevokerFactory func(tx pgx.Tx) DraftSessionRevoker

// ManageSessionRevoker は reissueManageUrl 時に旧 version 以下の manage session を
// 一括 revoke する。
type ManageSessionRevoker interface {
	RevokeAllManageByTokenVersion(
		ctx context.Context,
		photobookID photobook_id.PhotobookID,
		oldVersion int,
	) (int64, error)
}

// CurrentSessionRevoker は単一 session を session_id 指定で revoke する。
//
// M-1a: /api/manage/photobooks/{id}/session-revoke から、middleware が context に
// セットした現在 Cookie session の id を渡して破棄する。raw token / 元 manage_url_token
// には影響しない（設計書 §3.3）。
type CurrentSessionRevoker interface {
	RevokeOne(ctx context.Context, sessionID uuid.UUID) error
}

// ManageSessionRevokerFactory は pgx.Tx 起点で ManageSessionRevoker を作るファクトリ。
type ManageSessionRevokerFactory func(tx pgx.Tx) ManageSessionRevoker

// SlugGenerator は publish 時に public_url_slug を生成する。
//
// MVP 実装は usecase.MinimalSlugGenerator（crypto/rand から 12 文字の英数を作る）。
// 衝突検出・retry の高度化は MVP 範囲外（衝突発生時は publish handler が 409 で返す）。
type SlugGenerator interface {
	Generate(ctx context.Context) (slug.Slug, error)
}

// === OGP 連携 ports（M-2 同期 publish / ADR-0007） ===
//
// publish UC は本 interface 群を経由して OGP 機構を呼ぶ。
// 実装は infrastructure/ogp_adapter/ で ogpintegration を呼ぶ薄い wrapper。

// OgpPendingEnsurerWithTx は publish UC の WithTx 内で photobook_ogp_images の
// pending 行を冪等 INSERT する（ON CONFLICT DO NOTHING）。
//
// ADR-0007 §3 (1): publish と pending 行の存在を同一 TX で保証することで、
// 1st X crawler hit 時に row が無く `/og/default.png` redirect される事故を防ぐ。
type OgpPendingEnsurerWithTx interface {
	EnsurePendingWithTx(
		ctx context.Context,
		tx pgx.Tx,
		photobookID photobook_id.PhotobookID,
		now time.Time,
	) error
}

// OgpSyncOutcome は commit 後 sync の結果分類（log 用）。
//
// 値は ogpintegration.SyncOutcome と 1:1 対応するが、photobook 集約から ogp の型を
// 直接参照しないため string 表現でやり取りする（小文字 snake_case 固定）。
type OgpSyncOutcome string

const (
	OgpSyncOutcomeSuccess          OgpSyncOutcome = "success"
	OgpSyncOutcomeTimeout          OgpSyncOutcome = "timeout"
	OgpSyncOutcomeNotPublished     OgpSyncOutcome = "not_published"
	OgpSyncOutcomePhotobookMissing OgpSyncOutcome = "photobook_missing"
	OgpSyncOutcomeError            OgpSyncOutcome = "error"
)

// OgpSyncGenerator は publish commit 後の best-effort 同期生成を実行する。
//
// 呼び出し側は context.WithTimeout(2.5s) を渡す。失敗時も pending 行は維持され、
// outbox-worker / Cloud Scheduler の fallback 経路（ADR-0007 §3 (2) (3)、
// Scheduler 整備は未確定）で再試行される。
type OgpSyncGenerator interface {
	GenerateSync(
		ctx context.Context,
		photobookID photobook_id.PhotobookID,
		now time.Time,
	) OgpSyncOutcome
}
