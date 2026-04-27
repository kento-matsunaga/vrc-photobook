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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/infrastructure/session_adapter"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
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
