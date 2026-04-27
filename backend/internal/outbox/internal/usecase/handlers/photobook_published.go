// handlers パッケージは outbox-worker が dispatch する event ごとの handler 群。
//
// 現状すべて副作用を持たず、structured log のみ出す（email provider が ADR-0006 で
// 再選定中のため、副作用を伴う handler は未実装）。
// 将来の拡張:
//   - photobook.published     → OGP 再生成 / Analytics / email provider 確定後の通知
//   - image.became_available  → OGP 再生成 / viewer cache refresh
//   - image.failed            → cleanup 候補 / admin visibility / notification
//
// セキュリティ:
//   - payload 全文をログに出さない（サマリ field のみ）
//   - token / Cookie / manage URL / presigned URL / storage_key 完全値 / R2 credentials /
//     DATABASE_URL / Secret payload はログに出さない
//   - event_id / event_type / aggregate_type / aggregate_id / duration_ms / attempts は出してよい
package handlers

import (
	"context"
	"log/slog"
	"time"

	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
)

// PhotobookPublishedHandler は photobook.published event の no-op handler。
type PhotobookPublishedHandler struct {
	logger *slog.Logger
}

// NewPhotobookPublishedHandler は handler を組み立てる。logger は worker から共有される。
func NewPhotobookPublishedHandler(logger *slog.Logger) *PhotobookPublishedHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PhotobookPublishedHandler{logger: logger}
}

// Handle は no-op。successful 終了。
func (h *PhotobookPublishedHandler) Handle(ctx context.Context, ev outboxusecase.EventTarget) error {
	start := time.Now()
	h.logger.InfoContext(ctx, "outbox handler: photobook.published (no-op)",
		slog.String("event_id", ev.ID.String()),
		slog.String("event_type", ev.EventType),
		slog.String("aggregate_type", ev.AggregateType),
		slog.String("aggregate_id", ev.AggregateID.String()),
		slog.Int("attempts", ev.Attempts),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		slog.String("result", "ok"),
	)
	return nil
}
