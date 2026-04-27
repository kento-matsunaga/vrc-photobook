// manage_handler: 管理ページの HTTP handler。
//
// 設計参照:
//   - docs/plan/m2-public-viewer-and-manage-plan.md §4 / §6 / §12
//
// 仕様:
//   - GET /api/manage/photobooks/{id}
//   - manage Cookie 必須（router 側で RequireManageSession middleware を適用）
//   - 200 / 404 / 500
//   - manage_url_token / draft_edit_token / hash 値は応答に含めない
//   - manage URL の再送経路は ADR-0006（email provider 再選定中）の決着後に検討。
//     MVP は publish 完了画面での 1 回表示が標準で、再送 URL は応答に含めない
package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// ManageHandlers は管理ページの HTTP handler 群。
type ManageHandlers struct {
	getManage *usecase.GetManagePhotobook
}

// NewManageHandlers は ManageHandlers を組み立てる。
func NewManageHandlers(getManage *usecase.GetManagePhotobook) *ManageHandlers {
	return &ManageHandlers{getManage: getManage}
}

type managePhotobookPayload struct {
	PhotobookID           string     `json:"photobook_id"`
	Type                  string     `json:"type"`
	Title                 string     `json:"title"`
	Status                string     `json:"status"`
	Visibility            string     `json:"visibility"`
	HiddenByOperator      bool       `json:"hidden_by_operator"`
	PublicURLSlug         *string    `json:"public_url_slug,omitempty"`
	PublicURLPath         *string    `json:"public_url_path,omitempty"`
	PublishedAt           *time.Time `json:"published_at,omitempty"`
	DeletedAt             *time.Time `json:"deleted_at,omitempty"`
	DraftExpiresAt        *time.Time `json:"draft_expires_at,omitempty"`
	ManageURLTokenVersion int        `json:"manage_url_token_version"`
	AvailableImageCount   int        `json:"available_image_count"`
}

// GetManagePhotobook は GET /api/manage/photobooks/{id} ハンドラ。
func (h *ManageHandlers) GetManagePhotobook(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	rawID := chi.URLParam(r, "id")
	if rawID == "" {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	u, err := uuid.Parse(rawID)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	pid, err := photobook_id.FromUUID(u)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}

	out, err := h.getManage.Execute(r.Context(), usecase.GetManagePhotobookInput{
		PhotobookID: pid,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrManageNotFound) {
			writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
			return
		}
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}

	v := out.View
	writeJSON(w, http.StatusOK, managePhotobookPayload{
		PhotobookID:           v.PhotobookID,
		Type:                  v.Type,
		Title:                 v.Title,
		Status:                v.Status,
		Visibility:            v.Visibility,
		HiddenByOperator:      v.HiddenByOperator,
		PublicURLSlug:         v.PublicURLSlug,
		PublicURLPath:         v.PublicURLPath,
		PublishedAt:           ptrTimeUTC(v.PublishedAt),
		DeletedAt:             ptrTimeUTC(v.DeletedAt),
		DraftExpiresAt:        ptrTimeUTC(v.DraftExpiresAt),
		ManageURLTokenVersion: v.ManageURLTokenVersion,
		AvailableImageCount:   v.AvailableImageCount,
	})
}

// ptrTimeUTC は *time.Time の UTC 化（nil 安全）。
func ptrTimeUTC(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}
