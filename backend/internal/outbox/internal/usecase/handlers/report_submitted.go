package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
)

// ReportSubmittedHandler は report.submitted event の handler（PR35b）。
//
// MVP では **no-op + structured log**。
// `minor_safety_concern` は通知レベルを Warn 以上に上げる（v4 §3.6 / §7.4 / 計画書 §12.2）。
// Email / Slack 等の実通知は ADR-0006 後続（Email Provider 確定後）。
//
// セキュリティ:
//   - reporter_contact / detail / source_ip_hash は payload に入っていない設計
//     （SubmitReport UseCase で除外済）。本 handler は payload を decode するだけで、
//     reporter 個人情報・IP hash 完全値・Cookie・token 等は触らない
//   - log には report_id / target_photobook_id / reason / has_contact のみ
type ReportSubmittedHandler struct {
	logger *slog.Logger
}

// NewReportSubmittedHandler は handler を組み立てる。
func NewReportSubmittedHandler(logger *slog.Logger) *ReportSubmittedHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReportSubmittedHandler{logger: logger}
}

// reportSubmittedPayload は payload decode 用の最小プロジェクション。
// outbox/domain.ReportSubmittedPayload と JSON 互換。
type reportSubmittedPayload struct {
	Reason            string `json:"reason"`
	ReportID          string `json:"report_id"`
	TargetPhotobookID string `json:"target_photobook_id"`
	HasContact        bool   `json:"has_contact"`
}

// Handle は payload を decode し、reason に応じて log severity を出し分ける。
//
// 戻り値は常に nil（no-op）。decode 失敗でも nil を返さず error を返して
// worker の retry / dead 振り分けを使う（後続実装で副作用 handler に変えても安全）。
func (h *ReportSubmittedHandler) Handle(ctx context.Context, ev outboxusecase.EventTarget) error {
	start := time.Now()
	var p reportSubmittedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		// payload broken は永続 retry しても直らないため worker の MaxAttempts で dead に倒れる
		h.logger.WarnContext(ctx, "outbox handler: report.submitted payload decode failed",
			slog.String("event_id", ev.ID.String()),
			slog.String("event_type", ev.EventType),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("result", "decode_failed"),
		)
		return err
	}

	// minor_safety_concern は最優先対応カテゴリ（v4 §3.6 / §7.4）。
	// 通知レベルを上げて Cloud Run logs の severity ベース alert で拾えるようにする。
	severityIsUrgent := p.Reason == "minor_safety_concern"
	logFn := h.logger.InfoContext
	priority := "normal"
	if severityIsUrgent {
		logFn = h.logger.WarnContext
		priority = "urgent"
	}

	logFn(ctx, "outbox handler: report.submitted (no-op)",
		slog.String("event_id", ev.ID.String()),
		slog.String("event_type", ev.EventType),
		slog.String("aggregate_type", ev.AggregateType),
		slog.String("aggregate_id", ev.AggregateID.String()),
		slog.String("report_id", p.ReportID),
		slog.String("target_photobook_id", p.TargetPhotobookID),
		slog.String("reason", p.Reason),
		slog.Bool("has_contact", p.HasContact),
		slog.String("priority", priority),
		slog.Int("attempts", ev.Attempts),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		slog.String("result", "ok"),
	)
	return nil
}
