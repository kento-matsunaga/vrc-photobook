// Package http は HTTP ルーターを組み立てる。
//
// PR2: `/health` + `/readyz`（DB 未実装時 503 固定）。
// PR3: `/readyz` を pool 状態に応じた分岐に置き換え。
// PR8: Session 認可 middleware と UseCase 枠を用意（**未接続のまま**）。
// PR9c: Photobook の token 交換 endpoint 2 本（draft / manage）を追加。
//       本物の token 検証経由のみ。dummy token / 認証バイパスは作らない。
// PR21: imageupload の upload-intent / complete endpoint を追加。
//       draft session middleware で認可された context を前提に handler が動く。
//       CORS middleware を導入（ALLOWED_ORIGINS 環境変数）。
//
// 注意:
//   - protected route（/api/photobooks/{id}/images/...）は draft session middleware を経由する
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
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
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
	ImageUploadHandlers        *imageuploadhttp.Handlers
	UploadVerificationHandlers *uvhttp.Handlers
	DraftSessionValidator      authmiddleware.Validator
	ManageSessionValidator     authmiddleware.Validator
	AllowedOrigins             string
}

// NewRouter は API サーバの chi ルーターを返す。
//
// pool は nil でも可（その場合 /readyz は 503 db_not_configured）。
// PhotobookHandlers が nil の場合は token 交換 endpoint を登録しない。
// ImageUploadHandlers / DraftSessionValidator が nil の場合は upload-intent / complete
// endpoint を登録しない（PR21 Step A 段階の R2 Secret 未注入時など）。
func NewRouter(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()
	if cfg.AllowedOrigins != "" {
		r.Use(NewCORS(cfg.AllowedOrigins))
	}
	r.Get("/health", health.Health)
	r.Get("/readyz", health.Ready(cfg.Pool))

	// PR9c: token 交換 endpoint。
	if cfg.PhotobookHandlers != nil {
		r.Post("/api/auth/draft-session-exchange", cfg.PhotobookHandlers.DraftSessionExchange)
		r.Post("/api/auth/manage-session-exchange", cfg.PhotobookHandlers.ManageSessionExchange)
	}

	// PR21: imageupload endpoint。draft session middleware を chain。
	if cfg.ImageUploadHandlers != nil && cfg.DraftSessionValidator != nil {
		r.Route("/api/photobooks/{id}/images", func(sub chi.Router) {
			sub.Use(authmiddleware.RequireDraftSession(cfg.DraftSessionValidator, photobookIDFromURL))
			sub.Post("/upload-intent", cfg.ImageUploadHandlers.UploadIntent)
			sub.Post("/{imageId}/complete", cfg.ImageUploadHandlers.Complete)
		})
	}

	// PR22: upload-verifications endpoint。draft session middleware を chain。
	if cfg.UploadVerificationHandlers != nil && cfg.DraftSessionValidator != nil {
		r.Route("/api/photobooks/{id}/upload-verifications", func(sub chi.Router) {
			sub.Use(authmiddleware.RequireDraftSession(cfg.DraftSessionValidator, photobookIDFromURL))
			sub.Post("/", cfg.UploadVerificationHandlers.IssueUploadVerification)
		})
	}

	// PR25a: 公開 Viewer endpoint（認可不要、slug ベース）。
	if cfg.PhotobookPublicHandlers != nil {
		r.Get("/api/public/photobooks/{slug}", cfg.PhotobookPublicHandlers.GetPublicPhotobook)
	}

	// PR25a: 管理ページ read endpoint。manage session middleware を chain。
	if cfg.PhotobookManageHandlers != nil && cfg.ManageSessionValidator != nil {
		r.Route("/api/manage/photobooks/{id}", func(sub chi.Router) {
			sub.Use(authmiddleware.RequireManageSession(cfg.ManageSessionValidator, photobookIDFromURL))
			sub.Get("/", cfg.PhotobookManageHandlers.GetManagePhotobook)
		})
	}

	// PR27: 編集 UI 本格化 endpoint。draft session middleware を chain（manage Cookie では不可）。
	// PR28: publish endpoint も同 group に追加（draft Cookie 必須）。
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
