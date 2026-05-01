// Package http は HTTP ルーターを組み立てる。
//
// 配線:
//   - `/health` / `/readyz`（pool 状態に応じた分岐）
//   - 認可 token 交換: `/api/auth/draft-session-exchange` / `/api/auth/manage-session-exchange`
//   - 公開 Viewer: `/api/public/photobooks/{slug}`
//   - 管理ページ read: `/api/manage/photobooks/{id}/`（manage session middleware 経由）
//   - 編集 / 公開フロー: `/api/photobooks/{id}/...`（draft session middleware 経由）
//   - 画像アップロード: `/api/photobooks/{id}/images/...`（draft session middleware 経由）
//   - upload verification: `/api/photobooks/{id}/upload-verifications/`（draft session middleware 経由）
//   - CORS middleware（ALLOWED_ORIGINS 環境変数）を全体に適用
//
// 注意:
//   - protected route（/api/photobooks/{id}/images/... 等）は draft session middleware を経由する
//   - upload-intent は加えて Authorization: Bearer <upload_verification_token> が必須
//     （middleware ではなく handler 内で確認）
package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	authmiddleware "vrcpb/backend/internal/auth/session/middleware"
	"vrcpb/backend/internal/health"
	imageuploadhttp "vrcpb/backend/internal/imageupload/interface/http"
	ogphttp "vrcpb/backend/internal/ogp/interface/http"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	reporthttp "vrcpb/backend/internal/report/interface/http"
	uvhttp "vrcpb/backend/internal/uploadverification/interface/http"
)

// RouterConfig は NewRouter の依存集約。
type RouterConfig struct {
	Pool                       *pgxpool.Pool
	PhotobookHandlers          *photobookhttp.Handlers
	PhotobookPublicHandlers    *photobookhttp.PublicHandlers
	PhotobookManageHandlers    *photobookhttp.ManageHandlers
	PhotobookEditHandlers      *photobookhttp.EditHandlers
	PhotobookPublishHandlers   *photobookhttp.PublishHandlers
	PhotobookCreateHandlers    *photobookhttp.CreateHandlers
	OgpPublicHandlers          *ogphttp.PublicHandlers
	ImageUploadHandlers        *imageuploadhttp.Handlers
	UploadVerificationHandlers *uvhttp.Handlers
	ReportPublicHandlers       *reporthttp.PublicHandlers
	DraftSessionValidator      authmiddleware.Validator
	ManageSessionValidator     authmiddleware.Validator
	AllowedOrigins             string
}

