// Package http は Photobook 集約の HTTP handler を提供する。
//
// PR9c で本番 router に接続する endpoint:
//   - POST /api/auth/draft-session-exchange
//   - POST /api/auth/manage-session-exchange
//
// 設計参照:
//   - docs/plan/m2-photobook-session-integration-plan.md §10
//   - docs/adr/0003-frontend-token-session-flow.md
//
// セキュリティ:
//   - dummy token / 認証バイパス / 固定 token を **絶対に作らない**
//   - request body の raw token はログに出さない
//   - response body には raw session_token を**成功時のみ**乗せる（Frontend Route Handler が
//     PR10 で Cookie 化する前提）
//   - Set-Cookie は本 handler から **出さない**（M1 二重出力学習との整合）
//   - 失敗時は 400 / 401 を固定文言で返し、原因詳細を出さない（情報漏洩抑止）
//   - すべてのレスポンスに Cache-Control: no-store を付与
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// 失敗時 body（固定文言）。
const (
	bodyBadRequest   = `{"status":"bad_request"}`
	bodyUnauthorized = `{"status":"unauthorized"}`
	bodyServerError  = `{"status":"internal_error"}`
)

// Clock は時刻取得を抽象化（テスト用に固定可能）。
type Clock interface {
	Now() time.Time
}

// SystemClock は time.Now を返す。
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// Handlers は Photobook の HTTP handler 群。
type Handlers struct {
	draftExchange    *usecase.ExchangeDraftTokenForSession
	manageExchange   *usecase.ExchangeManageTokenForSession
	manageSessionTTL time.Duration
	clock            Clock
}

// NewHandlers は Handlers を組み立てる。
//
// manageSessionTTL は manage session の有効期限（業務知識 v4 §6.15 / 計画 §14.3 で 7 日確定）。
func NewHandlers(
	draftExchange *usecase.ExchangeDraftTokenForSession,
	manageExchange *usecase.ExchangeManageTokenForSession,
	manageSessionTTL time.Duration,
	clock Clock,
) *Handlers {
	if clock == nil {
		clock = SystemClock{}
	}
	return &Handlers{
		draftExchange:    draftExchange,
		manageExchange:   manageExchange,
		manageSessionTTL: manageSessionTTL,
		clock:            clock,
	}
}

// === draft-session-exchange ===

type draftExchangeRequest struct {
	DraftEditToken string `json:"draft_edit_token"`
}

type draftExchangeResponse struct {
	SessionToken string    `json:"session_token"`
	PhotobookID  string    `json:"photobook_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// DraftSessionExchange は POST /api/auth/draft-session-exchange ハンドラ。
//
// 仕様:
//   - JSON decode 失敗 → 400 bad_request
//   - draft_edit_token 空 / 形式不正 / 不一致 / 期限切れ → 401 unauthorized
//   - 成功 → 200 OK + { session_token, photobook_id, expires_at }
//   - すべて Cache-Control: no-store
//   - Set-Cookie は出さない
func (h *Handlers) DraftSessionExchange(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)

	var req draftExchangeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if req.DraftEditToken == "" {
		// 空 token は形式不正として 401 に集約（情報漏洩抑止）
		writeJSONStatus(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}
	tok, err := draft_edit_token.Parse(req.DraftEditToken)
	if err != nil {
		writeJSONStatus(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}

	out, err := h.draftExchange.Execute(r.Context(), usecase.ExchangeDraftTokenForSessionInput{
		RawToken: tok,
		Now:      h.clock.Now(),
	})
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidDraftToken) {
			writeJSONStatus(w, http.StatusUnauthorized, bodyUnauthorized)
			return
		}
		// 想定外（DB 障害等）も 500 で固定文言。原因はログ側で観測（本 handler はログを出さない）
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}

	writeJSON(w, http.StatusOK, draftExchangeResponse{
		SessionToken: out.RawSessionToken.Encode(),
		PhotobookID:  out.PhotobookID.String(),
		ExpiresAt:    out.ExpiresAt.UTC(),
	})
}

// === manage-session-exchange ===

type manageExchangeRequest struct {
	ManageUrlToken string `json:"manage_url_token"`
}

type manageExchangeResponse struct {
	SessionToken        string    `json:"session_token"`
	PhotobookID         string    `json:"photobook_id"`
	ExpiresAt           time.Time `json:"expires_at"`
	TokenVersionAtIssue int       `json:"token_version_at_issue"`
}

// ManageSessionExchange は POST /api/auth/manage-session-exchange ハンドラ。
//
// 仕様:
//   - JSON decode 失敗 → 400 bad_request
//   - manage_url_token 空 / 形式不正 / 不一致 → 401 unauthorized
//   - 成功 → 200 OK + { session_token, photobook_id, expires_at, token_version_at_issue }
//   - すべて Cache-Control: no-store
//   - Set-Cookie は出さない
func (h *Handlers) ManageSessionExchange(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)

	var req manageExchangeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if req.ManageUrlToken == "" {
		writeJSONStatus(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}
	tok, err := manage_url_token.Parse(req.ManageUrlToken)
	if err != nil {
		writeJSONStatus(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}

	out, err := h.manageExchange.Execute(r.Context(), usecase.ExchangeManageTokenForSessionInput{
		RawToken:         tok,
		Now:              h.clock.Now(),
		ManageSessionTTL: h.manageSessionTTL,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidManageToken) {
			writeJSONStatus(w, http.StatusUnauthorized, bodyUnauthorized)
			return
		}
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}

	writeJSON(w, http.StatusOK, manageExchangeResponse{
		SessionToken:        out.RawSessionToken.Encode(),
		PhotobookID:         out.PhotobookID.String(),
		ExpiresAt:           out.ExpiresAt.UTC(),
		TokenVersionAtIssue: out.TokenVersionAtIssue,
	})
}

// === helpers ===

// addNoStore は Cache-Control: no-store を付与する。
//
// Cache 抑止（CDN / ブラウザに raw session_token を含む応答を保持させない）。
func addNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

// decodeJSON は body を struct に decode する。
//
// 空 body / 非 JSON は error として返す。本実装では Content-Type チェックは行わない
// （計画 §request validation: JSON decode できれば許容）。
func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// writeJSONStatus は status と固定 body を書き出す（失敗時のフォールバック用）。
func writeJSONStatus(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// writeJSON は struct を JSON で 200 系に書き出す。
//
// encoding/json の MarshalErr は理論上発生しうるが、本 handler の response は単純な型のみで
// マーシャリング失敗は実用上発生しない。万一失敗した場合は 500 + 固定 body にフォールバック。
func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

