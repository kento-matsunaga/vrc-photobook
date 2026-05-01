// Package wireup は Photobook 集約の HTTP handler を組み立てるための facade。
//
// 配置の理由（Go internal ルール）:
//   - cmd/api/main.go は internal/photobook/internal/usecase を直接 import できない
//   - 本パッケージは photobook サブツリー内に居住し、internal/usecase + repository +
//     session_adapter を組み合わせて Handlers を返す
//   - main.go は本パッケージを 1 つ呼ぶだけで token 交換 endpoint の依存ツリーが揃う
//
// 拡張時の指針:
//   - publish / reissue / その他の Photobook UseCase 用 handler が増えたら、本パッケージで
//     一括して組み立てる
package wireup

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	openingstyle "vrcpb/backend/internal/photobook/domain/vo/opening_style"
	pblayout "vrcpb/backend/internal/photobook/domain/vo/photobook_layout"
	pbtype "vrcpb/backend/internal/photobook/domain/vo/photobook_type"
	pbvisibility "vrcpb/backend/internal/photobook/domain/vo/visibility"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/infrastructure/session_adapter"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
	"vrcpb/backend/internal/turnstile"
	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

// BuildHandlers は pool / TTL から Photobook 集約の HTTP Handlers を組み立てる。
//
// pool は本番では *pgxpool.Pool。manageSessionTTL は manage session の TTL（7 日想定）。
// clock は nil で SystemClock が使われる。
func BuildHandlers(
	pool *pgxpool.Pool,
	manageSessionTTL time.Duration,
	clock photobookhttp.Clock,
) *photobookhttp.Handlers {
	repo := photobookrdb.NewPhotobookRepository(pool)
	draftIssuer := session_adapter.NewDraftIssuer(pool)
	manageIssuer := session_adapter.NewManageIssuer(pool)

	draftExchange := usecase.NewExchangeDraftTokenForSession(repo, draftIssuer)
	manageExchange := usecase.NewExchangeManageTokenForSession(repo, manageIssuer)

	return photobookhttp.NewHandlers(draftExchange, manageExchange, manageSessionTTL, clock)
}

// BuildPublicHandlers は公開 Viewer 用の HTTP Handlers を組み立てる（PR25a）。
//
// r2Client は presigned GET URL 発行に使う。pool が nil / r2Client が nil の場合は nil を返す
// （main.go 側で endpoint 自体を登録しない判断）。
func BuildPublicHandlers(pool *pgxpool.Pool, r2Client r2.Client) *photobookhttp.PublicHandlers {
	if pool == nil || r2Client == nil {
		return nil
	}
	uc := usecase.NewGetPublicPhotobook(pool, r2Client)
	return photobookhttp.NewPublicHandlers(uc)
}

// BuildManageReadHandlers は管理ページ用の HTTP Handlers を組み立てる（PR25a）。
//
// pool が nil の場合は nil を返す。
func BuildManageReadHandlers(pool *pgxpool.Pool) *photobookhttp.ManageHandlers {
	if pool == nil {
		return nil
	}
	uc := usecase.NewGetManagePhotobook(pool)
	return photobookhttp.NewManageHandlers(uc)
}

// BuildPublishHandlers は publish 用 HTTP Handlers を組み立てる（PR28）。
//
// pool が nil なら nil を返す。
//
// PR36: ipHashSalt（REPORT_IP_HASH_SALT_V1 流用）と usage UseCase を渡すと
// 1 時間 5 冊の publish UsageLimit が有効化される（業務知識 v4 §3.7）。
// 空文字 / nil なら UsageLimit を skip。
func BuildPublishHandlers(
	pool *pgxpool.Pool,
	usage *usagelimitwireup.Check,
	ipHashSalt string,
) *photobookhttp.PublishHandlers {
	if pool == nil {
		return nil
	}
	return photobookhttp.NewPublishHandlers(BuildPublishFromDraft(pool, usage), ipHashSalt)
}

// BuildCreateDraftPhotobook は CreateDraftPhotobook UseCase を組み立てる（CLI / batch
// 用途で再利用できるように export）。
func BuildCreateDraftPhotobook(pool *pgxpool.Pool) *usecase.CreateDraftPhotobook {
	repo := photobookrdb.NewPhotobookRepository(pool)
	return usecase.NewCreateDraftPhotobook(repo)
}

// BuildPublishFromDraft は PublishFromDraft UseCase を組み立てる（HTTP handler 用 +
// CLI / batch 用に再利用できるよう export）。
//
// PR36: usage が nil なら UsageLimit 連動を skip（旧互換 + test 用）。
func BuildPublishFromDraft(pool *pgxpool.Pool, usage *usagelimitwireup.Check) *usecase.PublishFromDraft {
	return usecase.NewPublishFromDraft(
		pool,
		session_adapter.NewPhotobookTxRepositoryFactory(),
		session_adapter.NewDraftRevokerFactory(),
		usecase.NewMinimalSlugGenerator(),
		usage,
	)
}

