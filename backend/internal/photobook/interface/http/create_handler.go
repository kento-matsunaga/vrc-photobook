// Package http (create_handler.go) は POST /api/photobooks の HTTP handler。
//
// 設計参照:
//   - docs/plan/m2-create-entry-plan.md（採用フル: A + W + T1 + U2）
//   - 業務知識 v4 §3.1（タイプ選択時に server draft Photobook を作成し、draft_edit_token
//     を発行する。本 PR final closeout で §3.1 を改定予定）
//   - .agents/rules/turnstile-defensive-guard.md（L0-L4 多層、handler は L4）
//
// セキュリティ:
//   - request body の raw token はログに出さない
//   - response body には raw draft_edit_token を **成功時のみ** 1 度返す。Frontend が即座に
//     window.location.replace で /draft/<token> に渡し、ログ・履歴に残さない設計
//   - Set-Cookie は出さない（draft 入場は /draft/<token> route で session 交換 + Cookie 発行）
//   - すべての response に Cache-Control: no-store
//   - Turnstile 失敗 / token 空白は 403 turnstile_failed で固定文言、原因詳細を出さない
//   - 失敗時の bad_request / internal_error は固定文言で詳細を出さない（情報漏洩抑止）
package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	openingstyle "vrcpb/backend/internal/photobook/domain/vo/opening_style"
	pblayout "vrcpb/backend/internal/photobook/domain/vo/photobook_layout"
	pbtype "vrcpb/backend/internal/photobook/domain/vo/photobook_type"
	pbvisibility "vrcpb/backend/internal/photobook/domain/vo/visibility"
	"vrcpb/backend/internal/photobook/internal/usecase"
	"vrcpb/backend/internal/turnstile"
)

const (
	bodyTurnstileFailed      = `{"status":"turnstile_failed"}`
	bodyTurnstileUnavailable = `{"status":"turnstile_unavailable"}`
	// invalid_payload は HTTP 400 用。既存 handler.go の bodyBadRequest が "bad_request"
	// を出すため、本 handler では別名で "invalid_payload" 文字列を使う（Frontend 既存
	// error mapping と整合: SubmitReport / Upload / Publish の lib も "invalid_payload"）
	bodyInvalidPayload = `{"status":"invalid_payload"}`

	// title / creator_display_name の上限。業務知識 v4 §3.1 と既存 photobook 制約に揃える。
	maxTitleLen              = 100
	maxCreatorDisplayNameLen = 50

	// draft_expires_at = now + 7 day（業務知識 v4 §3.1 / §6.13）
	defaultDraftTTL = 7 * 24 * time.Hour
)

// CreateHandlers は /api/photobooks 用の HTTP handler。
type CreateHandlers struct {
	create            *usecase.CreateDraftPhotobook
	turnstileVerifier turnstile.Verifier
	turnstileHostname string
	turnstileAction   string
	clock             Clock
}

// NewCreateHandlers は CreateHandlers を組み立てる。
//
// turnstileAction は本 PR では "photobook-create" を hardcode で渡す（cmd/api/main.go
// の wireup site で固定）。env / Secret 変更は不要（既存 TURNSTILE_SECRET_KEY 流用）。
func NewCreateHandlers(
	create *usecase.CreateDraftPhotobook,
	verifier turnstile.Verifier,
	turnstileHostname string,
	turnstileAction string,
	clock Clock,
) *CreateHandlers {
	if clock == nil {
		clock = SystemClock{}
	}
	return &CreateHandlers{
		create:            create,
		turnstileVerifier: verifier,
		turnstileHostname: turnstileHostname,
		turnstileAction:   turnstileAction,
		clock:             clock,
	}
}

type createRequest struct {
	Type               string `json:"type"`
	Title              string `json:"title,omitempty"`
	CreatorDisplayName string `json:"creator_display_name,omitempty"`
	TurnstileToken     string `json:"turnstile_token"`
}

