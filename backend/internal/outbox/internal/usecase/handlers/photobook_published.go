// handlers パッケージは outbox-worker が dispatch する event ごとの handler 群。
//
// PR33d で `photobook.published` handler は **OGP 生成の副作用を持つ** 形に変更した。
// 他 2 種（image.became_available / image.failed）は引き続き no-op + structured log。
//
// 副作用 handler の責務分離:
//   - 本パッケージは photobook.published payload の decode + OgpGenerator 呼び出し
//   - 実 OGP 生成（renderer / R2 PUT / DB 更新）は ogp/internal/usecase 配下で完結
//   - 接続は cmd 層が ogp/wireup から adapter を組み立てて outbox/wireup に注入
//
// セキュリティ:
//   - payload 全文をログに出さない（サマリ field + photobook_id まで）
//   - token / Cookie / manage URL / presigned URL / storage_key 完全値 / R2 credentials /
//     DATABASE_URL / Secret payload はログに出さない
//   - event_id / event_type / aggregate_type / aggregate_id / ogp_image_id /
//     duration_ms / attempts / result は出してよい
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/outbox/contract"
	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
)

// 本パッケージで使う interface / sentinel は contract package（外部からも見える）に
// 切り出してある。実装は ogp/wireup の adapter が contract.OgpGenerator を満たす。

// photobookPublishedPayload は ogp/domain.PhotobookPublishedPayload と JSON 互換。
// handler は photobook_id だけ使うので最小フィールドのみ受け取る。
type photobookPublishedPayload struct {
	PhotobookID string `json:"photobook_id"`
}

// PhotobookPublishedHandler は photobook.published event の OGP 生成 handler。
type PhotobookPublishedHandler struct {
	logger    *slog.Logger
	generator contract.OgpGenerator
}

// NewPhotobookPublishedHandler は handler を組み立てる。
func NewPhotobookPublishedHandler(generator contract.OgpGenerator, logger *slog.Logger) *PhotobookPublishedHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PhotobookPublishedHandler{logger: logger, generator: generator}
}

// Handle は payload から photobook_id を取り出し、OGP 生成を呼ぶ。
//
// 戻り値:
//   - nil（成功）: outbox worker が MarkProcessed
//   - nil（NotPublic）: 永続 skip として processed 扱い（worker は ErrNotPublishedSkippable
//     を errors.Is で判定して MarkProcessed に倒すことも検討。本実装では handler が
//     nil 返却で processed に流す方針）
//   - non-nil: outbox worker が attempts++ で MarkFailedRetry / MarkDead に振り分け
func (h *PhotobookPublishedHandler) Handle(ctx context.Context, ev outboxusecase.EventTarget) error {
	start := time.Now()

	var p photobookPublishedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		// payload broken は **永続 retry しても直らない**ため failed → dead 行きでよい。
		// handler は error を返し、worker の MaxAttempts でいずれ dead に倒れる。
		return fmt.Errorf("decode payload: %w", err)
	}
	pid, err := uuid.Parse(p.PhotobookID)
	if err != nil {
		return fmt.Errorf("invalid photobook_id in payload: %w", err)
	}

	res, err := h.generator.GenerateForPhotobook(ctx, pid, time.Now().UTC())
	if err != nil {
		if errors.Is(err, contract.ErrNotPublishedSkippable) {
			// 永続 skip。retry しても同じ結果なので processed に倒す（nil 返却）。
			h.logger.InfoContext(ctx, "outbox handler: photobook.published (skipped not-public)",
				slog.String("event_id", ev.ID.String()),
				slog.String("event_type", ev.EventType),
				slog.String("aggregate_type", ev.AggregateType),
				slog.String("aggregate_id", ev.AggregateID.String()),
				slog.Int("attempts", ev.Attempts),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.String("result", "skipped_not_public"),
			)
			return nil
		}
		// その他の error は worker が retry / dead に振り分け。
		return err
	}

	h.logger.InfoContext(ctx, "outbox handler: photobook.published (ogp generated)",
		slog.String("event_id", ev.ID.String()),
		slog.String("event_type", ev.EventType),
		slog.String("aggregate_type", ev.AggregateType),
		slog.String("aggregate_id", ev.AggregateID.String()),
		slog.String("ogp_image_id", res.OgpImageID.String()),
		slog.Bool("generated", res.Generated),
		slog.Int("attempts", ev.Attempts),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		slog.String("result", "ok"),
	)
	return nil
}