// CreateAndPublishCLIInput は CLI / batch 経由で publish-ready な photobook を 1 件
// 作成するときの入力。HTTP layer を経由せず UseCase を直接呼ぶ用途。
type CreateAndPublishCLIInput struct {
	Type               pbtype.PhotobookType
	Title              string
	Layout             pblayout.PhotobookLayout
	OpeningStyle       openingstyle.OpeningStyle
	Visibility         pbvisibility.Visibility
	CreatorDisplayName string
	RightsAgreed       bool
	Now                time.Time
}

// CreateAndPublishCLIOutput は CLI に返す最小サマリ。raw token は含めない。
type CreateAndPublishCLIOutput struct {
	PhotobookID        string
	Slug               string
	OutboxPendingCount int
}

// CreateAndPublishForCLI は draft 作成 → publish を 1 関数で実行する（CLI / batch 用、
// 例: 検証用に公開済 photobook を 1 件用意したいとき）。
//
// 通常運用では HTTP / UseCase を別々に呼ぶが、本 helper は UseCase を直接呼んで
// outbox event INSERT を含む正規パイプラインを **1 プロセス内で完結**させる。
// raw token はこの helper 内で使い切り、戻り値には含めない。
func CreateAndPublishForCLI(
	ctx context.Context,
	pool *pgxpool.Pool,
	in CreateAndPublishCLIInput,
) (CreateAndPublishCLIOutput, error) {
	createUC := BuildCreateDraftPhotobook(pool)
	createOut, err := createUC.Execute(ctx, usecase.CreateDraftPhotobookInput{
		Type:               in.Type,
		Title:              in.Title,
		Layout:             in.Layout,
		OpeningStyle:       in.OpeningStyle,
		Visibility:         in.Visibility,
		CreatorDisplayName: in.CreatorDisplayName,
		RightsAgreed:       in.RightsAgreed,
		Now:                in.Now,
	})
	if err != nil {
		return CreateAndPublishCLIOutput{}, err
	}
	pid := createOut.Photobook.ID()
	_ = createOut.RawDraftToken // 破棄

	// CLI 経路は UsageLimit の対象外（運営/admin による作成）
	publishUC := BuildPublishFromDraft(pool, nil)
	publishOut, err := publishUC.Execute(ctx, usecase.PublishFromDraftInput{
		PhotobookID:     pid,
		ExpectedVersion: 0,
		Now:             in.Now,
	})
	if err != nil {
		return CreateAndPublishCLIOutput{}, err
	}
	_ = publishOut.RawManageToken // 破棄

	slug := ""
	if s := publishOut.Photobook.PublicUrlSlug(); s != nil {
		slug = s.String()
	}

	var outboxCount int
	if err := pool.QueryRow(ctx,
		"SELECT count(*)::int FROM outbox_events WHERE aggregate_id=$1::uuid AND event_type='photobook.published' AND status='pending'",
		pid.String(),
	).Scan(&outboxCount); err != nil {
		return CreateAndPublishCLIOutput{}, err
	}

	return CreateAndPublishCLIOutput{
		PhotobookID:        pid.String(),
		Slug:               slug,
		OutboxPendingCount: outboxCount,
	}, nil
}

// BuildCreateHandlers は POST /api/photobooks 用の HTTP Handlers を組み立てる
// （作成導線追加 PR、docs/plan/m2-create-entry-plan.md）。
//
// pool / verifier のいずれかが nil なら nil を返す（main.go 側で endpoint を登録しない判断）。
// turnstileAction は "photobook-create" を hardcode で渡す（env 変更不要、既存
// TURNSTILE_SECRET_KEY 流用、Cloudflare ダッシュボード変更不要）。
func BuildCreateHandlers(
	pool *pgxpool.Pool,
	verifier turnstile.Verifier,
	turnstileHostname string,
	turnstileAction string,
) *photobookhttp.CreateHandlers {
	if pool == nil || verifier == nil {
		return nil
	}
	repo := photobookrdb.NewPhotobookRepository(pool)
	createUC := usecase.NewCreateDraftPhotobook(repo)
	return photobookhttp.NewCreateHandlers(createUC, verifier, turnstileHostname, turnstileAction, photobookhttp.SystemClock{})
}

// BuildEditHandlers は編集 UI 本格化（PR27）用の HTTP Handlers を組み立てる。
//
// r2Client は edit-view の display/thumbnail presigned URL 発行に必要。
// pool / r2Client が nil なら nil を返す（main.go 側で endpoint を登録しない判断）。
func BuildEditHandlers(pool *pgxpool.Pool, r2Client r2.Client) *photobookhttp.EditHandlers {
	if pool == nil || r2Client == nil {
		return nil
	}
	return photobookhttp.NewEditHandlers(
		usecase.NewGetEditView(pool, r2Client),
		usecase.NewUpdatePhotoCaption(pool),
		usecase.NewBulkReorderPhotosOnPage(pool),
		usecase.NewUpdatePhotobookSettings(pool),
		usecase.NewAddPage(pool),
		usecase.NewRemovePage(pool),
		usecase.NewRemovePhoto(pool),
		usecase.NewSetCoverImage(pool),
		usecase.NewClearCoverImage(pool),
	)
}
