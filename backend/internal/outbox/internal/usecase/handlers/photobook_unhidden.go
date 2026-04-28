package handlers

import (
	"context"
	"log/slog"
	"time"

	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
)

// PhotobookUnhiddenHandler は photobook.unhidden event の no-op handler（PR34b）。
// OGP 再生成 / CDN purge 等の副作用は後続 PR で追加する。
type PhotobookUnhiddenHandler struct {
	logger *slog.Logger
}

// NewPhotobookUnhiddenHandler は handler を組み立てる。
func NewPhotobookUnhiddenHandler(logger *slog.Logger) *PhotobookUnhiddenHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PhotobookUnhiddenHandler{logger: logger}
}

// Handle は no-op + structured log。
func (h *PhotobookUnhiddenHandler) Handle(ctx context.Context, ev outboxusecase.EventTarget) error {
	start := time.Now()
	h.logger.InfoContext(ctx, "outbox handler: photobook.unhidden (no-op)",
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
