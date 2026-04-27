// Package usecase は image-processor の UseCase を提供する。
//
// 設計参照:
//   - docs/plan/m2-image-processor-plan.md §6 / §7 / §10 / §10A / §10B
//   - docs/adr/0005-image-upload-flow.md
//   - docs/design/aggregates/image/ドメイン設計.md §4
//
// 公開する UseCase:
//   - ProcessImage: 単一 Image に対する原本取得 → decode → resize → encode → R2 PUT →
//     DB MarkAvailable + AttachVariant(display+thumbnail) → R2 DELETE(original prefix)
//   - ProcessPending: status=processing の Image を最大 N 件まで claim して逐次 ProcessImage
//
// セキュリティ:
//   - storage_key / R2 credentials / file 内容はログに出さない（plan §10B.2）
//   - 失敗理由は failure_reason VO の固定値のみで表現する
package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	imagedomain "vrcpb/backend/internal/image/domain"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/failure_reason"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/image_format"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/image/domain/vo/mime_type"
	"vrcpb/backend/internal/image/domain/vo/normalized_format"
	"vrcpb/backend/internal/image/domain/vo/storage_key"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/imageprocessor/infrastructure/imaging"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	outboxdomain "vrcpb/backend/internal/outbox/domain"
	outboxaggregate "vrcpb/backend/internal/outbox/domain/vo/aggregate_type"
	outboxevent "vrcpb/backend/internal/outbox/domain/vo/event_type"
	outboxrdb "vrcpb/backend/internal/outbox/infrastructure/repository/rdb"
)

// 共通エラー（呼び出し側に区別を必要とする粒度のみ）。
var (
	// ErrImageNotFound: claim 対象なし / id 不一致。
	ErrImageNotFound = errors.New("image not found")
	// ErrImageNotProcessing: status != processing（既に available / failed / deleted など）。
	ErrImageNotProcessing = errors.New("image is not in processing state")
	// ErrR2Unavailable: R2 接続失敗（fail-closed、retry 余地あり）。
	ErrR2Unavailable = errors.New("r2 unavailable")
	// ErrProcessFailed: 画像処理に失敗し、Image を MarkFailed にした後に呼び出し側へ通知する用。
	// failure_reason VO は ErrorWithReason 経由で取得する。
	ErrProcessFailed = errors.New("image processing failed")
)

// ErrorWithReason は MarkFailed と同時に呼び出し側へ failure_reason を伝える wrap error。
type ErrorWithReason struct {
	Reason failure_reason.FailureReason
	Inner  error
}

func (e *ErrorWithReason) Error() string {
	if e.Inner == nil {
		return fmt.Sprintf("processing failed: reason=%s", e.Reason.String())
	}
	return fmt.Sprintf("processing failed: reason=%s: %v", e.Reason.String(), e.Inner)
}

func (e *ErrorWithReason) Unwrap() error { return ErrProcessFailed }

// ProcessImageInput は単一 Image 処理の引数。
type ProcessImageInput struct {
	ImageID image_id.ImageID
	Now     time.Time
}

// ProcessImageOutput は処理結果。
type ProcessImageOutput struct {
	ImageID      image_id.ImageID
	Status       string // "available" / "failed"
	VariantCount int
}

