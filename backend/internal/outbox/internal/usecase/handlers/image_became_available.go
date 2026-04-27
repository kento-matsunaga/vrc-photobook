package handlers

import (
	"context"
	"log/slog"
	"time"

	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
)

// ImageBecameAvailableHandler は image.became_available event の no-op handler。
type ImageBecameAvailableHandler struct {
	logger *slog.Logger
}

// NewImageBecameAvailableHandler は handler を組み立てる。
func NewImageBecameAvailableHandler(logger *slog.Logger) *ImageBecameAvailableHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ImageBecameAvailableHandler{logger: logger}
}

// Handle は no-op。successful 終了。
func (h *ImageBecameAvailableHandler) Handle(ctx context.Context, ev outboxusecase.EventTarget) error {
	start := time.Now()
	h.logger.InfoContext(ctx, "outbox handler: image.became_available (no-op)",
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
