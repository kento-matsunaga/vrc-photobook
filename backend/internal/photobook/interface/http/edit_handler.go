// edit_handler: 編集画面 (PR27) の HTTP handler 群。
//
// 設計参照:
//   - docs/plan/m2-frontend-edit-ui-fullspec-plan.md §4 / §5
//
// 仕様:
//   - すべて draft Cookie 必須（router 側で RequireDraftSession middleware を適用）
//   - status='draft' AND version=$expected で OCC、0 行 UPDATE は 409 version_conflict
//   - 失敗詳細は body に出さない（draft 以外 / version 不一致 を区別しない）
//   - storage_key 完全値 / R2 credentials / 未公開 photobook の存在情報 は出さない
//   - Cache-Control: no-store / X-Robots-Tag: noindex,nofollow を全レスポンスに付与
package http

import (
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	imagedomainvo "vrcpb/backend/internal/image/domain/vo/image_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

const (
	bodyConflict = `{"status":"version_conflict"}`
)

// EditHandlers は編集画面の HTTP handler 群。
type EditHandlers struct {
	getEditView         *usecase.GetEditView
	updatePhotoCaption  *usecase.UpdatePhotoCaption
	bulkReorder         *usecase.BulkReorderPhotosOnPage
	updateSettings      *usecase.UpdatePhotobookSettings
	addPage             *usecase.AddPage
	removePage          *usecase.RemovePage
	removePhoto         *usecase.RemovePhoto
	setCoverImage       *usecase.SetCoverImage
	clearCoverImage     *usecase.ClearCoverImage
}

// NewEditHandlers は EditHandlers を組み立てる。
func NewEditHandlers(
	getEditView *usecase.GetEditView,
	updatePhotoCaption *usecase.UpdatePhotoCaption,
	bulkReorder *usecase.BulkReorderPhotosOnPage,
	updateSettings *usecase.UpdatePhotobookSettings,
	addPage *usecase.AddPage,
	removePage *usecase.RemovePage,
	removePhoto *usecase.RemovePhoto,
	setCover *usecase.SetCoverImage,
	clearCover *usecase.ClearCoverImage,
) *EditHandlers {
	return &EditHandlers{
		getEditView: getEditView, updatePhotoCaption: updatePhotoCaption,
		bulkReorder: bulkReorder, updateSettings: updateSettings,
		addPage: addPage, removePage: removePage, removePhoto: removePhoto,
		setCoverImage: setCover, clearCoverImage: clearCover,
	}
}

// commonHeaders は編集系応答の共通ヘッダ。
func commonHeaders(w http.ResponseWriter) {
	addNoStore(w)
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
}

// parsePhotobookID は URL の {id} を photobook_id VO に変換する。
func parsePhotobookID(r *http.Request) (photobook_id.PhotobookID, bool) {
	raw := chi.URLParam(r, "id")
	u, err := uuid.Parse(raw)
	if err != nil {
		return photobook_id.PhotobookID{}, false
	}
	pid, err := photobook_id.FromUUID(u)
	if err != nil {
		return photobook_id.PhotobookID{}, false
	}
	return pid, true
}

// parseUUIDParam は任意の URL param を uuid に変換する。
func parseUUIDParam(r *http.Request, name string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, name)
	u, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, false
	}
	return u, true
}

// === GET /api/photobooks/{id}/edit-view ===