// ProcessImage は単一 Image を完了状態 (available / failed) に進める UseCase。
//
// 大まかなフロー（plan §10）:
//  1. TX 内で Image を FindByID → status==processing かを確認
//  2. R2 ListObjects で原本 key を解決（images table に storage_key を持たないため）
//  3. R2 GetObject → imaging.Decode → 寸法検証 → JPEG re-encode (display + thumbnail)
//  4. R2 PutObject(display) → R2 PutObject(thumbnail)
//  5. DB UPDATE: MarkAvailable + AttachVariant(display) + AttachVariant(thumbnail)
//  6. TX commit
//  7. R2 DeleteObject(original)（失敗しても DB は available のまま、orphan は PR25 cleanup）
//
// 失敗パターンと state 遷移（plan §10A.2）:
//   - 原本 R2 で 0 件 / NoSuchKey → MarkFailed(object_not_found)
//   - HEIC（source_format=heic） → MarkFailed(unsupported_format) （PR23 では libheif 未統合）
//   - decode 失敗 → MarkFailed(decode_failed)
//   - format 不一致 → MarkFailed(unsupported_format)
//   - 寸法 / size 範囲外 → MarkFailed(dimensions_too_large or file_too_large)
//   - encode 失敗 → MarkFailed(variant_generation_failed)
//   - R2 PUT 失敗 → TX rollback、status=processing のまま retry 可能
//   - DB UPDATE 失敗 → TX rollback、display/thumbnail は R2 orphan（PR25 cleanup）
type ProcessImage struct {
	pool     *pgxpool.Pool
	r2Client r2.Client
	logger   *slog.Logger
}

// NewProcessImage は UseCase を組み立てる。
func NewProcessImage(pool *pgxpool.Pool, r2Client r2.Client, logger *slog.Logger) *ProcessImage {
	if logger == nil {
		logger = slog.Default()
	}
	return &ProcessImage{pool: pool, r2Client: r2Client, logger: logger}
}

