// publish_handler: draft → published 遷移の HTTP handler（PR28）。
//
// 設計参照:
//   - docs/plan/m2-frontend-edit-ui-fullspec-plan.md §10
//   - 業務知識 v4 §6 manage URL（再表示禁止）
//
// 仕様:
//   - POST /api/photobooks/{id}/publish
//   - draft Cookie 必須（router 側で middleware 適用）
//   - status='draft' AND version=$expected で OCC、0 行は 409 version_conflict
//   - response: { photobook_id, slug, public_url_path, manage_url_path, published_at }
//     manage_url_path に **raw token** を含む（再表示しないため、UI が即座にユーザーに見せて
//     コピーを促す）。response body 経由でのみ伝送し、log / Set-Cookie / DB 永続化はしない
//   - Cache-Control: no-store / X-Robots-Tag: noindex,nofollow
package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"vrcpb/backend/internal/photobook/domain"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// PublishHandlers は publish endpoint の HTTP handler。
type PublishHandlers struct {
	publish *usecase.PublishFromDraft
}

// NewPublishHandlers は PublishHandlers を組み立てる。
func NewPublishHandlers(publish *usecase.PublishFromDraft) *PublishHandlers {
	return &PublishHandlers{publish: publish}
}

type publishRequest struct {
	ExpectedVersion int `json:"expected_version"`
}

type publishResponse struct {
	PhotobookID    string    `json:"photobook_id"`
	Slug           string    `json:"slug"`
	PublicURLPath  string    `json:"public_url_path"`  // "/p/{slug}"
	ManageURLPath  string    `json:"manage_url_path"`  // "/manage/token/{raw}"。**再表示しない、UI 即時提示**
	PublishedAt    time.Time `json:"published_at"`
}

// Publish は POST /api/photobooks/{id}/publish ハンドラ。
func (h *PublishHandlers) Publish(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req publishRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}

	out, err := h.publish.Execute(r.Context(), usecase.PublishFromDraftInput{
		PhotobookID:     pid,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	})
	if err != nil {
		writePublishError(w, err)
		return
	}

	pb := out.Photobook
	if pb.PublicUrlSlug() == nil || pb.PublishedAt() == nil {
		// invariant 違反（publish 直後に slug / published_at が無いことはあり得ない）
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}
	slugStr := pb.PublicUrlSlug().String()
	resp := publishResponse{
		PhotobookID:   pb.ID().String(),
		Slug:          slugStr,
		PublicURLPath: "/p/" + slugStr,
		ManageURLPath: "/manage/token/" + out.RawManageToken.Encode(),
		PublishedAt:   pb.PublishedAt().UTC(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// writePublishError は publish 系 error を HTTP status に変換する。
//
// 状態不整合 / OCC 違反は 409 に集約。「draft 以外」「version 不一致」「rights 未同意」
// 「title / creator 空」を区別しない（情報漏洩抑止）。
func writePublishError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrPublishConflict),
		errors.Is(err, photobookrdb.ErrOptimisticLockConflict),
		errors.Is(err, photobookrdb.ErrNotDraft),
		errors.Is(err, domain.ErrNotDraft),
		errors.Is(err, domain.ErrRightsNotAgreed),
		errors.Is(err, domain.ErrEmptyCreatorName),
		errors.Is(err, domain.ErrEmptyTitle):
		writeJSONStatus(w, http.StatusConflict, bodyConflict)
	case errors.Is(err, photobookrdb.ErrNotFound):
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
	default:
		// pg unique violation 等、想定外は 500
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// slug 衝突等は 409 として扱う（極稀、MinimalSlugGenerator では事実上発生しない）
			writeJSONStatus(w, http.StatusConflict, bodyConflict)
			return
		}
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
	}
}