// NewRouter は API サーバの chi ルーターを返す。
//
// pool は nil でも可（その場合 /readyz は 503 db_not_configured）。
// PhotobookHandlers が nil の場合は token 交換 endpoint を登録しない。
// ImageUploadHandlers / DraftSessionValidator が nil の場合は upload-intent / complete
// endpoint を登録しない（R2 Secret 未注入で上位が handler を組み立てなかった場合等）。
func NewRouter(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()
	if cfg.AllowedOrigins != "" {
		r.Use(NewCORS(cfg.AllowedOrigins))
	}
	r.Get("/health", health.Health)
	r.Get("/readyz", health.Ready(cfg.Pool))

	// token 交換 endpoint（draft / manage）。本物の token 検証経由のみ。
	if cfg.PhotobookHandlers != nil {
		r.Post("/api/auth/draft-session-exchange", cfg.PhotobookHandlers.DraftSessionExchange)
		r.Post("/api/auth/manage-session-exchange", cfg.PhotobookHandlers.ManageSessionExchange)
	}

	// 作成導線 endpoint（認可不要、Turnstile 必須、docs/plan/m2-create-entry-plan.md）。
	// Cookie / session middleware は経由しない。LP「今すぐ作る」→ /create → POST /api/photobooks
	// → response.draft_edit_token を Frontend が即消費して /draft/<token> に redirect。
	if cfg.PhotobookCreateHandlers != nil {
		r.Post("/api/photobooks", cfg.PhotobookCreateHandlers.CreatePhotobook)
	}

	// imageupload endpoint。draft session middleware を chain。
	if cfg.ImageUploadHandlers != nil && cfg.DraftSessionValidator != nil {
		r.Route("/api/photobooks/{id}/images", func(sub chi.Router) {
			sub.Use(authmiddleware.RequireDraftSession(cfg.DraftSessionValidator, photobookIDFromURL))
			sub.Post("/upload-intent", cfg.ImageUploadHandlers.UploadIntent)
			sub.Post("/{imageId}/complete", cfg.ImageUploadHandlers.Complete)
		})
	}

	// upload-verifications endpoint。draft session middleware を chain。
	if cfg.UploadVerificationHandlers != nil && cfg.DraftSessionValidator != nil {
		r.Route("/api/photobooks/{id}/upload-verifications", func(sub chi.Router) {
			sub.Use(authmiddleware.RequireDraftSession(cfg.DraftSessionValidator, photobookIDFromURL))
			sub.Post("/", cfg.UploadVerificationHandlers.IssueUploadVerification)
		})
	}

	// 公開 Viewer endpoint（認可不要、slug ベース）。
	if cfg.PhotobookPublicHandlers != nil {
		r.Get("/api/public/photobooks/{slug}", cfg.PhotobookPublicHandlers.GetPublicPhotobook)
	}

	// 通報受付 endpoint（認可不要、Turnstile 必須、PR35b）。
	// Cookie / session middleware は経由しない。
	if cfg.ReportPublicHandlers != nil {
		r.Post("/api/public/photobooks/{slug}/reports", cfg.ReportPublicHandlers.SubmitReport)
	}

	// OGP lookup endpoint（公開、photobook_id ベース）。Workers proxy / Frontend
	// generateMetadata から呼ばれる。draft / private / hidden / deleted は
	// status='not_public' を返し、image_url_path は default OGP に倒す。
	if cfg.OgpPublicHandlers != nil {
		r.Get("/api/public/photobooks/{photobookId}/ogp", cfg.OgpPublicHandlers.GetOgp)
	}

	// 管理ページ read endpoint。manage session middleware を chain。
	if cfg.PhotobookManageHandlers != nil && cfg.ManageSessionValidator != nil {
		r.Route("/api/manage/photobooks/{id}", func(sub chi.Router) {
			sub.Use(authmiddleware.RequireManageSession(cfg.ManageSessionValidator, photobookIDFromURL))
			sub.Get("/", cfg.PhotobookManageHandlers.GetManagePhotobook)
		})
	}

	// 編集 / publish endpoint。draft session middleware を chain（manage Cookie では不可）。
	if cfg.PhotobookEditHandlers != nil && cfg.DraftSessionValidator != nil {
		r.Route("/api/photobooks/{id}", func(sub chi.Router) {
			sub.Use(authmiddleware.RequireDraftSession(cfg.DraftSessionValidator, photobookIDFromURL))
			sub.Get("/edit-view", cfg.PhotobookEditHandlers.GetEditView)
			sub.Patch("/settings", cfg.PhotobookEditHandlers.UpdateSettings)
			sub.Post("/pages", cfg.PhotobookEditHandlers.AddPage)
			sub.Delete("/pages/{pageId}", cfg.PhotobookEditHandlers.RemovePage)
			sub.Patch("/photos/reorder", cfg.PhotobookEditHandlers.BulkReorderPhotos)
			sub.Patch("/photos/{photoId}/caption", cfg.PhotobookEditHandlers.UpdatePhotoCaption)
			sub.Delete("/photos/{photoId}", cfg.PhotobookEditHandlers.RemovePhoto)
			sub.Patch("/cover-image", cfg.PhotobookEditHandlers.SetCoverImage)
			sub.Delete("/cover-image", cfg.PhotobookEditHandlers.ClearCoverImage)
			if cfg.PhotobookPublishHandlers != nil {
				sub.Post("/publish", cfg.PhotobookPublishHandlers.Publish)
			}
		})
	}
	return r
}

// photobookIDFromURL は chi の URL param `{id}` を auth/session の photobook_id VO に変換する。
//
// session middleware は auth/session/domain/vo/photobook_id を期待する（独立 VO）。
// imageupload handler は photobook/domain/vo/photobook_id を使うため、handler 内で
// chi.URLParam から再 parse する（型は別物だが UUID 値は一致する）。
func photobookIDFromURL(r *http.Request) (photobook_id.PhotobookID, error) {
	raw := chi.URLParam(r, "id")
	if raw == "" {
		return photobook_id.PhotobookID{}, errors.New("missing photobook id in URL")
	}
	u, err := uuid.Parse(raw)
	if err != nil {
		return photobook_id.PhotobookID{}, err
	}
	return photobook_id.FromUUID(u)
}
