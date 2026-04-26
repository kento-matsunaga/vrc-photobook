package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	imagedomain "vrcpb/backend/internal/image/domain"
	"vrcpb/backend/internal/image/domain/vo/failure_reason"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// CompleteUpload エラー。
var (
	// ErrImageNotFound は images.id 一致なし / 別 photobook 所有 / 既に削除済。
	ErrImageNotFound = errors.New("image not found")

	// ErrImageNotUploading は status != uploading（idempotent return / 異常系両方）。
	// 呼び出し側は idempotent return か業務エラーかを判定する。
	ErrImageNotUploading = errors.New("image is not in uploading state")

	// ErrStorageKeyMismatch は Frontend echo back の storage_key prefix 不一致。
	ErrStorageKeyMismatch = errors.New("storage_key prefix mismatch")

	// ErrUploadValidationFailed は HeadObject で検出した検証失敗（外部には failure_reason
	// だけ返す）。
	ErrUploadValidationFailed = errors.New("upload validation failed")

	// ErrR2Unavailable は R2 接続失敗（fail-closed）。
	ErrR2Unavailable = errors.New("r2 unavailable")
)

// CompleteUploadInput は CompleteUpload の入力。
//
// StorageKey は upload-intent response で Frontend に渡したものを echo back させる。
// 受け取った StorageKey が `photobooks/{photobook_id}/images/{image_id}/original/`
// で始まるかを Backend 側で検証する（spoofing 防止）。
type CompleteUploadInput struct {
	PhotobookID photobook_id.PhotobookID
	ImageID     image_id.ImageID
	StorageKey  string
	Now         time.Time
}

// CompleteUploadOutput は遷移結果。
type CompleteUploadOutput struct {
	ImageID image_id.ImageID
	Status  string // "processing" もしくは既存 status の文字列（idempotent return）
}

// CompleteUpload は upload 完了処理を行う UseCase。
type CompleteUpload struct {
	pool     *pgxpool.Pool
	r2Client r2.Client
}

// NewCompleteUpload は UseCase を組み立てる。
func NewCompleteUpload(pool *pgxpool.Pool, r2Client r2.Client) *CompleteUpload {
	return &CompleteUpload{pool: pool, r2Client: r2Client}
}

// Execute は次の順で処理する:
//  1. images.FindByID
//  2. owner_photobook_id == in.PhotobookID 確認（不一致は ErrImageNotFound、情報を漏らさない）
//  3. status == uploading 確認（既に processing 等は idempotent return）
//  4. storage_key prefix 検証
//  5. R2 HeadObject（不存在なら MarkFailed(object_not_found)）
//  6. ContentLength <= 10MB / ContentType 妥当性
//  7. images.MarkProcessing
//
// 失敗時は適切な ImageStatus で MarkFailed することで、後続 image-processor / Reconcile が
// 整合した状態で扱える。
func (u *CompleteUpload) Execute(ctx context.Context, in CompleteUploadInput) (CompleteUploadOutput, error) {
	repo := imagerdb.NewImageRepository(u.pool)

	img, err := repo.FindByID(ctx, in.ImageID)
	if err != nil {
		if errors.Is(err, imagerdb.ErrNotFound) {
			return CompleteUploadOutput{}, ErrImageNotFound
		}
		return CompleteUploadOutput{}, err
	}
	if !img.OwnerPhotobookID().Equal(in.PhotobookID) {
		// 別 photobook の image_id を渡されたら情報漏洩しないため NotFound 扱い
		return CompleteUploadOutput{}, ErrImageNotFound
	}

	// status が uploading でなければ:
	//   - processing / available なら idempotent return（呼び出し側が再 POST しても同じ）
	//   - failed / deleted / purged は ErrImageNotUploading
	if !img.IsUploading() {
		switch {
		case img.IsProcessing(), img.IsAvailable():
			return CompleteUploadOutput{ImageID: img.ID(), Status: img.Status().String()}, nil
		default:
			return CompleteUploadOutput{}, ErrImageNotUploading
		}
	}

	// storage_key prefix 検証
	wantPrefix := fmt.Sprintf("photobooks/%s/images/%s/original/",
		in.PhotobookID.String(), in.ImageID.String())
	if !strings.HasPrefix(in.StorageKey, wantPrefix) {
		return CompleteUploadOutput{}, ErrStorageKeyMismatch
	}

	// R2 HeadObject
	head, err := u.r2Client.HeadObject(ctx, in.StorageKey)
	if err != nil {
		if errors.Is(err, r2.ErrObjectNotFound) {
			// MarkFailed(object_not_found)
			if mf, mfErr := failedImage(img, failure_reason.ObjectNotFound(), in.Now); mfErr == nil {
				_ = repo.MarkFailed(ctx, mf)
			}
			return CompleteUploadOutput{}, fmt.Errorf("%w: object not found", ErrUploadValidationFailed)
		}
		return CompleteUploadOutput{}, fmt.Errorf("%w: %w", ErrR2Unavailable, err)
	}

	// size 確認（HeadObject の ContentLength は実 PUT サイズを返す）
	if head.ContentLength > MaxUploadByteSize {
		if mf, mfErr := failedImage(img, failure_reason.FileTooLarge(), in.Now); mfErr == nil {
			_ = repo.MarkFailed(ctx, mf)
		}
		return CompleteUploadOutput{}, fmt.Errorf("%w: file too large", ErrUploadValidationFailed)
	}
	if head.ContentLength < 1 {
		if mf, mfErr := failedImage(img, failure_reason.SizeMismatch(), in.Now); mfErr == nil {
			_ = repo.MarkFailed(ctx, mf)
		}
		return CompleteUploadOutput{}, fmt.Errorf("%w: size mismatch", ErrUploadValidationFailed)
	}

	// content-type 確認（whitelist）
	if _, ok := allowedContentTypes[head.ContentType]; !ok {
		if mf, mfErr := failedImage(img, failure_reason.UnsupportedFormat(), in.Now); mfErr == nil {
			_ = repo.MarkFailed(ctx, mf)
		}
		return CompleteUploadOutput{}, fmt.Errorf("%w: unsupported format", ErrUploadValidationFailed)
	}

	// images.MarkProcessing
	processed, err := img.MarkProcessing(in.Now)
	if err != nil {
		return CompleteUploadOutput{}, err
	}
	if err := repo.MarkProcessing(ctx, processed); err != nil {
		// 並行で別 request が complete を呼んで先に processing にした場合 ErrConflict。
		// その場合 idempotent return として扱う。
		if errors.Is(err, imagerdb.ErrConflict) {
			return CompleteUploadOutput{ImageID: img.ID(), Status: "processing"}, nil
		}
		return CompleteUploadOutput{}, err
	}
	return CompleteUploadOutput{ImageID: img.ID(), Status: "processing"}, nil
}

// failedImage は uploading → failed への遷移結果を返す（domain メソッド経由）。
func failedImage(
	img imagedomain.Image,
	reason failure_reason.FailureReason,
	now time.Time,
) (imagedomain.Image, error) {
	return img.MarkFailed(reason, now)
}
