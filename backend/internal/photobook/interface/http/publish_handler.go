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
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"vrcpb/backend/internal/photobook/domain"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// PublishHandlers は publish endpoint の HTTP handler。
type PublishHandlers struct {
	publish    *usecase.PublishFromDraft
	ipHashSalt string // PR36: REPORT_IP_HASH_SALT_V1 流用、空なら UsageLimit skip
}

// NewPublishHandlers は PublishHandlers を組み立てる。
//
// PR36: ipHashSalt（REPORT_IP_HASH_SALT_V1）を渡すと publish の UsageLimit が有効化される。
// 空文字なら UsageLimit を skip。
func NewPublishHandlers(publish *usecase.PublishFromDraft, ipHashSalt string) *PublishHandlers {
	return &PublishHandlers{publish: publish, ipHashSalt: ipHashSalt}
}

type publishRequest struct {
	ExpectedVersion int `json:"expected_version"`
	// 2026-05-03 STOP α P0 v2: 公開前の権利・配慮確認同意（業務知識 v4 §3.1）。
	// false / 不在の場合は 409 publish_precondition_failed reason=rights_not_agreed を返す。
	// true の場合は publish と同 TX で rights_agreed=true / rights_agreed_at=now を DB 永続化。
	RightsAgreed bool `json:"rights_agreed"`
}

type publishResponse struct {
	PhotobookID    string    `json:"photobook_id"`
	Slug           string    `json:"slug"`
	PublicURLPath  string    `json:"public_url_path"`  // "/p/{slug}"
	ManageURLPath  string    `json:"manage_url_path"`  // "/manage/token/{raw}"。**再表示しない、UI 即時提示**
	PublishedAt    time.Time `json:"published_at"`
}

// publishPreconditionFailedBody は authenticated owner 向けの 409 response body。
//
// 2026-05-03 STOP α P0 v2: 「draft 以外」「rights 未同意」「creator 空」「title 空」などを
// reason enum で開示する。draft session 必須経路で攻撃者は到達できないため、敵対者観測
// 抑止より UX 復旧導線を優先（業務知識 v4 §3.1 / §6 manage URL 別との整合）。
//
// reason 値:
//   - "rights_not_agreed": 権利・配慮確認 checkbox 未同意
//   - "not_draft": status が draft でない（既に published / deleted）
//   - "empty_creator": creator_display_name 空（a8fe0db 後は dead code、B 案で再活用）
//   - "empty_title": title 空（dead code、B 案で再活用）
//   - "unknown_precondition": 想定外への safeguard
type publishPreconditionFailedBody struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type publishVersionConflictBody struct {
	Status string `json:"status"`
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
		RightsAgreed:    req.RightsAgreed,
		Now:             time.Now().UTC(),
		RemoteIP:        publishRemoteIP(r),
		IPHashSalt:      h.ipHashSalt,
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

// writePublishError は publish 系 error を HTTP status / 構造化 body に変換する。
//
// 2026-05-03 STOP α P0 v2: 旧来は全 409 を `bodyConflict` (status=conflict のみ) に
// 集約していたが、authenticated owner にとって復旧導線を出せず UX が破綻していた
// （「公開条件に合致しません」のみで何を直せばよいか分からない）。
//
// 新方針:
//   - version_conflict: OCC 違反 / FindByID 不在 / repository OCC / pg unique。
//     Frontend で「最新を取得」CTA を出す対象。
//   - publish_precondition_failed (reason 付き): rights 未同意 / draft 以外 / etc。
//     authenticated owner 向けに reason enum で開示。raw ID / 内部詳細は出さない。
//
// /publish は draft session 必須で攻撃者は到達不能 → 敵対者観測抑止より UX 優先。
func writePublishError(w http.ResponseWriter, err error) {
	// PR36: UsageLimit 起因の 429（threshold / fail-closed）
	var rl *usecase.PublishRateLimited
	if errors.As(err, &rl) {
		writePublishRateLimited(w, rl.RetryAfterSeconds)
		return
	}
	switch {
	// publish_precondition_failed (reason 開示)
	case errors.Is(err, domain.ErrRightsNotAgreed):
		writePublishPrecondition(w, "rights_not_agreed")
	case errors.Is(err, domain.ErrNotDraft),
		errors.Is(err, photobookrdb.ErrNotDraft):
		writePublishPrecondition(w, "not_draft")
	case errors.Is(err, domain.ErrEmptyCreatorName):
		// a8fe0db 後は dead path（CanPublish が creator 空をチェックしない）。
		// B 案 (/edit に creator 入力欄追加) で再活性化する想定で reason 維持。
		writePublishPrecondition(w, "empty_creator")
	case errors.Is(err, domain.ErrEmptyTitle):
		writePublishPrecondition(w, "empty_title")

	// version_conflict (OCC / 状態競合)
	case errors.Is(err, usecase.ErrPublishConflict),
		errors.Is(err, photobookrdb.ErrOptimisticLockConflict):
		writePublishVersionConflict(w)

	// not_found (FindByID 経由で生で抜けるパスは現状無いが defensive)
	case errors.Is(err, photobookrdb.ErrNotFound):
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)

	default:
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// slug 衝突等は version_conflict として扱う（極稀、MinimalSlugGenerator では事実上発生しない）
			writePublishVersionConflict(w)
			return
		}
		writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
	}
}

// writePublishPrecondition は 409 + publish_precondition_failed body を書き出す。
func writePublishPrecondition(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusConflict)
	body := publishPreconditionFailedBody{
		Status: "publish_precondition_failed",
		Reason: reason,
	}
	_ = json.NewEncoder(w).Encode(body)
}

// writePublishVersionConflict は 409 + version_conflict body を書き出す。
func writePublishVersionConflict(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusConflict)
	body := publishVersionConflictBody{Status: "version_conflict"}
	_ = json.NewEncoder(w).Encode(body)
}

// writePublishRateLimited は HTTP 429 + Retry-After を書き出す（PR36）。
//
// セキュリティ: scope_hash / count / limit / IP / token は header / body に出さない。
func writePublishRateLimited(w http.ResponseWriter, retryAfterSeconds int) {
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

// publishRemoteIP は publish endpoint で UsageLimit 用に Remote IP を取り出す。
//
// セキュリティ: 戻り値の生 IP は UseCase 内で salt + sha256 → hex 化されてから保存される。
// 本関数の戻り値を logs に直接出さない（呼び出し側で usage_counters の scope_hash 経由のみ）。
func publishRemoteIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("Cf-Connecting-Ip")); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		// 先頭が client IP（Cloudflare 経由前提、PR35b と同方針）
		parts := strings.Split(v, ",")
		return strings.TrimSpace(parts[0])
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}
