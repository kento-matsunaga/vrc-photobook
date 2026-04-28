package handlers

import (
	"context"
	"log/slog"
	"time"

	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
)

// PhotobookHiddenHandler は photobook.hidden event の no-op handler（PR34b）。
// 副作用（CDN purge / OGP cache invalidation）は後続 PR で追加する。
type PhotobookHiddenHandler struct {
	logger *slog.Logger
}

// NewPhotobookHiddenHandler は handler を組み立てる。
func NewPhotobookHiddenHandler(logger *slog.Logger) *PhotobookHiddenHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PhotobookHiddenHandler{logger: logger}
}

// Handle は no-op + structured log。successful 終了。
func (h *PhotobookHiddenHandler) Handle(ctx context.Context, ev outboxusecase.EventTarget) error {
	start := time.Now()
	h.logger.InfoContext(ctx, "outbox handler: photobook.hidden (no-op)",
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