// Execute は単一 Image を処理する。
func (u *ProcessImage) Execute(ctx context.Context, in ProcessImageInput) (ProcessImageOutput, error) {
	start := time.Now()

	// 状態確認は TX 外で先に。FindByID は variants 込で読むが claim 用 SELECT FOR UPDATE は使わない
	// （pool 越しに重い処理を行うため、claim は別 TX で短く完結させる方針 plan §10.8）。
	repo := imagerdb.NewImageRepository(u.pool)
	img, err := repo.FindByID(ctx, in.ImageID)
	if err != nil {
		if errors.Is(err, imagerdb.ErrNotFound) {
			return ProcessImageOutput{}, ErrImageNotFound
		}
		return ProcessImageOutput{}, err
	}
	if !img.IsProcessing() {
		return ProcessImageOutput{ImageID: img.ID(), Status: img.Status().String()}, ErrImageNotProcessing
	}

	// HEIC は MVP で未対応 → 即 MarkFailed(unsupported_format)。
	if img.SourceFormat().Equal(image_format.Heic()) {
		return u.failAndReturn(ctx, img, failure_reason.UnsupportedFormat(), in.Now, start, "heic_unsupported")
	}

	// 原本 key を prefix から解決（plan §8.2: ListObjectsV2）。
	originalPrefix := fmt.Sprintf("photobooks/%s/images/%s/original/",
		img.OwnerPhotobookID().String(), img.ID().String())
	listOut, err := u.r2Client.ListObjects(ctx, originalPrefix)
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("%w: list original: %w", ErrR2Unavailable, err)
	}
	if len(listOut.Keys) == 0 {
		return u.failAndReturn(ctx, img, failure_reason.ObjectNotFound(), in.Now, start, "original_not_found")
	}
	originalKey := listOut.Keys[0]

	// R2 GetObject → decode → encode（CPU / network、TX 外）。
	getOut, err := u.r2Client.GetObject(ctx, originalKey)
	if err != nil {
		if errors.Is(err, r2.ErrObjectNotFound) {
			return u.failAndReturn(ctx, img, failure_reason.ObjectNotFound(), in.Now, start, "original_not_found")
		}
		return ProcessImageOutput{}, fmt.Errorf("%w: get original: %w", ErrR2Unavailable, err)
	}
	body, readErr := readAllAndClose(getOut.Body)
	if readErr != nil {
		return ProcessImageOutput{}, fmt.Errorf("%w: read original: %w", ErrR2Unavailable, readErr)
	}

	originalByteSize, err := byte_size.New(int64(len(body)))
	if err != nil {
		return u.failAndReturn(ctx, img, failure_reason.FileTooLarge(), in.Now, start, "byte_size_invalid")
	}

	expectedFormat := sourceFormatToImaging(img.SourceFormat())
	if expectedFormat == "" {
		return u.failAndReturn(ctx, img, failure_reason.UnsupportedFormat(), in.Now, start, "source_format_unknown")
	}

	decoded, err := imaging.Decode(bytes.NewReader(body), expectedFormat)
	if err != nil {
		switch {
		case errors.Is(err, imaging.ErrUnsupportedFormat):
			return u.failAndReturn(ctx, img, failure_reason.UnsupportedFormat(), in.Now, start, "decode_format_mismatch")
		default:
			return u.failAndReturn(ctx, img, failure_reason.DecodeFailed(), in.Now, start, "decode_failed")
		}
	}

	originalDims, err := image_dimensions.New(decoded.Width, decoded.Height)
	if err != nil {
		return u.failAndReturn(ctx, img, failure_reason.DimensionsTooLarge(), in.Now, start, "dimensions_invalid")
	}

	// display / thumbnail encode。
	display, err := imaging.EncodeJPEG(decoded.Image, imaging.DisplayLongSide, imaging.DisplayQuality)
	if err != nil {
		return u.failAndReturn(ctx, img, failure_reason.VariantGenerationFailed(), in.Now, start, "encode_display_failed")
	}
	thumbnail, err := imaging.EncodeJPEG(decoded.Image, imaging.ThumbnailLongSide, imaging.ThumbnailQuality)
	if err != nil {
		return u.failAndReturn(ctx, img, failure_reason.VariantGenerationFailed(), in.Now, start, "encode_thumbnail_failed")
	}

	// storage_key（display / thumbnail、JPEG 固定）。
	displayKey, err := storage_key.GenerateForVariant(img.OwnerPhotobookID(), img.ID(), variant_kind.Display())
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("generate display storage_key: %w", err)
	}
	thumbnailKey, err := storage_key.GenerateForVariant(img.OwnerPhotobookID(), img.ID(), variant_kind.Thumbnail())
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("generate thumbnail storage_key: %w", err)
	}

	// R2 PUT は DB TX の外で先に実行する（PUT 成功後に DB を確定する R1 案、plan §11）。
	// PUT 失敗は ErrR2Unavailable で返し、Image は processing のまま（次回 retry）。
	if err := u.r2Client.PutObject(ctx, r2.PutObjectInput{
		StorageKey:  displayKey.String(),
		ContentType: "image/jpeg",
		Body:        display.Body,
	}); err != nil {
		return ProcessImageOutput{}, fmt.Errorf("%w: put display: %w", ErrR2Unavailable, err)
	}
	if err := u.r2Client.PutObject(ctx, r2.PutObjectInput{
		StorageKey:  thumbnailKey.String(),
		ContentType: "image/jpeg",
		Body:        thumbnail.Body,
	}); err != nil {
		return ProcessImageOutput{}, fmt.Errorf("%w: put thumbnail: %w", ErrR2Unavailable, err)
	}

	// domain side: MarkAvailable + AttachVariant×2。
	available, err := img.MarkAvailable(imagedomain.MarkAvailableParams{
		NormalizedFormat:   normalized_format.Jpg(),
		OriginalDimensions: originalDims,
		OriginalByteSize:   originalByteSize,
		MetadataStrippedAt: in.Now,
		Now:                in.Now,
	})
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("mark available: %w", err)
	}

	displayBs, err := byte_size.New(int64(len(display.Body)))
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("display byte_size: %w", err)
	}
	thumbnailBs, err := byte_size.New(int64(len(thumbnail.Body)))
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("thumbnail byte_size: %w", err)
	}
	displayDims, err := image_dimensions.New(display.Width, display.Height)
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("display dims: %w", err)
	}
	thumbnailDims, err := image_dimensions.New(thumbnail.Width, thumbnail.Height)
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("thumbnail dims: %w", err)
	}
	displayVariant, err := imagedomain.NewImageVariant(imagedomain.NewImageVariantParams{
		ImageID:    img.ID(),
		Kind:       variant_kind.Display(),
		StorageKey: displayKey,
		Dimensions: displayDims,
		ByteSize:   displayBs,
		MimeType:   mime_type.Jpeg(),
		CreatedAt:  in.Now,
	})
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("new display variant: %w", err)
	}
	thumbnailVariant, err := imagedomain.NewImageVariant(imagedomain.NewImageVariantParams{
		ImageID:    img.ID(),
		Kind:       variant_kind.Thumbnail(),
		StorageKey: thumbnailKey,
		Dimensions: thumbnailDims,
		ByteSize:   thumbnailBs,
		MimeType:   mime_type.Jpeg(),
		CreatedAt:  in.Now,
	})
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("new thumbnail variant: %w", err)
	}

	// DB TX: MarkAvailable + AttachVariant×2 + Outbox INSERT（短く完結、plan §10.8 / PR30）。
	if err := database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		txRepo := imagerdb.NewImageRepository(tx)
		if err := txRepo.MarkAvailable(ctx, available); err != nil {
			return fmt.Errorf("mark available db: %w", err)
		}
		if err := txRepo.AttachVariant(ctx, displayVariant); err != nil {
			return fmt.Errorf("attach display: %w", err)
		}
		if err := txRepo.AttachVariant(ctx, thumbnailVariant); err != nil {
			return fmt.Errorf("attach thumbnail: %w", err)
		}
		// PR30: image.became_available event を同 TX で Outbox に INSERT
		ev, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: outboxaggregate.Image(),
			AggregateID:   img.ID().UUID(),
			EventType:     outboxevent.ImageBecameAvailable(),
			Payload: outboxdomain.ImageBecameAvailablePayload{
				EventVersion:     outboxdomain.EventVersion,
				OccurredAt:       in.Now.UTC(),
				ImageID:          img.ID().String(),
				PhotobookID:      img.OwnerPhotobookID().String(),
				UsageKind:        img.UsageKind().String(),
				NormalizedFormat: "jpg", // plan §10 で display/thumbnail は JPEG 統一
				VariantCount:     2,
			},
			Now: in.Now.UTC(),
		})
		if err != nil {
			return fmt.Errorf("build image.became_available event: %w", err)
		}
		if err := outboxrdb.NewOutboxRepository(tx).Create(ctx, ev); err != nil {
			return fmt.Errorf("outbox create image.became_available: %w", err)
		}
		return nil
	}); err != nil {
		// DB 失敗時、display / thumbnail は R2 orphan（PR25 Reconcile cleanup）。
		return ProcessImageOutput{}, err
	}

	// 後始末: original を削除。失敗しても DB 状態は available のまま（orphan は PR25 cleanup）。
	if err := u.r2Client.DeleteObject(ctx, originalKey); err != nil {
		u.logger.WarnContext(ctx, "delete original failed (orphan, will be cleaned by Reconcile)",
			slog.String("image_id", img.ID().String()),
			slog.String("photobook_id", img.OwnerPhotobookID().String()),
			slog.String("error", err.Error()),
		)
	}

	u.logger.InfoContext(ctx, "image processed",
		slog.String("image_id", img.ID().String()),
		slog.String("photobook_id", img.OwnerPhotobookID().String()),
		slog.String("result", "available"),
		slog.String("source_format", img.SourceFormat().String()),
		slog.Int("variant_count", 2),
		slog.Int64("processing_duration_ms", time.Since(start).Milliseconds()),
	)

	return ProcessImageOutput{ImageID: img.ID(), Status: "available", VariantCount: 2}, nil
}