// 応答 payload は usecase view を camelCase / snake_case 混在で返す。
type editPresignedURL struct {
	URL       string    `json:"url"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	ExpiresAt time.Time `json:"expires_at"`
}

type editVariantSetPayload struct {
	Display   editPresignedURL `json:"display"`
	Thumbnail editPresignedURL `json:"thumbnail"`
}

type editPhotoPayload struct {
	PhotoID      string                `json:"photo_id"`
	ImageID      string                `json:"image_id"`
	DisplayOrder int                   `json:"display_order"`
	Caption      *string               `json:"caption,omitempty"`
	Variants     editVariantSetPayload `json:"variants"`
}

type editPagePayload struct {
	PageID       string             `json:"page_id"`
	DisplayOrder int                `json:"display_order"`
	Caption      *string            `json:"caption,omitempty"`
	Photos       []editPhotoPayload `json:"photos"`
}

type editSettingsPayload struct {
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Description  *string `json:"description,omitempty"`
	Layout       string  `json:"layout"`
	OpeningStyle string  `json:"opening_style"`
	Visibility   string  `json:"visibility"`
	CoverTitle   *string `json:"cover_title,omitempty"`
}

type editViewPayload struct {
	PhotobookID     string                 `json:"photobook_id"`
	Status          string                 `json:"status"`
	Version         int                    `json:"version"`
	Settings        editSettingsPayload    `json:"settings"`
	CoverImageID    *string                `json:"cover_image_id,omitempty"`
	Cover           *editVariantSetPayload `json:"cover,omitempty"`
	Pages           []editPagePayload      `json:"pages"`
	ProcessingCount int                    `json:"processing_count"`
	FailedCount     int                    `json:"failed_count"`
	DraftExpiresAt  *time.Time             `json:"draft_expires_at,omitempty"`
}

// GetEditView は GET /api/photobooks/{id}/edit-view ハンドラ。
func (h *EditHandlers) GetEditView(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	out, err := h.getEditView.Execute(r.Context(), usecase.GetEditViewInput{PhotobookID: pid})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrEditPhotobookNotFound):
			writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		case errors.Is(err, usecase.ErrEditNotAllowed):
			writeJSONStatus(w, http.StatusConflict, bodyConflict)
		default:
			writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, toEditViewPayload(out.View))
}

func toEditViewPayload(v usecase.EditPhotobookView) editViewPayload {
	pages := make([]editPagePayload, 0, len(v.Pages))
	for _, p := range v.Pages {
		photos := make([]editPhotoPayload, 0, len(p.Photos))
		for _, ph := range p.Photos {
			photos = append(photos, editPhotoPayload{
				PhotoID: ph.PhotoID, ImageID: ph.ImageID, DisplayOrder: ph.DisplayOrder,
				Caption: ph.Caption,
				Variants: editVariantSetPayload{
					Display: editPresignedURL{
						URL: ph.Display.URL, Width: ph.Display.Width, Height: ph.Display.Height,
						ExpiresAt: ph.Display.ExpiresAt.UTC(),
					},
					Thumbnail: editPresignedURL{
						URL: ph.Thumbnail.URL, Width: ph.Thumbnail.Width, Height: ph.Thumbnail.Height,
						ExpiresAt: ph.Thumbnail.ExpiresAt.UTC(),
					},
				},
			})
		}
		pages = append(pages, editPagePayload{
			PageID: p.PageID, DisplayOrder: p.DisplayOrder, Caption: p.Caption, Photos: photos,
		})
	}
	var cover *editVariantSetPayload
	if v.Cover != nil {
		c := editVariantSetPayload{
			Display: editPresignedURL{
				URL: v.Cover.Display.URL, Width: v.Cover.Display.Width, Height: v.Cover.Display.Height,
				ExpiresAt: v.Cover.Display.ExpiresAt.UTC(),
			},
			Thumbnail: editPresignedURL{
				URL: v.Cover.Thumbnail.URL, Width: v.Cover.Thumbnail.Width, Height: v.Cover.Thumbnail.Height,
				ExpiresAt: v.Cover.Thumbnail.ExpiresAt.UTC(),
			},
		}
		cover = &c
	}
	var draftExp *time.Time
	if v.DraftExpiresAt != nil {
		t := v.DraftExpiresAt.UTC()
		draftExp = &t
	}
	return editViewPayload{
		PhotobookID: v.PhotobookID, Status: v.Status, Version: v.Version,
		Settings: editSettingsPayload{
			Type: v.Settings.Type, Title: v.Settings.Title, Description: v.Settings.Description,
			Layout: v.Settings.Layout, OpeningStyle: v.Settings.OpeningStyle,
			Visibility: v.Settings.Visibility, CoverTitle: v.Settings.CoverTitle,
		},
		CoverImageID: v.CoverImageID, Cover: cover, Pages: pages,
		ProcessingCount: v.ProcessingCount, FailedCount: v.FailedCount,
		DraftExpiresAt: draftExp,
	}
}

// === PATCH /api/photobooks/{id}/photos/{photoId}/caption ===

type updateCaptionRequest struct {
	Caption         *string `json:"caption"` // null or "" → caption をクリア
	ExpectedVersion int     `json:"expected_version"`
}

// UpdatePhotoCaption は photo caption 単独編集。
func (h *EditHandlers) UpdatePhotoCaption(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	photoUUID, ok := parseUUIDParam(r, "photoId")
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	photoID, err := photo_id.FromUUID(photoUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req updateCaptionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	var capVO *caption.Caption
	if req.Caption != nil && *req.Caption != "" {
		c, err := caption.New(*req.Caption)
		if err != nil {
			writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
			return
		}
		capVO = &c
	}
	if err := h.updatePhotoCaption.Execute(r.Context(), usecase.UpdatePhotoCaptionInput{
		PhotobookID:     pid,
		PhotoID:         photoID,
		Caption:         capVO,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === PATCH /api/photobooks/{id}/photos/reorder ===

type reorderItem struct {
	PhotoID      string `json:"photo_id"`
	DisplayOrder int    `json:"display_order"`
}

type reorderRequest struct {
	PageID          string        `json:"page_id"`
	Assignments     []reorderItem `json:"assignments"`
	ExpectedVersion int           `json:"expected_version"`
}

// BulkReorderPhotos は複数 photo の display_order を一括で再配置する。
func (h *EditHandlers) BulkReorderPhotos(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req reorderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	pageUUID, err := uuid.Parse(req.PageID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	pageID, err := page_id.FromUUID(pageUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if len(req.Assignments) == 0 {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	assigns := make([]usecase.PhotoOrderItem, 0, len(req.Assignments))
	for _, a := range req.Assignments {
		phUUID, err := uuid.Parse(a.PhotoID)
		if err != nil {
			writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
			return
		}
		phID, err := photo_id.FromUUID(phUUID)
		if err != nil {
			writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
			return
		}
		ord, err := display_order.New(a.DisplayOrder)
		if err != nil {
			writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
			return
		}
		assigns = append(assigns, usecase.PhotoOrderItem{PhotoID: phID, NewOrder: ord})
	}
	if err := h.bulkReorder.Execute(r.Context(), usecase.BulkReorderPhotosOnPageInput{
		PhotobookID:     pid,
		PageID:          pageID,
		Assignments:     assigns,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === PATCH /api/photobooks/{id}/cover-image ===

type setCoverRequest struct {
	ImageID         string `json:"image_id"`
	ExpectedVersion int    `json:"expected_version"`
}

// SetCoverImage は cover_image_id を更新する。
func (h *EditHandlers) SetCoverImage(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req setCoverRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	imgUUID, err := uuid.Parse(req.ImageID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	imgID, err := imagedomainvo.FromUUID(imgUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if err := h.setCoverImage.Execute(r.Context(), usecase.SetCoverImageInput{
		PhotobookID:     pid,
		ImageID:         imgID,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === DELETE /api/photobooks/{id}/cover-image ===

type clearCoverRequest struct {
	ExpectedVersion int `json:"expected_version"`
}

// ClearCoverImage は cover_image_id を NULL にする。
func (h *EditHandlers) ClearCoverImage(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req clearCoverRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if err := h.clearCoverImage.Execute(r.Context(), usecase.ClearCoverImageInput{
		PhotobookID:     pid,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === PATCH /api/photobooks/{id}/settings ===

type updateSettingsRequest struct {
	Type            string  `json:"type"`
	Title           string  `json:"title"`
	Description     *string `json:"description"`
	Layout          string  `json:"layout"`
	OpeningStyle    string  `json:"opening_style"`
	Visibility      string  `json:"visibility"`
	CoverTitle      *string `json:"cover_title"`
	ExpectedVersion int     `json:"expected_version"`
}

// UpdateSettings は draft Photobook の settings 一括 PATCH。
func (h *EditHandlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req updateSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if !validSettingsLengths(req) {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if err := h.updateSettings.Execute(r.Context(), usecase.UpdatePhotobookSettingsInput{
		PhotobookID:     pid,
		Type:            req.Type,
		Title:           req.Title,
		Description:     req.Description,
		Layout:          req.Layout,
		OpeningStyle:    req.OpeningStyle,
		Visibility:      req.Visibility,
		CoverTitle:      req.CoverTitle,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validSettingsLengths は title / description / cover_title の長さと文字種を最低限チェック。
//
// VO レベルの厳密 validation は domain 側 / DB CHECK で再担保する想定。
func validSettingsLengths(req updateSettingsRequest) bool {
	if utf8.RuneCountInString(req.Title) < 1 || utf8.RuneCountInString(req.Title) > 80 {
		return false
	}
	if req.Description != nil && utf8.RuneCountInString(*req.Description) > 500 {
		return false
	}
	if req.CoverTitle != nil && utf8.RuneCountInString(*req.CoverTitle) > 80 {
		return false
	}
	return true
}

// === POST /api/photobooks/{id}/pages ===

type addPageRequest struct {
	ExpectedVersion int `json:"expected_version"`
}

type addPageResponse struct {
	PageID       string `json:"page_id"`
	DisplayOrder int    `json:"display_order"`
}

// AddPage は draft Photobook に page を追加する。
func (h *EditHandlers) AddPage(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req addPageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	out, err := h.addPage.Execute(r.Context(), usecase.AddPageInput{
		PhotobookID:     pid,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	})
	if err != nil {
		writeEditMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, addPageResponse{
		PageID: out.Page.ID().String(), DisplayOrder: out.Page.DisplayOrder().Int(),
	})
}

// === DELETE /api/photobooks/{id}/pages/{pageId} ===

type removePageRequest struct {
	ExpectedVersion int `json:"expected_version"`
}

// RemovePage は draft Photobook から page を削除する。
func (h *EditHandlers) RemovePage(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	pageUUID, ok := parseUUIDParam(r, "pageId")
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	pageID, err := page_id.FromUUID(pageUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req removePageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if err := h.removePage.Execute(r.Context(), usecase.RemovePageInput{
		PhotobookID:     pid,
		PageID:          pageID,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === DELETE /api/photobooks/{id}/photos/{photoId} ===

type removePhotoRequest struct {
	PageID          string `json:"page_id"`
	ExpectedVersion int    `json:"expected_version"`
}

// RemovePhoto は draft Photobook から photo を削除する。
func (h *EditHandlers) RemovePhoto(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	photoUUID, ok := parseUUIDParam(r, "photoId")
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	photoID, err := photo_id.FromUUID(photoUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req removePhotoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	pageUUID, err := uuid.Parse(req.PageID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	pageID, err := page_id.FromUUID(pageUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if err := h.removePhoto.Execute(r.Context(), usecase.RemovePhotoInput{
		PhotobookID:     pid,
		PageID:          pageID,
		PhotoID:         photoID,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeEditMutationError は UseCase の error を HTTP status に変換する。
//
// すべての OCC 違反 / 状態不整合 / Image owner 不一致は **409 version_conflict** に集約。
// 外部に詳細を漏らさない（業務知識 v4 の情報漏洩抑止方針）。
func writeEditMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, photobookrdb.ErrOptimisticLockConflict),
		errors.Is(err, photobookrdb.ErrPhotoNotFound),
		errors.Is(err, photobookrdb.ErrPageNotFound),
		errors.Is(err, photobookrdb.ErrImageNotAttachable),
		errors.Is(err, photobookrdb.ErrNotDraft):
		writeJSONStatus(w, http.StatusConflict, bodyConflict)
	default:
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
	}
}
