// Package ogp_adapter は Photobook UseCase の OGP ports interface を、
// ogp 機構の ogpintegration 経由で実装する薄い adapter。
//
// 配置の理由:
//   - Photobook UseCase は ogp の internal/usecase を直接 import できない
//     （Go internal ルール）
//   - ogpintegration は ogp 配下の facade で、internal/usecase を呼べる
//   - 本 adapter は photobook 側に居住し、ogpintegration を呼ぶ「橋渡し」だけを行う
//   - 依存方向: photobook/usecase → photobook/infrastructure/ogp_adapter →
//                 ogp/ogpintegration → ogp/internal/usecase
//
// セキュリティ:
//   - raw storage_key / token / Cookie / Secret は本 adapter / ogpintegration ともに
//     戻り値 / log で漏らさない
//   - 失敗時の outcome は分類済 enum のみを返す
package ogp_adapter

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"vrcpb/backend/internal/ogp/ogpintegration"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// PendingEnsurer は usecase.OgpPendingEnsurerWithTx を実装する。
//
// publish UC の WithTx 内で呼ばれ、photobook_ogp_images の pending 行を冪等 INSERT する。
type PendingEnsurer struct{}

// NewPendingEnsurer は PendingEnsurer を作る。
func NewPendingEnsurer() *PendingEnsurer { return &PendingEnsurer{} }

// EnsurePendingWithTx は ogpintegration へ委譲する。
func (a *PendingEnsurer) EnsurePendingWithTx(
	ctx context.Context,
	tx pgx.Tx,
	photobookID photobook_id.PhotobookID,
	now time.Time,
) error {
	return ogpintegration.EnsureCreatedPendingWithTx(ctx, tx, photobookID.UUID(), now)
}

// SyncGenerator は usecase.OgpSyncGenerator を実装する。
//
// 内部に *ogpintegration.SyncGenerator を抱える。renderer は app 起動時に 1 度だけ
// 初期化され、複数 publish にわたって再利用される（embed font ロード負荷の回避）。
type SyncGenerator struct {
	inner *ogpintegration.SyncGenerator
}

// NewSyncGenerator は SyncGenerator を作る。
//
// inner が nil の場合は GenerateSync が常に "error" を返す safety fallback として
// 振る舞う（test や OGP 機能を一時 disable したい場合の用途）。
func NewSyncGenerator(inner *ogpintegration.SyncGenerator) *SyncGenerator {
	return &SyncGenerator{inner: inner}
}

// GenerateSync は ogpintegration へ委譲する。
//
// 呼び出し側は context.WithTimeout(2.5s) を渡す前提（ADR-0007 §3 (2)）。
// 戻り値は分類済 outcome のみ。失敗時も pending 行は維持され、outbox / scheduler の
// fallback で再試行される。
func (a *SyncGenerator) GenerateSync(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	now time.Time,
) usecase.OgpSyncOutcome {
	if a.inner == nil {
		return usecase.OgpSyncOutcomeError
	}
	return mapOutcome(a.inner.Generate(ctx, photobookID.UUID(), now))
}

// MapOutcome は ogpintegration.SyncOutcome を usecase.OgpSyncOutcome に変換する。
//
// 値域は 1:1 対応。未知 outcome は防御的に Error に倒す。
// 単体テストのため exported（同 package 外からも分類の対応関係を確認できるように）。
func MapOutcome(o ogpintegration.SyncOutcome) usecase.OgpSyncOutcome { return mapOutcome(o) }

// mapOutcome は ogpintegration.SyncOutcome を usecase.OgpSyncOutcome に変換する。
func mapOutcome(o ogpintegration.SyncOutcome) usecase.OgpSyncOutcome {
	switch o {
	case ogpintegration.SyncOutcomeSuccess:
		return usecase.OgpSyncOutcomeSuccess
	case ogpintegration.SyncOutcomeTimeout:
		return usecase.OgpSyncOutcomeTimeout
	case ogpintegration.SyncOutcomeNotPublished:
		return usecase.OgpSyncOutcomeNotPublished
	case ogpintegration.SyncOutcomePhotobookMissing:
		return usecase.OgpSyncOutcomePhotobookMissing
	default:
		return usecase.OgpSyncOutcomeError
	}
}