type createResponse struct {
	PhotobookID      string    `json:"photobook_id"`
	DraftEditToken   string    `json:"draft_edit_token"`
	DraftEditURLPath string    `json:"draft_edit_url_path"`
	DraftExpiresAt   time.Time `json:"draft_expires_at"`
}

// CreatePhotobook は POST /api/photobooks ハンドラ。
//
// 受入条件:
//   - body JSON decode 成功
//   - type が enum 内（event / daily / portfolio / avatar / world / memory / free）
//   - title / creator_display_name が長さ制限内（任意フィールド、空文字許容）
//   - turnstile_token が trim 後 non-empty + Cloudflare siteverify 成功
//
// 失敗時:
//   - 400 invalid_payload: JSON decode / type 不正 / 長さ超過
//   - 403 turnstile_failed: token 空白のみ / siteverify 失敗 / hostname-action 不一致
//   - 503 turnstile_unavailable: Cloudflare siteverify network error
//   - 500 internal_error: DB / draft_edit_token 生成 etc.
//
// 成功時:
//   - 201 Created + JSON body { photobook_id, draft_edit_token, draft_edit_url_path,
//     draft_expires_at }
//   - Cache-Control: no-store
func (h *CreateHandlers) CreatePhotobook(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)

	var req createRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyInvalidPayload)
		return
	}

	// L4: Turnstile 多層ガード。空白のみのトークンを Cloudflare siteverify に投げない
	if strings.TrimSpace(req.TurnstileToken) == "" {
		writeJSONStatus(w, http.StatusForbidden, bodyTurnstileFailed)
		return
	}

	// type 必須、enum チェック
	pbType, err := pbtype.Parse(req.Type)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyInvalidPayload)
		return
	}

	// title / creator_display_name は任意、長さチェックのみ
	title := strings.TrimSpace(req.Title)
	if len(title) > maxTitleLen {
		writeJSONStatus(w, http.StatusBadRequest, bodyInvalidPayload)
		return
	}
	creatorName := strings.TrimSpace(req.CreatorDisplayName)
	if len(creatorName) > maxCreatorDisplayNameLen {
		writeJSONStatus(w, http.StatusBadRequest, bodyInvalidPayload)
		return
	}

	// Turnstile siteverify
	_, err = h.turnstileVerifier.Verify(r.Context(), turnstile.VerifyInput{
		Token:    req.TurnstileToken,
		Action:   h.turnstileAction,
		Hostname: h.turnstileHostname,
	})
	if err != nil {
		if errors.Is(err, turnstile.ErrUnavailable) {
			writeJSONStatus(w, http.StatusServiceUnavailable, bodyTurnstileUnavailable)
			return
		}
		writeJSONStatus(w, http.StatusForbidden, bodyTurnstileFailed)
		return
	}

	// CreateDraftPhotobook UseCase を実行
	out, err := h.create.Execute(r.Context(), usecase.CreateDraftPhotobookInput{
		Type:               pbType,
		Title:              title,
		Layout:             pblayout.Simple(),
		OpeningStyle:       openingstyle.Light(),
		Visibility:         pbvisibility.Unlisted(),
		CreatorDisplayName: creatorName,
		RightsAgreed:       false,
		Now:                h.clock.Now(),
		DraftTTL:           defaultDraftTTL,
	})
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}

	rawToken := out.RawDraftToken.Encode()
	pbID := out.Photobook.ID().String()
	expiresAt := h.clock.Now().Add(defaultDraftTTL).UTC()
	if pa := out.Photobook; pa.DraftExpiresAt() != nil {
		expiresAt = pa.DraftExpiresAt().UTC()
	}

	writeJSON(w, http.StatusCreated, createResponse{
		PhotobookID:      pbID,
		DraftEditToken:   rawToken,
		DraftEditURLPath: "/draft/" + rawToken,
		DraftExpiresAt:   expiresAt,
	})
}
