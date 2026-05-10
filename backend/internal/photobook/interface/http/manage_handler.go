// manage_handler: 管理ページの HTTP handler。
//
// 設計参照:
//   - docs/plan/m2-public-viewer-and-manage-plan.md §4 / §6 / §12
//   - docs/plan/m-1-manage-mvp-safety-plan.md §3 (M-1a)
//
// 仕様:
//   - GET    /api/manage/photobooks/{id}
//   - PATCH  /api/manage/photobooks/{id}/visibility    (M-1a)
//   - PATCH  /api/manage/photobooks/{id}/sensitive     (M-1a)
//   - POST   /api/manage/photobooks/{id}/draft-session (M-1a)
//   - POST   /api/manage/photobooks/{id}/session-revoke (M-1a)
//   - manage Cookie 必須（router 側で RequireManageSession middleware を適用）
//   - manage_url_token / draft_edit_token / hash 値は応答に含めない
//   - draft-session の raw session_token は **成功時のみ** body に乗せる（Frontend Route
//     Handler が Cookie 化する前提）。Set-Cookie は本 handler から出さない
package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	authmiddleware "vrcpb/backend/internal/auth/session/middleware"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// 失敗 body（manage 経路）。詳細を出さず固定文言で返す。
const (
	bodyManagePublicNotAllowed = `{"status":"manage_precondition_failed","reason":"public_change_not_allowed"}`
	bodyManageNotDraft         = `{"status":"manage_precondition_failed","reason":"not_draft"}`
	bodyVersionConflict        = `{"status":"version_conflict"}`
)

// ManageHandlers は管理ページの HTTP handler 群。
//
// M-1a 拡張: 既存 GetManagePhotobook に加え、visibility / sensitive / draft-session 発行 /
// session revoke の 4 mutation を追加。すべて manage middleware 通過後の Cookie session で認可。
type ManageHandlers struct {
	getManage         *usecase.GetManagePhotobook
	updateVisibility  *usecase.UpdatePhotobookVisibilityFromManage
	updateSensitive   *usecase.UpdatePhotobookSensitiveFromManage
	issueDraftSession *usecase.IssueDraftSessionFromManage
	revokeCurrent     *usecase.RevokeCurrentManageSession
	clock             Clock
}

// NewManageHandlers は ManageHandlers を組み立てる。
//
// nil の UseCase は本 handler では受け付けない（wireup で全 UseCase を必ず渡す）。
// clock が nil なら SystemClock を使う。
func NewManageHandlers(
	getManage *usecase.GetManagePhotobook,
	updateVisibility *usecase.UpdatePhotobookVisibilityFromManage,
	updateSensitive *usecase.UpdatePhotobookSensitiveFromManage,
	issueDraftSession *usecase.IssueDraftSessionFromManage,
	revokeCurrent *usecase.RevokeCurrentManageSession,
	clock Clock,
) *ManageHandlers {
	if clock == nil {
		clock = SystemClock{}
	}
	return &ManageHandlers{
		getManage:         getManage,
		updateVisibility:  updateVisibility,
		updateSensitive:   updateSensitive,
		issueDraftSession: issueDraftSession,
		revokeCurrent:     revokeCurrent,
		clock:             clock,
	}
}

type managePhotobookPayload struct {
	PhotobookID           string     `json:"photobook_id"`
	Type                  string     `json:"type"`
	Title                 string     `json:"title"`
	Status                string     `json:"status"`
	Visibility            string     `json:"visibility"`
	Sensitive             bool       `json:"sensitive"`
	HiddenByOperator      bool       `json:"hidden_by_operator"`
	PublicURLSlug         *string    `json:"public_url_slug,omitempty"`
	PublicURLPath         *string    `json:"public_url_path,omitempty"`
	PublishedAt           *time.Time `json:"published_at,omitempty"`
	DeletedAt             *time.Time `json:"deleted_at,omitempty"`
	DraftExpiresAt        *time.Time `json:"draft_expires_at,omitempty"`
	ManageURLTokenVersion int        `json:"manage_url_token_version"`
	AvailableImageCount   int        `json:"available_image_count"`
	// M-1a: PATCH /visibility / /sensitive の expected_version 用。
	Version int `json:"version"`
}

