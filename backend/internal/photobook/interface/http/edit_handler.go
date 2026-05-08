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
	"vrcpb/backend/internal/photobook/domain"
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
	// STOP P-2: 編集 mutation の publish-precondition-ux 流 reason 分離 (authenticated owner
	// 経路は復旧導線優先)。計画 §3.4 / §11.3 / `.agents/rules/publish-precondition-ux.md`。
	bodyConflictPageLimit       = `{"status":"version_conflict","reason":"page_limit_exceeded"}`
	bodyConflictSplitEmpty      = `{"status":"version_conflict","reason":"split_would_create_empty_page"}`
	bodyConflictInvalidPosition = `{"status":"bad_request","reason":"invalid_position"}`
	// STOP P-3: merge / pages reorder の reason 分離 (計画 §3.4.4 / §3.4.5 / §5.5 / §5.13)。
	bodyConflictMergeIntoSelf            = `{"status":"version_conflict","reason":"merge_into_self"}`
	bodyConflictCannotRemoveLastPage     = `{"status":"version_conflict","reason":"cannot_remove_last_page"}`
	bodyBadRequestInvalidReorderAssigns  = `{"status":"bad_request","reason":"invalid_reorder_assignments"}`
)

// EditHandlers は編集画面の HTTP handler 群。
type EditHandlers struct {
	getEditView        *usecase.GetEditView
	updatePhotoCaption *usecase.UpdatePhotoCaption
	bulkReorder        *usecase.BulkReorderPhotosOnPage
	updateSettings     *usecase.UpdatePhotobookSettings
	addPage            *usecase.AddPage
	removePage         *usecase.RemovePage
	removePhoto        *usecase.RemovePhoto
	setCoverImage      *usecase.SetCoverImage
	clearCoverImage    *usecase.ClearCoverImage
	// attachAvailableImages は /prepare/attach-images で「photobook の available 未 attach
	// image を 1 TX で bulk attach」する usecase（plan v2 §3.4 / §5）。nil なら handler は
	// 503 を返す（main.go で wire しない選択肢を残す）。
	attachAvailableImages *usecase.AttachAvailableImages
	// STOP P-2: m2-edit Phase A 核 3 endpoint
	updatePageCaption *usecase.UpdatePageCaption
	splitPage         *usecase.SplitPage
	movePhoto         *usecase.MovePhotoBetweenPages
	// STOP P-3: m2-edit Phase A 補強 2 endpoint (merge / pages reorder)
	mergePages   *usecase.MergePages
	reorderPages *usecase.ReorderPages
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
	attachAvailableImages *usecase.AttachAvailableImages,
	updatePageCaption *usecase.UpdatePageCaption,
	splitPage *usecase.SplitPage,
	movePhoto *usecase.MovePhotoBetweenPages,
	mergePages *usecase.MergePages,
	reorderPages *usecase.ReorderPages,
) *EditHandlers {
	return &EditHandlers{
		getEditView: getEditView, updatePhotoCaption: updatePhotoCaption,
		bulkReorder: bulkReorder, updateSettings: updateSettings,
		addPage: addPage, removePage: removePage, removePhoto: removePhoto,
		setCoverImage: setCover, clearCoverImage: clearCover,
		attachAvailableImages: attachAvailableImages,
		updatePageCaption:     updatePageCaption,
		splitPage:             splitPage,
		movePhoto:             movePhoto,
		mergePages:            mergePages,
		reorderPages:          reorderPages,
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
	// Images は photobook 内の全 active image（plan v2 §3.2 P0-b）。
	// reload 復元 + progress UI ground truth、attach 済 / 未配置 を問わず列挙。
	Images         []editImagePayload `json:"images"`
	DraftExpiresAt *time.Time         `json:"draft_expires_at,omitempty"`
}

type editImagePayload struct {
	ImageID          string    `json:"image_id"`
	Status           string    `json:"status"`
	SourceFormat     string    `json:"source_format"`
	OriginalByteSize int64     `json:"original_byte_size"`
	FailureReason    *string   `json:"failure_reason,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
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
	images := make([]editImagePayload, 0, len(v.Images))
	for _, img := range v.Images {
		images = append(images, editImagePayload{
			ImageID:          img.ImageID,
			Status:           img.Status,
			SourceFormat:     img.SourceFormat,
			OriginalByteSize: img.OriginalByteSize,
			FailureReason:    img.FailureReason,
			CreatedAt:        img.CreatedAt.UTC(),
		})
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
		Images:         images,
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
// 既存方針:
//   - OCC 違反 / 状態不整合 / Image owner 不一致 → 409 version_conflict (敵対者観測抑止)
//
// STOP P-2 拡張 (publish-precondition-ux ルールを編集 mutation にも展開、計画 §3.4 / §11.3):
//   - domain.ErrPageLimitExceeded → 409 + reason `page_limit_exceeded`
//   - usecase.ErrSplitWouldCreateEmptyPage → 409 + reason `split_would_create_empty_page`
//   - usecase.ErrInvalidMovePosition → 400 + reason `invalid_position` (defensive、handler
//     入口で先に弾くが UseCase 側でも検査するため漏れた場合の保険)
//
// STOP P-3 拡張 (merge / pages reorder、計画 §3.4.4 / §3.4.5 / §5.5 / §5.13):
//   - usecase.ErrMergeIntoSelf → 409 + reason `merge_into_self`
//   - usecase.ErrCannotRemoveLastPage → 409 + reason `cannot_remove_last_page`
//   - usecase.ErrInvalidReorderAssignments → 400 + reason `invalid_reorder_assignments`
//
// 外部に詳細を漏らさない方針は維持。reason 追加は authenticated owner edit 経路のみ。
func writeEditMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrPageLimitExceeded):
		writeJSONStatus(w, http.StatusConflict, bodyConflictPageLimit)
	case errors.Is(err, usecase.ErrSplitWouldCreateEmptyPage):
		writeJSONStatus(w, http.StatusConflict, bodyConflictSplitEmpty)
	case errors.Is(err, usecase.ErrInvalidMovePosition):
		writeJSONStatus(w, http.StatusBadRequest, bodyConflictInvalidPosition)
	case errors.Is(err, usecase.ErrMergeIntoSelf):
		writeJSONStatus(w, http.StatusConflict, bodyConflictMergeIntoSelf)
	case errors.Is(err, usecase.ErrCannotRemoveLastPage):
		writeJSONStatus(w, http.StatusConflict, bodyConflictCannotRemoveLastPage)
	case errors.Is(err, usecase.ErrInvalidReorderAssignments):
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequestInvalidReorderAssigns)
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

// === POST /api/photobooks/{id}/prepare/attach-images ===
//
// /prepare の「編集へ進む」が呼ぶ bulk attach endpoint（plan v2 §3.4 / §5）。
// request body は expected_version のみで image_id 配列は受け取らない（user 指示 §6.1、
// server ground truth から ListAvailableUnattachedImageIDs で取得して attach する）。
// response は count-only、raw image_id / page_id / photo_id を返さない。

type attachPrepareImagesRequest struct {
	ExpectedVersion int `json:"expected_version"`
}

type attachPrepareImagesResponse struct {
	AttachedCount int `json:"attached_count"`
	PageCount     int `json:"page_count"`
	SkippedCount  int `json:"skipped_count"`
}

// AttachPrepareImages は POST /api/photobooks/{id}/prepare/attach-images ハンドラ。
//
// 認可: draft session middleware が photobook id 一致の Cookie を検証済の前提。
// 失敗 mapping:
//   - 400 bad_request: JSON decode 失敗
//   - 404 not_found: photobook 不存在
//   - 409 version_conflict: status != draft / OCC version 不一致 / draft 期限切れ
//   - 500 internal_error: 想定外
//   - 503 service_unavailable: usecase 未注入（main 側で wire していない場合）
func (h *EditHandlers) AttachPrepareImages(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	if h.attachAvailableImages == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, bodyServerError)
		return
	}

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}

	var req attachPrepareImagesRequest
	if err := decodeJSON(r, &req); err != nil {
		// 型違い（"abc" 等）/ malformed JSON は decode 失敗で 400
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	// expected_version の入力 validation:
	//   - omitted（field 不在）→ Go zero-value 0 として扱う（version 0 = 初回 attach の正常入力）
	//   - 負数 → 不正入力で 400（version は 0 以上のみ）
	//   - 0 以上 → usecase に渡し、photobook の実 version と比較（mismatch なら 409）
	if req.ExpectedVersion < 0 {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}

	out, err := h.attachAvailableImages.Execute(r.Context(), usecase.AttachAvailableImagesInput{
		PhotobookID:     pid,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now(),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrEditPhotobookNotFound):
			writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		case errors.Is(err, usecase.ErrEditNotAllowed),
			errors.Is(err, photobookrdb.ErrOptimisticLockConflict),
			errors.Is(err, photobookrdb.ErrNotDraft):
			writeJSONStatus(w, http.StatusConflict, bodyConflict)
		default:
			writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, attachPrepareImagesResponse{
		AttachedCount: out.AttachedCount,
		PageCount:     out.PageCount,
		SkippedCount:  out.SkippedCount,
	})
}

// ============================================================================
// STOP P-2: m2-edit Phase A 核 3 endpoint
// ----------------------------------------------------------------------------
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §3.4 / §6
// API spec:
//   - PATCH /pages/{pageId}/caption        (A 方式: {"version": N+1})
//   - POST  /pages/{pageId}/split          (B 方式: 更新後 EditView)
//   - PATCH /photos/{photoId}/move         (B 方式: 更新後 EditView)
// ============================================================================

// === PATCH /api/photobooks/{id}/pages/{pageId}/caption ===

type updatePageCaptionRequest struct {
	Caption         *string `json:"caption"` // null or "" → caption をクリア
	ExpectedVersion int     `json:"expected_version"`
}

type updatePageCaptionResponse struct {
	Version int `json:"version"`
}

// UpdatePageCaption は page caption 単独編集 (A 方式: version のみ返す)。
func (h *EditHandlers) UpdatePageCaption(w http.ResponseWriter, r *http.Request) {
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
	var req updatePageCaptionRequest
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
	if err := h.updatePageCaption.Execute(r.Context(), usecase.UpdatePageCaptionInput{
		PhotobookID:     pid,
		PageID:          pageID,
		Caption:         capVO,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	// A 方式: 成功時 version+1 を返す (request の expected_version + 1)
	writeJSON(w, http.StatusOK, updatePageCaptionResponse{Version: req.ExpectedVersion + 1})
}

// === POST /api/photobooks/{id}/pages/{pageId}/split ===

type splitPageRequest struct {
	PhotoID         string `json:"photo_id"`
	ExpectedVersion int    `json:"expected_version"`
}

// SplitPage は source page の指定 photo の "次から" 末尾までを新 page に分離する (B 方式)。
//
// 成功時: 更新後 EditView 全体を返す (handler が改めて GetEditView を呼ぶ)。
func (h *EditHandlers) SplitPage(w http.ResponseWriter, r *http.Request) {
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
	sourcePageID, err := page_id.FromUUID(pageUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req splitPageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	phUUID, err := uuid.Parse(req.PhotoID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	splitAtPhotoID, err := photo_id.FromUUID(phUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if _, err := h.splitPage.Execute(r.Context(), usecase.SplitPageInput{
		PhotobookID:     pid,
		SourcePageID:    sourcePageID,
		SplitAtPhotoID:  splitAtPhotoID,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	// B 方式: 更新後 EditView を返す
	h.writeEditViewAfterMutation(w, r, pid)
}

// === PATCH /api/photobooks/{id}/photos/{photoId}/move ===

type movePhotoRequest struct {
	TargetPageID    string `json:"target_page_id"`
	Position        string `json:"position"` // "start" | "end"
	ExpectedVersion int    `json:"expected_version"`
}

// MovePhoto は photo を別 page (or 同 page) の start / end に移動する (B 方式)。
func (h *EditHandlers) MovePhoto(w http.ResponseWriter, r *http.Request) {
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
	var req movePhotoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	targetUUID, err := uuid.Parse(req.TargetPageID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	targetPageID, err := page_id.FromUUID(targetUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	// position は MVP "start" / "end" のみ。それ以外は 400 reason `invalid_position`。
	var position usecase.MovePosition
	switch req.Position {
	case "start":
		position = usecase.MovePositionStart
	case "end":
		position = usecase.MovePositionEnd
	default:
		writeJSONStatus(w, http.StatusBadRequest, bodyConflictInvalidPosition)
		return
	}
	if err := h.movePhoto.Execute(r.Context(), usecase.MovePhotoBetweenPagesInput{
		PhotobookID:     pid,
		PhotoID:         photoID,
		TargetPageID:    targetPageID,
		Position:        position,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	// B 方式: 更新後 EditView を返す
	h.writeEditViewAfterMutation(w, r, pid)
}

// writeEditViewAfterMutation は SplitPage / MovePhoto などの B 方式 endpoint で、mutation
// 成功後に GetEditView を呼んで更新後の EditView を JSON 返却する共通ヘルパー。
//
// 後続 GetEditView が失敗した場合 (理論上は発生しないが defensive): 500 を返す。mutation
// 自体は成功しているため Frontend は別途 reload で取得を試みれば成立する。
func (h *EditHandlers) writeEditViewAfterMutation(
	w http.ResponseWriter,
	r *http.Request,
	pid photobook_id.PhotobookID,
) {
	out, err := h.getEditView.Execute(r.Context(), usecase.GetEditViewInput{PhotobookID: pid})
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}
	writeJSON(w, http.StatusOK, toEditViewPayload(out.View))
}

// ============================================================================
// STOP P-3: m2-edit Phase A 補強 2 endpoint
// ----------------------------------------------------------------------------
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §3.4.4 / §3.4.5
// API spec:
//   - POST  /pages/{pageId}/merge-into/{targetPageId}  (B 方式: 更新後 EditView)
//   - PATCH /pages/reorder                              (B 方式: 更新後 EditView)
// ============================================================================

// === POST /api/photobooks/{id}/pages/{pageId}/merge-into/{targetPageId} ===

type mergePagesRequest struct {
	ExpectedVersion int `json:"expected_version"`
}

// MergePages は source page (pageId) の全 photo を target page 末尾に追加し、source page を
// 削除する (B 方式)。source の caption / page_meta は破棄される (UI で警告 modal を出す前提)。
//
// edge case:
//   - source == target → 409 + reason `merge_into_self` (5.5)
//   - photobook に page 1 件 → 409 + reason `cannot_remove_last_page` (5.13)
func (h *EditHandlers) MergePages(w http.ResponseWriter, r *http.Request) {
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
	sourcePageID, err := page_id.FromUUID(pageUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	targetUUID, ok := parseUUIDParam(r, "targetPageId")
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	targetPageID, err := page_id.FromUUID(targetUUID)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req mergePagesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if err := h.mergePages.Execute(r.Context(), usecase.MergePagesInput{
		PhotobookID:     pid,
		SourcePageID:    sourcePageID,
		TargetPageID:    targetPageID,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	// B 方式: 更新後 EditView を返す
	h.writeEditViewAfterMutation(w, r, pid)
}

// === PATCH /api/photobooks/{id}/pages/reorder ===

type reorderPagesAssignment struct {
	PageID       string `json:"page_id"`
	DisplayOrder int    `json:"display_order"`
}

type reorderPagesRequest struct {
	Assignments     []reorderPagesAssignment `json:"assignments"`
	ExpectedVersion int                      `json:"expected_version"`
}

// ReorderPages は photobook 配下の全 page を一括再採番する (B 方式)。
//
// assignments は当該 photobook の全 page を含む必要があり、display_order は 0..N-1 の
// permutation でなければ 400 reason `invalid_reorder_assignments`。
func (h *EditHandlers) ReorderPages(w http.ResponseWriter, r *http.Request) {
	commonHeaders(w)

	pid, ok := parsePhotobookID(r)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}
	var req reorderPagesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if len(req.Assignments) == 0 {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequestInvalidReorderAssigns)
		return
	}
	assigns := make([]usecase.ReorderPagesAssignment, 0, len(req.Assignments))
	for _, a := range req.Assignments {
		pgUUID, err := uuid.Parse(a.PageID)
		if err != nil {
			writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
			return
		}
		pgID, err := page_id.FromUUID(pgUUID)
		if err != nil {
			writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
			return
		}
		ord, err := display_order.New(a.DisplayOrder)
		if err != nil {
			writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
			return
		}
		assigns = append(assigns, usecase.ReorderPagesAssignment{PageID: pgID, DisplayOrder: ord})
	}
	if err := h.reorderPages.Execute(r.Context(), usecase.ReorderPagesInput{
		PhotobookID:     pid,
		Assignments:     assigns,
		ExpectedVersion: req.ExpectedVersion,
		Now:             time.Now().UTC(),
	}); err != nil {
		writeEditMutationError(w, err)
		return
	}
	// B 方式: 更新後 EditView を返す
	h.writeEditViewAfterMutation(w, r, pid)
}
