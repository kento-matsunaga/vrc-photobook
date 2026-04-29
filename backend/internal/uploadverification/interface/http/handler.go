// Package http は upload verification の HTTP handler を提供する。
//
// 設計参照:
//   - docs/plan/m2-frontend-upload-ui-plan.md §3 / §8
//   - docs/adr/0005-image-upload-flow.md §Turnstile
//
// 公開 endpoint:
//   - POST /api/photobooks/{id}/upload-verifications
//
// 認可:
//   - draft session Cookie（middleware で context に Session を入れる前提）
//   - URL の photobook_id と context Session の photobook_id が一致すること
//   - manage session は不可（middleware が draft session のみ受ける）
//
// セキュリティ:
//   - upload_verification_token は response body にのみ含める（logs 禁止）
//   - Turnstile token / Cookie 値 / その他 raw 値はログ出力させない
//   - 失敗詳細は body に出さず、固定文言を返す
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	authmiddleware "vrcpb/backend/internal/auth/session/middleware"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/uploadverification/internal/usecase"
)

// 失敗時 body（固定文言）。
const (
	bodyBadRequest          = `{"status":"bad_request"}`
	bodyUnauthorized        = `{"status":"unauthorized"}`
	bodyVerificationFailed  = `{"status":"verification_failed"}`
	bodyTurnstileUnavail    = `{"status":"turnstile_unavailable"}`
	bodyServerError         = `{"status":"internal_error"}`
)

// Clock は時刻取得を抽象化（テスト用に固定可能）。
type Clock interface{ Now() time.Time }

// SystemClock は time.Now を返す。
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// Handlers は upload verification の HTTP handler 群。
type Handlers struct {
	issue    *usecase.IssueUploadVerificationSession
	hostname string
	action   string
	clock    Clock
}

// NewHandlers は Handlers を組み立てる。
//
// hostname: Turnstile siteverify で期待する hostname (e.g. "app.vrc-photobook.com")
// action:   Turnstile widget action (e.g. "upload")
func NewHandlers(
	issue *usecase.IssueUploadVerificationSession,
	hostname string,
	action string,
	clock Clock,
) *Handlers {
	if clock == nil {
		clock = SystemClock{}
	}
	return &Handlers{issue: issue, hostname: hostname, action: action, clock: clock}
}

// === POST /api/photobooks/{id}/upload-verifications ===

type issueRequest struct {
	TurnstileToken string `json:"turnstile_token"`
}

type issueResponse struct {
	UploadVerificationToken string `json:"upload_verification_token"`
	ExpiresAt               string `json:"expires_at"`
	AllowedIntentCount      int    `json:"allowed_intent_count"`
}

// IssueUploadVerification は POST /api/photobooks/{id}/upload-verifications。
func (h *Handlers) IssueUploadVerification(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	// URL の {id} → photobook_id
	pidRaw := chi.URLParam(r, "id")
	pidUUID, err := uuid.Parse(pidRaw)
	if err != nil {
		writeFixed(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	pid, err := photobook_id.FromUUID(pidUUID)
	if err != nil {
		writeFixed(w, http.StatusBadRequest, bodyBadRequest)
		return
	}

	// draft session middleware で context に置かれた Session の photobook_id と一致確認
	sess, ok := authmiddleware.SessionFromContext(r.Context())
	if !ok {
		writeFixed(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}
	if sess.PhotobookID().UUID() != pid.UUID() {
		writeFixed(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}

	// body parse
	var req issueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFixed(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	// L4: 多層防御 Turnstile ガード（`.agents/rules/turnstile-defensive-guard.md`）。
	// 空白のみのトークンを Cloudflare siteverify / UseCase に渡さず即拒否（PR36-0 横展開）。
	trimmedToken := strings.TrimSpace(req.TurnstileToken)
	if trimmedToken == "" {
		writeFixed(w, http.StatusBadRequest, bodyBadRequest)
		return
	}

	// remoteIP は X-Forwarded-For の末尾 / Cf-Connecting-IP（任意）
	remoteIP := remoteIPFromRequest(r)

	out, err := h.issue.Execute(r.Context(), usecase.IssueInput{
		PhotobookID:    pid,
		TurnstileToken: trimmedToken,
		RemoteIP:       remoteIP,
		Hostname:       h.hostname,
		Action:         h.action,
		Now:            h.clock.Now(),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrUploadVerificationFailed):
			writeFixed(w, http.StatusForbidden, bodyVerificationFailed)
		case errors.Is(err, usecase.ErrTurnstileUnavailable):
			writeFixed(w, http.StatusServiceUnavailable, bodyTurnstileUnavail)
		default:
			writeFixed(w, http.StatusInternalServerError, bodyServerError)
		}
		return
	}

	resp := issueResponse{
		UploadVerificationToken: out.RawToken.Encode(),
		ExpiresAt:               out.Session.ExpiresAt().UTC().Format(time.RFC3339),
		AllowedIntentCount:      out.Session.AllowedIntentCount().Int(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeFixed(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// remoteIPFromRequest は Cf-Connecting-IP / X-Forwarded-For 末尾から実 IP を取り出す。
//
// Cloudflare → Cloud Run 経由なので Cf-Connecting-IP を優先。
// 値はログ出力させないため、本関数の戻り値を slog 引数に直接乗せないこと。
func remoteIPFromRequest(r *http.Request) string {
	if v := r.Header.Get("Cf-Connecting-IP"); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		// 末尾を取る（多段 proxy 想定）
		for i := len(v) - 1; i >= 0; i-- {
			if v[i] == ',' {
				return v[i+1:]
			}
		}
		return v
	}
	return r.RemoteAddr
}
