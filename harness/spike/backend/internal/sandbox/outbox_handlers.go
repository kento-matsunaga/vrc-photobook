// Outbox + 自動 reconciler 起動基盤の sandbox エンドポイント（M1 priority 7）。
//
// 設計上の対応:
//   - cross-cutting/outbox.md §6 ワーカー実装方針（FOR UPDATE SKIP LOCKED）
//   - cross-cutting/reconcile-scripts.md §3.7.2 outbox_failed_retry（自動 reconciler）
//
// セキュリティ方針:
//   - payload 全文はレスポンス・ログに出さない（一覧 API は payload を返さない）
//   - last_error 詳細はサーバ側の slog にのみ残し、クライアントには分類キーのみを返す
//   - presigned URL / Secret / token を payload に入れない（呼び出し側の責務）
package sandbox

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/spike-backend/internal/db/sqlcgen"
)

// outboxEnqueueRequest は POST /sandbox/outbox/enqueue のリクエスト body。
//
// payload は最小 JSON を想定（PoC では値の中身は問わない）。本実装では
// 集約のドメインメソッド側がドメインイベントを構築する。
type outboxEnqueueRequest struct {
	EventType     string          `json:"event_type"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	Payload       json.RawMessage `json:"payload"`
}

// outboxEnqueueResponse は発行済みイベントの最小要約。
type outboxEnqueueResponse struct {
	ID            string `json:"id"`
	EventType     string `json:"event_type"`
	AggregateType string `json:"aggregate_type"`
	AggregateID   string `json:"aggregate_id"`
	Status        string `json:"status"`
	Attempts      int32  `json:"attempts"`
}

// OutboxEnqueue はテスト用に outbox_events へ単発 INSERT する。
//
// 本実装での同一 TX INSERT は ApplicationService 側で担保（cross-cutting/outbox.md §2）。
// PoC では sandbox 経由で単独 INSERT し、後続の process-once フローを検証する。
func OutboxEnqueue(queries *sqlcgen.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if queries == nil {
			writeError(w, http.StatusServiceUnavailable, "outbox_not_configured")
			return
		}

		var req outboxEnqueueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if strings.TrimSpace(req.EventType) == "" {
			writeError(w, http.StatusBadRequest, "event_type_required")
			return
		}
		if strings.TrimSpace(req.AggregateType) == "" {
			writeError(w, http.StatusBadRequest, "aggregate_type_required")
			return
		}
		aggUUID, err := uuid.Parse(strings.TrimSpace(req.AggregateID))
		if err != nil {
			writeError(w, http.StatusBadRequest, "aggregate_id_invalid")
			return
		}

		eventID, err := newPgUUIDv4()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "event_id_generation_failed")
			return
		}

		// payload 未指定なら空 JSON を入れる。中身は検証しない（PoC のため）。
		payload := []byte(req.Payload)
		if len(payload) == 0 {
			payload = []byte("{}")
		}

		row, err := queries.CreateOutboxEvent(r.Context(), sqlcgen.CreateOutboxEventParams{
			ID:            eventID,
			EventType:     strings.TrimSpace(req.EventType),
			AggregateType: strings.TrimSpace(req.AggregateType),
			AggregateID:   toPgUUID(aggUUID),
			Payload:       payload,
		})
		if err != nil {
			slog.Warn("outbox enqueue failed", "error", err.Error())
			writeError(w, http.StatusInternalServerError, "outbox_enqueue_failed")
			return
		}

		writeJSON(w, http.StatusOK, outboxEnqueueResponse{
			ID:            uuid.UUID(row.ID.Bytes).String(),
			EventType:     row.EventType,
			AggregateType: row.AggregateType,
			AggregateID:   uuid.UUID(row.AggregateID.Bytes).String(),
			Status:        row.Status,
			Attempts:      row.Attempts,
		})
	}
}

// outboxProcessOnceResponse はバッチ処理の集計結果。
type outboxProcessOnceResponse struct {
	Claimed   int      `json:"claimed"`
	Processed int      `json:"processed"`
	Failed    int      `json:"failed"`
	EventIDs  []string `json:"event_ids"`
}

// OutboxProcessOnce は pending イベントを最大 limit 件 claim して mock 処理する。
//
// PoC のハンドラ規則:
//   - event_type が "ForceFail" を含む → MarkOutboxFailed（failed に集約）
//   - それ以外（例: ImageIngestionRequested） → MarkOutboxProcessed
//
// 本実装では event_type ごとのハンドラルーティングが入る（cross-cutting/outbox.md §6.2）。
func OutboxProcessOnce(queries *sqlcgen.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if queries == nil {
			writeError(w, http.StatusServiceUnavailable, "outbox_not_configured")
			return
		}

		limit := int32(10)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if v, err := parseInt32(raw); err == nil && v > 0 && v <= 1000 {
				limit = v
			}
		}

		claimed, err := queries.ClaimPendingOutboxEvents(r.Context(), limit)
		if err != nil {
			slog.Warn("outbox claim failed", "error", err.Error())
			writeError(w, http.StatusInternalServerError, "outbox_claim_failed")
			return
		}

		resp := outboxProcessOnceResponse{
			Claimed:  len(claimed),
			EventIDs: make([]string, 0, len(claimed)),
		}

		for _, ev := range claimed {
			id := uuid.UUID(ev.ID.Bytes).String()
			resp.EventIDs = append(resp.EventIDs, id)
			if strings.Contains(ev.EventType, "ForceFail") {
				errMsg := "mock_forced_fail"
				if err := queries.MarkOutboxFailed(r.Context(), sqlcgen.MarkOutboxFailedParams{
					ID:        ev.ID,
					LastError: &errMsg,
				}); err != nil {
					slog.Warn("outbox mark failed error", "error", err.Error())
					continue
				}
				resp.Failed++
				continue
			}
			if err := queries.MarkOutboxProcessed(r.Context(), ev.ID); err != nil {
				slog.Warn("outbox mark processed error", "error", err.Error())
				continue
			}
			resp.Processed++
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// outboxRetryFailedResponse は outbox_failed_retry 自動 reconciler の最小実装結果。
type outboxRetryFailedResponse struct {
	Requeued int64 `json:"requeued"`
}

// OutboxRetryFailed は failed 状態のイベントを pending に戻す
// （cross-cutting/reconcile-scripts.md §3.7.2 outbox_failed_retry の最小実装）。
func OutboxRetryFailed(queries *sqlcgen.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if queries == nil {
			writeError(w, http.StatusServiceUnavailable, "outbox_not_configured")
			return
		}
		n, err := queries.RetryFailedOutboxEvents(r.Context())
		if err != nil {
			slog.Warn("outbox retry-failed failed", "error", err.Error())
			writeError(w, http.StatusInternalServerError, "outbox_retry_failed")
			return
		}
		writeJSON(w, http.StatusOK, outboxRetryFailedResponse{Requeued: n})
	}
}

// outboxListItem は GET /sandbox/outbox/list の最小要素。
// payload / last_error は含めない（情報漏えい抑止）。
type outboxListItem struct {
	ID            string  `json:"id"`
	EventType     string  `json:"event_type"`
	AggregateType string  `json:"aggregate_type"`
	AggregateID   string  `json:"aggregate_id"`
	Status        string  `json:"status"`
	Attempts      int32   `json:"attempts"`
	CreatedAt     string  `json:"created_at"`
	ProcessedAt   *string `json:"processed_at,omitempty"`
}

// outboxListResponse はリスト + status 別件数の集計を返す。
type outboxListResponse struct {
	Total    int                  `json:"total"`
	ByStatus map[string]int64     `json:"by_status"`
	Events   []outboxListItem     `json:"events"`
}

// OutboxList は最近の outbox イベント一覧と status 別件数を返す（payload は返さない）。
func OutboxList(queries *sqlcgen.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if queries == nil {
			writeError(w, http.StatusServiceUnavailable, "outbox_not_configured")
			return
		}

		limit := int32(50)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if v, err := parseInt32(raw); err == nil && v > 0 && v <= 1000 {
				limit = v
			}
		}

		rows, err := queries.ListOutboxEvents(r.Context(), limit)
		if err != nil {
			slog.Warn("outbox list failed", "error", err.Error())
			writeError(w, http.StatusInternalServerError, "outbox_list_failed")
			return
		}
		counts, err := queries.CountOutboxEventsByStatus(r.Context())
		if err != nil {
			slog.Warn("outbox count failed", "error", err.Error())
			writeError(w, http.StatusInternalServerError, "outbox_count_failed")
			return
		}

		resp := outboxListResponse{
			Total:    len(rows),
			ByStatus: make(map[string]int64, len(counts)),
			Events:   make([]outboxListItem, 0, len(rows)),
		}
		for _, c := range counts {
			resp.ByStatus[c.Status] = c.Count
		}
		for _, row := range rows {
			item := outboxListItem{
				ID:            uuid.UUID(row.ID.Bytes).String(),
				EventType:     row.EventType,
				AggregateType: row.AggregateType,
				AggregateID:   uuid.UUID(row.AggregateID.Bytes).String(),
				Status:        row.Status,
				Attempts:      row.Attempts,
				CreatedAt:     row.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
			}
			if row.ProcessedAt.Valid {
				ts := row.ProcessedAt.Time.Format("2006-01-02T15:04:05Z07:00")
				item.ProcessedAt = &ts
			}
			resp.Events = append(resp.Events, item)
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// OutboxResetForTest は PoC 検証用に outbox_events を全件削除する。
// 本実装には流用しない。
func OutboxResetForTest(queries *sqlcgen.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if queries == nil {
			writeError(w, http.StatusServiceUnavailable, "outbox_not_configured")
			return
		}
		if err := queries.ResetOutboxForTest(r.Context()); err != nil {
			slog.Warn("outbox reset failed", "error", err.Error())
			writeError(w, http.StatusInternalServerError, "outbox_reset_failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// parseInt32 は文字列を int32 に変換する。失敗時はエラー。
func parseInt32(raw string) (int32, error) {
	v := int64(0)
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, errors.New("not_int")
		}
		v = v*10 + int64(c-'0')
		if v > 0x7fffffff {
			return 0, errors.New("overflow")
		}
	}
	return int32(v), nil
}

// 静的検査用: pgx を import 済みであることを保証（将来の TX 拡張用）。
var _ = pgx.ErrNoRows
var _ pgtype.UUID