// GetManagePhotobook は GET /api/manage/photobooks/{id} ハンドラ。
func (h *ManageHandlers) GetManagePhotobook(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	pid, ok := parsePhotobookIDOrNotFound(w, r)
	if !ok {
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
		Sensitive:             v.Sensitive,
		HiddenByOperator:      v.HiddenByOperator,
		PublicURLSlug:         v.PublicURLSlug,
		PublicURLPath:         v.PublicURLPath,
		PublishedAt:           ptrTimeUTC(v.PublishedAt),
		DeletedAt:             ptrTimeUTC(v.DeletedAt),
		DraftExpiresAt:        ptrTimeUTC(v.DraftExpiresAt),
		ManageURLTokenVersion: v.ManageURLTokenVersion,
		AvailableImageCount:   v.AvailableImageCount,
		Version:               v.Version,
	})
}

// =============================================================================
// M-1a: PATCH /visibility
// =============================================================================

type updateVisibilityRequest struct {
	Visibility      string `json:"visibility"`
	ExpectedVersion int    `json:"expected_version"`
}

type versionResponse struct {
	Version int `json:"version"`
}

// UpdateVisibility は PATCH /api/manage/photobooks/{id}/visibility ハンドラ。
//
// 仕様:
//   - body { "visibility": "unlisted" | "private", "expected_version": int }
//   - "public" 指定 → 409 manage_precondition_failed (reason: public_change_not_allowed)
//   - 未知 visibility → 400 invalid_payload
//   - OCC violation / status≠published → 409 version_conflict（区別しない、敵対者観測抑止）
//   - 成功 → 200 OK + { "version": N+1 }
func (h *ManageHandlers) UpdateVisibility(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	pid, ok := parsePhotobookIDOrNotFound(w, r)
	if !ok {
		return
	}

	var req updateVisibilityRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyInvalidPayload)
		return
	}
	v, err := visibility.Parse(req.Visibility)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyInvalidPayload)
		return
	}
	// /manage では public 化禁止（業務知識 v4 §3.2）。public は専用 reason で返す。
	if v.Equal(visibility.Public()) {
		writeJSONStatus(w, http.StatusConflict, bodyManagePublicNotAllowed)
		return
	}

	err = h.updateVisibility.Execute(r.Context(), usecase.UpdatePhotobookVisibilityFromManageInput{
		PhotobookID:     pid,
		Visibility:      v,
		ExpectedVersion: req.ExpectedVersion,
		Now:             h.clock.Now(),
	})
	if err != nil {
		if errors.Is(err, usecase.ErrManagePublicChangeNotAllowed) {
			writeJSONStatus(w, http.StatusConflict, bodyManagePublicNotAllowed)
			return
		}
		if errors.Is(err, rdb.ErrOptimisticLockConflict) {
			writeJSONStatus(w, http.StatusConflict, bodyVersionConflict)
			return
		}
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}

	writeJSON(w, http.StatusOK, versionResponse{Version: req.ExpectedVersion + 1})
}

// =============================================================================
// M-1a: PATCH /sensitive
// =============================================================================

type updateSensitiveRequest struct {
	Sensitive       bool `json:"sensitive"`
	ExpectedVersion int  `json:"expected_version"`
}

// UpdateSensitive は PATCH /api/manage/photobooks/{id}/sensitive ハンドラ。
//
// 仕様:
//   - body { "sensitive": bool, "expected_version": int }
//   - OCC violation / status≠published → 409 version_conflict
//   - 成功 → 200 OK + { "version": N+1 }
func (h *ManageHandlers) UpdateSensitive(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	pid, ok := parsePhotobookIDOrNotFound(w, r)
	if !ok {
		return
	}

	var req updateSensitiveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyInvalidPayload)
		return
	}

	err := h.updateSensitive.Execute(r.Context(), usecase.UpdatePhotobookSensitiveFromManageInput{
		PhotobookID:     pid,
		Sensitive:       req.Sensitive,
		ExpectedVersion: req.ExpectedVersion,
		Now:             h.clock.Now(),
	})
	if err != nil {
		if errors.Is(err, rdb.ErrOptimisticLockConflict) {
			writeJSONStatus(w, http.StatusConflict, bodyVersionConflict)
			return
		}
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}

	writeJSON(w, http.StatusOK, versionResponse{Version: req.ExpectedVersion + 1})
}