// failAndReturn は domain.MarkFailed → repo.MarkFailed + Outbox INSERT を同 TX で
// 実行し、log を出して結果を返す。
//
// MarkFailed 自体が失敗した場合は inner error を上に返す（DB 接続障害など）。
// PR30: failed 確定 TX に image.failed event を同 TX で INSERT。
func (u *ProcessImage) failAndReturn(
	ctx context.Context,
	img imagedomain.Image,
	reason failure_reason.FailureReason,
	now time.Time,
	startedAt time.Time,
	hint string,
) (ProcessImageOutput, error) {
	failed, err := img.MarkFailed(reason, now)
	if err != nil {
		return ProcessImageOutput{}, fmt.Errorf("domain mark failed: %w", err)
	}
	conflictNoOp := false
	if err := database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		txRepo := imagerdb.NewImageRepository(tx)
		if err := txRepo.MarkFailed(ctx, failed); err != nil {
			// 0 行影響（既に状態遷移済）は race condition 想定。
			// 同 TX 内で event を入れる前にこの分岐に入ったら、event は出さず終了する。
			if errors.Is(err, imagerdb.ErrConflict) {
				conflictNoOp = true
				return nil
			}
			return fmt.Errorf("repo mark failed: %w", err)
		}
		// PR30: image.failed event を同 TX で Outbox に INSERT
		ev, evErr := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: outboxaggregate.Image(),
			AggregateID:   img.ID().UUID(),
			EventType:     outboxevent.ImageFailed(),
			Payload: outboxdomain.ImageFailedPayload{
				EventVersion:  outboxdomain.EventVersion,
				OccurredAt:    now.UTC(),
				ImageID:       img.ID().String(),
				PhotobookID:   img.OwnerPhotobookID().String(),
				FailureReason: reason.String(),
			},
			Now: now.UTC(),
		})
		if evErr != nil {
			return fmt.Errorf("build image.failed event: %w", evErr)
		}
		if err := outboxrdb.NewOutboxRepository(tx).Create(ctx, ev); err != nil {
			return fmt.Errorf("outbox create image.failed: %w", err)
		}
		return nil
	}); err != nil {
		return ProcessImageOutput{}, err
	}
	if conflictNoOp {
		u.logger.WarnContext(ctx, "mark failed conflict (already transitioned)",
			slog.String("image_id", img.ID().String()),
			slog.String("hint", hint),
		)
	}
	u.logger.InfoContext(ctx, "image processing failed",
		slog.String("image_id", img.ID().String()),
		slog.String("photobook_id", img.OwnerPhotobookID().String()),
		slog.String("result", "failed"),
		slog.String("failure_reason", reason.String()),
		slog.String("source_format", img.SourceFormat().String()),
		slog.String("hint", hint),
		slog.Int64("processing_duration_ms", time.Since(startedAt).Milliseconds()),
	)
	return ProcessImageOutput{ImageID: img.ID(), Status: "failed"}, &ErrorWithReason{Reason: reason}
}

// readAllAndClose は io.ReadCloser を読み切って Close する。
func readAllAndClose(rc io.ReadCloser) ([]byte, error) {
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// sourceFormatToImaging は source_format VO を imaging.SourceFormat に変換する。
//
// HEIC は別経路（呼び出し側で短絡）で扱うため、ここに来た場合は "" を返す。
func sourceFormatToImaging(f image_format.ImageFormat) imaging.SourceFormat {
	switch {
	case f.Equal(image_format.Jpg()):
		return imaging.SourceJPEG
	case f.Equal(image_format.Png()):
		return imaging.SourcePNG
	case f.Equal(image_format.Webp()):
		return imaging.SourceWebP
	default:
		return ""
	}
}

// IsProcessFailedReason は err が ErrorWithReason であれば reason を返す。
func IsProcessFailedReason(err error) (failure_reason.FailureReason, bool) {
	var ewr *ErrorWithReason
	if errors.As(err, &ewr) {
		return ewr.Reason, true
	}
	return failure_reason.FailureReason{}, false
}

