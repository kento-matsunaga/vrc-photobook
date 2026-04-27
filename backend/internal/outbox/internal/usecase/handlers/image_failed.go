package handlers

import (
	"context"
	"log/slog"
	"time"

	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
)

// ImageFailedHandler は image.failed event の no-op handler。
type ImageFailedHandler struct {
	logger *slog.Logger
}

// NewImageFailedHandler は handler を組み立てる。
func NewImageFailedHandler(logger *slog.Logger) *ImageFailedHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ImageFailedHandler{logger: logger}
}

// Handle は no-op。successful 終了。
func (h *ImageFailedHandler) Handle(ctx context.Context, ev outboxusecase.EventTarget) error {
	start := time.Now()
	h.logger.InfoContext(ctx, "outbox handler: image.failed (no-op)",
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