// =============================================================================
// M-1a: POST /draft-session
// =============================================================================

type draftSessionFromManageResponse struct {
	SessionToken string    `json:"session_token"`
	PhotobookID  string    `json:"photobook_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// IssueDraftSession は POST /api/manage/photobooks/{id}/draft-session ハンドラ。
//
// 仕様:
//   - body 不要
//   - photobook が draft 状態でない場合 → 409 manage_precondition_failed (reason: not_draft)
//   - 成功 → 200 OK + { session_token, photobook_id, expires_at }
//   - Set-Cookie は本 handler から出さない（Frontend Route Handler が Cookie 化）
//   - **raw session_token は body にだけ乗せる、ログには出さない**
func (h *ManageHandlers) IssueDraftSession(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	pid, ok := parsePhotobookIDOrNotFound(w, r)
	if !ok {
		return
	}

	out, err := h.issueDraftSession.Execute(r.Context(), usecase.IssueDraftSessionFromManageInput{
		PhotobookID: pid,
		Now:         h.clock.Now(),
	})
	if err != nil {
		if errors.Is(err, usecase.ErrManageNotDraftForResume) {
			writeJSONStatus(w, http.StatusConflict, bodyManageNotDraft)
			return
		}
		if errors.Is(err, usecase.ErrManageNotFound) {
			writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
			return
		}
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}

	writeJSON(w, http.StatusOK, draftSessionFromManageResponse{
		SessionToken: out.RawSessionToken.Encode(),
		PhotobookID:  pid.String(),
		ExpiresAt:    out.ExpiresAt.UTC(),
	})
}

// =============================================================================
// M-1a: POST /session-revoke
// =============================================================================

type okResponse struct {
	OK bool `json:"ok"`
}

// RevokeCurrentSession は POST /api/manage/photobooks/{id}/session-revoke ハンドラ。
//
// 仕様:
//   - body 不要
//   - middleware が context にセットした現在 session を 1 件 revoke
//   - 成功 → 200 OK + { "ok": true }
//   - Set-Cookie / Cookie clear は本 handler から出さない（Workers Route Handler が処理）
//   - revoke 自体は冪等。失敗時は 500
func (h *ManageHandlers) RevokeCurrentSession(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	// path photobook_id は middleware で session.PhotobookID と一致確認済。本 handler では
	// session_id を取り出して revoke するだけ。
	s, ok := authmiddleware.SessionFromContext(r.Context())
	if !ok {
		// middleware を通った後でこのハンドラに来る前提なので不到達。万一来たら 401。
		writeJSONStatus(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}

	if err := h.revokeCurrent.Execute(r.Context(), usecase.RevokeCurrentManageSessionInput{
		SessionID: s.ID().UUID(),
	}); err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}

	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

// =============================================================================
// helpers
// =============================================================================

// parsePhotobookIDOrNotFound は URL の {id} を photobook_id に parse する。
// 不正値 / parse 失敗 → 404 を返して false。
func parsePhotobookIDOrNotFound(w http.ResponseWriter, r *http.Request) (photobook_id.PhotobookID, bool) {
	rawID := chi.URLParam(r, "id")
	if rawID == "" {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return photobook_id.PhotobookID{}, false
	}
	u, err := uuid.Parse(rawID)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return photobook_id.PhotobookID{}, false
	}
	pid, err := photobook_id.FromUUID(u)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return photobook_id.PhotobookID{}, false
	}
	return pid, true
}

// ptrTimeUTC は *time.Time の UTC 化（nil 安全）。
func ptrTimeUTC(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}
