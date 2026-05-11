// Package ogpintegration は OGP 集約の操作を **ogp サブツリー外** から安全に呼び出せる
// ようにする薄い facade。
//
// 配置の理由（Go internal ルール）:
//   - usecase / repository / renderer は `internal/ogp/internal/...` 配下のため
//     ogp サブツリーからしか import できない
//   - Photobook 集約から OGP を呼ぶには、ogp 配下に公開窓口が必要
//   - ogpintegration は ogp サブツリー内なので internal を import 可能
//   - photobook 側は本パッケージだけを import すればよく、internal の中身を直接触れずに済む
//
// 提供する操作（ADR-0007 / docs/plan/m2-ogp-sync-publish-plan.md STOP β）:
//   - EnsureCreatedPendingWithTx : publish UC の WithTx 内で pending 行を冪等 INSERT
//   - SyncGenerator              : commit 後 best-effort 同期生成 (timeout 2.5s 想定)
//
// セキュリティ:
//   - raw storage_key / token / Cookie / Secret は log / 戻り値で漏らさない
//   - 失敗時の reason は ogp_failure_reason VO 経由で sanitize 済（renderer/UC が担保）
package ogpintegration

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/ogp/infrastructure/renderer"
	ogprdb "vrcpb/backend/internal/ogp/infrastructure/repository/rdb"
	ogpusecase "vrcpb/backend/internal/ogp/internal/usecase"
	photobookid "vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// EnsureCreatedPendingWithTx は tx 起点で photobook_ogp_images の pending 行を冪等 INSERT する。
//
// publish UC の WithTx 内で呼ぶ前提（ADR-0007 §3 (1)）:
//   - 既に worker 側で pending row を作成済の場合は ON CONFLICT DO NOTHING で素通り
//   - 新規 publish の場合は新しい pending row が作られる
//
// 引数の photobookUUID は外部の UUID。本関数内で ogp 機構の photobook_id VO に変換する。
func EnsureCreatedPendingWithTx(
	ctx context.Context,
	tx pgx.Tx,
	photobookUUID uuid.UUID,
	now time.Time,
) error {
	pid, err := photobookid.FromUUID(photobookUUID)
	if err != nil {
		return fmt.Errorf("photobook_id VO: %w", err)
	}
	repo := ogprdb.NewOgpRepository(tx)
	return repo.EnsureCreatedPending(ctx, pid, now)
}

// SyncOutcome は SyncGenerator.Generate の結果分類。
//
// slog の `outcome` field 値として直接使う（小文字 snake_case で固定）。
// 業務知識に紐づく分類のため、追加時は ADR-0007 / m2-ogp-sync-publish-plan.md と整合させる。
type SyncOutcome string

const (
	// SyncOutcomeSuccess は render + R2 PUT + MarkGenerated まで成功。
	SyncOutcomeSuccess SyncOutcome = "success"
	// SyncOutcomeTimeout は呼び出し側 context が timeout / cancel された。
	SyncOutcomeTimeout SyncOutcome = "timeout"
	// SyncOutcomeNotPublished は photobook が published でない / hidden 状態。
	// publish UC 直後はここに来ない想定だが、防御的に分類する。
	SyncOutcomeNotPublished SyncOutcome = "not_published"
	// SyncOutcomePhotobookMissing は photobook 自体が見つからない（極めて稀）。
	SyncOutcomePhotobookMissing SyncOutcome = "photobook_missing"
	// SyncOutcomeError はそれ以外（render / R2 PUT / DB 書き込み失敗 等）。
	// failure_reason 詳細は UseCase 内で MarkFailed の代わりに pending を維持する設計
	// (ADR-0007 §3 (2) sync 失敗時は pending を維持し worker fallback に委ねる)。
	SyncOutcomeError SyncOutcome = "error"
)

// String は SyncOutcome を string に変換する（slog 用）。
func (o SyncOutcome) String() string { return string(o) }

// SyncGenerator は publish commit 後の best-effort 同期生成器。
//
// 内部で renderer / fetcher を保持し、Generate() で 1 photobook の OGP 生成を実行する。
// renderer は embed font ロードのため、app 起動時に 1 度だけ生成する想定（wireup から
// NewSyncGenerator を呼ぶ）。
type SyncGenerator struct {
	pool       *pgxpool.Pool
	r2Client   r2.Client
	renderer   *renderer.Renderer
	bucketName string
	logger     *slog.Logger
}

// NewSyncGenerator は依存をまとめて SyncGenerator を返す。
//
// renderer の初期化（embed font ロード）はここで 1 度だけ行う。
func NewSyncGenerator(
	pool *pgxpool.Pool,
	r2Client r2.Client,
	bucketName string,
	logger *slog.Logger,
) (*SyncGenerator, error) {
	if logger == nil {
		logger = slog.Default()
	}
	rdr, err := renderer.New()
	if err != nil {
		return nil, fmt.Errorf("renderer init: %w", err)
	}
	return &SyncGenerator{
		pool:       pool,
		r2Client:   r2Client,
		renderer:   rdr,
		bucketName: bucketName,
		logger:     logger,
	}, nil
}

// Generate は 1 photobook の OGP 同期生成を best-effort で実行する。
//
// 呼び出し側は context.WithTimeout(2.5s) を渡すことを想定。
// 戻り値は分類済 outcome のみ（呼び出し側で slog 記録）。
// 失敗時も pending 行は維持され、outbox-worker / Cloud Scheduler の fallback
// 経路（ADR-0007 §3 (3)、Scheduler 整備は未確定）で再試行される。
func (g *SyncGenerator) Generate(
	ctx context.Context,
	photobookUUID uuid.UUID,
	now time.Time,
) SyncOutcome {
	pid, err := photobookid.FromUUID(photobookUUID)
	if err != nil {
		return SyncOutcomeError
	}
	fetcher := ogpusecase.NewPhotobookFetcherFromRdb(g.pool)
	uc := ogpusecase.NewGenerateOgpForPhotobook(g.pool, fetcher, g.r2Client, g.renderer, g.bucketName, g.logger)
	_, err = uc.Execute(ctx, ogpusecase.GenerateOgpInput{
		PhotobookID: pid,
		Now:         now,
	})
	return classifyOgpErr(err)
}

// ClassifyOgpErr は UseCase の error を SyncOutcome に分類する。
//
// 単体テストのため exported（同 package 外からも分類ロジックを確認できるように）。
func ClassifyOgpErr(err error) SyncOutcome {
	return classifyOgpErr(err)
}

// classifyOgpErr は UseCase の error を SyncOutcome に分類する。
func classifyOgpErr(err error) SyncOutcome {
	if err == nil {
		return SyncOutcomeSuccess
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return SyncOutcomeTimeout
	}
	if errors.Is(err, ogpusecase.ErrPhotobookNotFound) {
		return SyncOutcomePhotobookMissing
	}
	if errors.Is(err, ogpusecase.ErrNotPublished) {
		return SyncOutcomeNotPublished
	}
	return SyncOutcomeError
}
