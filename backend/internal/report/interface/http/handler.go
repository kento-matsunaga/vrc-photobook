// Package http は Report 集約の HTTP handler。
//
// 設計参照:
//   - docs/plan/m2-report-plan.md §6 / §11
//
// 仕様:
//   - POST /api/public/photobooks/{slug}/reports（Cookie 不要、Turnstile 必須）
//   - 201 / 400 / 403 / 404 / 500 を返す
//   - 失敗の理由詳細は body に出さない（敵対者対策、photobook 不在 / 公開対象外を区別なし）
//   - Cache-Control: private, no-store / X-Robots-Tag: noindex,nofollow / Set-Cookie なし
//   - reporter_contact / detail / source_ip_hash / report_id を log に出さない（report_id は内部用、
//     応答 body に含むがログ出力しない）
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"vrcpb/backend/internal/report/domain/vo/report_detail"
	"vrcpb/backend/internal/report/domain/vo/report_reason"
	"vrcpb/backend/internal/report/domain/vo/reporter_contact"
	"vrcpb/backend/internal/report/wireup"
)

const (
	bodyInvalidPayload  = `{"status":"invalid_payload"}`
	bodyTurnstileFailed = `{"status":"turnstile_failed"}`
	bodyNotFound        = `{"status":"not_found"}`
	bodyInternalError   = `{"status":"internal_error"}`
)

// rate-limited body は retry_after_seconds が動的なので関数で生成する。
// scope_hash / count / limit / IP / token は body / header に出さない。

// PublicHandlers は Report 集約の公開 HTTP handler 群。
type PublicHandlers struct {
	handlers *wireup.Handlers
}

// NewPublicHandlers は PublicHandlers を組み立てる。
func NewPublicHandlers(handlers *wireup.Handlers) *PublicHandlers {
	return &PublicHandlers{handlers: handlers}
}

// submitRequest は POST body のスキーマ。
type submitRequest struct {
	Reason          string `json:"reason"`
	Detail          string `json:"detail"`
	ReporterContact string `json:"reporter_contact"`
	TurnstileToken  string `json:"turnstile_token"`
}

// submitResponse は 201 成功時の body。
//
// セキュリティ: reporter_contact / detail / source_ip_hash は応答に含めない。
// report_id は内部識別用に返すが Frontend では表示しない方針（PR35a §16 #7）。
type submitResponse struct {
	Status   string `json:"status"`
	ReportID string `json:"report_id"`
}

// SubmitReport は POST /api/public/photobooks/{slug}/reports ハンドラ。
func (h *PublicHandlers) SubmitReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store, must-revalidate")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	slugParam := chi.URLParam(r, "slug")
	if slugParam == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(bodyInvalidPayload))
		return
	}

	var req submitRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(bodyInvalidPayload))
		return
	}

	reasonVO, err := report_reason.Parse(req.Reason)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(bodyInvalidPayload))
		return
	}
	detailVO, err := report_detail.Parse(req.Detail)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(bodyInvalidPayload))
		return
	}
	contactVO, err := reporter_contact.Parse(req.ReporterContact)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(bodyInvalidPayload))
		return
	}
	// L4: 多層防御 Turnstile ガード（`.agents/rules/turnstile-defensive-guard.md`）。
	// 空白のみのトークンも UseCase / siteverify に渡さず即拒否。
	if strings.TrimSpace(req.TurnstileToken) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(bodyInvalidPayload))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	out, err := h.handlers.Submit(ctx, wireup.SubmitReportInput{
		Slug:            slugParam,
		Reason:          reasonVO,
		Detail:          detailVO,
		ReporterContact: contactVO,
		TurnstileToken:  req.TurnstileToken,
		RemoteIP:        extractRemoteIP(r),
		Now:             time.Now().UTC(),
	})
	if err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	resp := submitResponse{Status: "submitted", ReportID: out.ReportID.String()}
	_ = json.NewEncoder(w).Encode(resp)
}

// writeError は UseCase エラーを HTTP status + body に変換する。
//
// 敵対者対策で photobook 不在 / 公開対象外 / Turnstile 失敗の内部詳細を漏らさない。
func writeError(w http.ResponseWriter, err error) {
	// PR36: UsageLimit 起因の 429（threshold / fail-closed 両方）
	var rl *wireup.RateLimited
	if errors.As(err, &rl) {
		writeRateLimited(w, rl.RetryAfterSeconds)
		return
	}
	switch {
	case errors.Is(err, wireup.ErrInvalidSlug):
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(bodyInvalidPayload))
	case errors.Is(err, wireup.ErrTurnstileTokenMissing):
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(bodyInvalidPayload))
	case errors.Is(err, wireup.ErrTurnstileVerificationFailed):
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(bodyTurnstileFailed))
	case errors.Is(err, wireup.ErrTurnstileUnavailable):
		// fail-closed: 503 ではなく 500（外部に Cloudflare 障害情報を漏らさない）
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(bodyInternalError))
	case errors.Is(err, wireup.ErrTargetNotEligibleForReport):
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(bodyNotFound))
	case errors.Is(err, wireup.ErrSaltNotConfigured):
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(bodyInternalError))
	default:
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(bodyInternalError))
	}
}

// writeRateLimited は HTTP 429 + Retry-After header + body を書き出す。
//
// セキュリティ: scope_hash / count / limit / IP / token は header / body に出さない。
func writeRateLimited(w http.ResponseWriter, retryAfterSeconds int) {
	if retryAfterSeconds < 1 {
		retryAfterSeconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	w.Header().Set("Cache-Control", "private, no-store, must-revalidate")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"status":"rate_limited","retry_after_seconds":` + strconv.Itoa(retryAfterSeconds) + `}`))
}

// extractRemoteIP は Cf-Connecting-Ip 優先 + X-Forwarded-For 末尾 + RemoteAddr の順で取得。
//
// セキュリティ:
//   - 取得した IP は UseCase 内で source_ip_hash に変換されるだけ
//   - 生 IP を log / DB / Outbox payload に書かない（呼び出し側が UseCase に渡すのみ）
func extractRemoteIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("Cf-Connecting-Ip")); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		// XFF は左→右で client → proxies。最も右に近い信頼できる proxy が左端を上書きする
		// が、Cloud Run + Cloudflare 構成では先頭が client IP（Cloudflare による上書き）。
		// 末尾末端の場合の安全策として「最も左を採用」しつつ、複数候補時は先頭を取る。
		parts := strings.Split(v, ",")
		return strings.TrimSpace(parts[0])
	}
	// RemoteAddr は host:port 形式。port 部を切り捨てる単純実装。
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}
