// Package usecase は imageupload UseCase を提供する。
//
// 設計参照:
//   - docs/plan/m2-r2-presigned-url-plan.md §6 / §7
//   - docs/adr/0005-image-upload-flow.md
//
// 公開する UseCase:
//   - IssueUploadIntent: Upload Verification consume + Image 行 INSERT + presigned URL 発行
//   - CompleteUpload: Image FindByID + R2 HeadObject + MarkProcessing / MarkFailed
//
// セキュリティ:
//   - presigned URL / R2 credentials / raw token / Cookie はログに出さない
//   - 失敗理由を外部に区別して出さない（敵対者の学習防止）
//   - max size 10MB / content_type whitelist / SVG 拒否は本層で確認
package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	imagedomain "vrcpb/backend/internal/image/domain"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/image_format"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/image/domain/vo/image_usage_kind"
	"vrcpb/backend/internal/image/domain/vo/storage_key"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	uploadrdb "vrcpb/backend/internal/uploadverification/infrastructure/repository/rdb"
)

// PresignTTL は presigned PUT URL の有効期限（ADR-0005 §presigned URL）。
const PresignTTL = 15 * time.Minute

// 共通エラー（外部に出す業務エラー）。
var (
	// ErrUploadVerificationFailed は Upload Verification consume 失敗。
	// upload-verification token 不正 / 期限切れ / 回数超過 / photobook 不一致を区別しない。
	ErrUploadVerificationFailed = errors.New("upload verification failed")

	// ErrInvalidUploadParameters は content_type / size / source_format の妥当性検証失敗。
	ErrInvalidUploadParameters = errors.New("invalid upload parameters")

	// ErrPresignFailed は R2 PresignPutObject 失敗（fail-closed で 500 系を返す）。
	ErrPresignFailed = errors.New("presign put failed")
)

// content_type の whitelist。SVG / HTML 等は拒否（PR21 計画書 §11）。
var allowedContentTypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
	"image/heic": {},
}

// MaxUploadByteSize は 1 画像の上限（v4 §3.10）。
const MaxUploadByteSize = 10 * 1024 * 1024

// === IssueUploadIntent ===

// VerificationConsumer は Upload Verification consume の依存抽象。
//
// 同 TX 内で消費されることを呼び出し側が保証する（atomic な失敗なら photobook UseCase 側で
// rollback、ただし PR20 の Repository は接続単位で動くため、本 UseCase では同 TX 内で
// 取得した tx-bound Repository を渡す）。
type VerificationConsumer interface {
	ConsumeOneByHashAndPhotobook(
		ctx context.Context,
		rawTokenEncoded string,
		photobookID photobook_id.PhotobookID,
		now time.Time,
	) error
}

// IssueUploadIntentInput は IssueUploadIntent の入力。
type IssueUploadIntentInput struct {
	PhotobookID            photobook_id.PhotobookID
	UploadVerificationToken string // raw token（base64url 43）。Authorization: Bearer から
	ContentType            string
	DeclaredByteSize       int64
	SourceFormat           string // jpg / png / webp / heic
	Now                    time.Time
}

// IssueUploadIntentOutput は presigned URL を含む結果。
//
// 値の取り扱い:
//   - UploadURL は logs / structured fields に出さない
//   - StorageKey は logs に出さない
//   - 必須 headers は Frontend が PUT 時に同名で送る前提
type IssueUploadIntentOutput struct {
	ImageID         image_id.ImageID
	UploadURL       string
	RequiredHeaders map[string]string
	StorageKey      storage_key.StorageKey
	ExpiresAt       time.Time
}

// IssueUploadIntent は Upload Verification consume + Image 行作成 + presigned URL 発行を
// 同一 TX 内で実行する UseCase。
type IssueUploadIntent struct {
	pool      *pgxpool.Pool
	r2Client  r2.Client
	consumer  VerificationConsumer // tx 非依存版（fallback / fake）。tx 内では新しく作る。
	presignTTL time.Duration
}

// NewIssueUploadIntent は UseCase を組み立てる。
//
// presignTTL は 0 で 15 分既定。
func NewIssueUploadIntent(pool *pgxpool.Pool, r2Client r2.Client, presignTTL time.Duration) *IssueUploadIntent {
	if presignTTL <= 0 {
		presignTTL = PresignTTL
	}
	return &IssueUploadIntent{
		pool:       pool,
		r2Client:   r2Client,
		presignTTL: presignTTL,
	}
}

// Execute は同一 TX 内で:
//  1. Upload Verification consume（PR20 Repository 経由）
//  2. content_type / size / source_format の軽量検証
//  3. images 行 INSERT (status='uploading')
//  4. storage_key 生成
//  5. R2 PresignPutObject
// を実行する。consume 成功後の image INSERT 失敗は TX rollback で巻き戻る。
func (u *IssueUploadIntent) Execute(ctx context.Context, in IssueUploadIntentInput) (IssueUploadIntentOutput, error) {
	if err := validateUploadParams(in.ContentType, in.DeclaredByteSize, in.SourceFormat); err != nil {
		return IssueUploadIntentOutput{}, err
	}
	srcFmt, err := image_format.Parse(in.SourceFormat)
	if err != nil {
		return IssueUploadIntentOutput{}, fmt.Errorf("%w: source_format", ErrInvalidUploadParameters)
	}
	rawToken := in.UploadVerificationToken
	if rawToken == "" {
		return IssueUploadIntentOutput{}, ErrUploadVerificationFailed
	}

	var out IssueUploadIntentOutput
	err = database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		// Upload Verification を同 TX 内で consume
		uploadRepo := uploadrdb.NewUploadVerificationSessionRepository(tx)
		if err := consumeWithToken(ctx, uploadRepo, rawToken, in.PhotobookID, in.Now); err != nil {
			return err
		}

		// 新規 image_id + Image domain entity
		newID, err := image_id.New()
		if err != nil {
			return err
		}
		img, err := imagedomain.NewUploadingImage(imagedomain.NewUploadingImageParams{
			ID:               newID,
			OwnerPhotobookID: in.PhotobookID,
			UsageKind:        image_usage_kind.Photo(),
			SourceFormat:     srcFmt,
			Now:              in.Now,
		})
		if err != nil {
			return err
		}

		imageRepo := imagerdb.NewImageRepository(tx)
		if err := imageRepo.CreateUploading(ctx, img); err != nil {
			return err
		}

		// storage_key 生成: original variant prefix を使う（ADR-0005 §storage_key、
		// MVP では original variant 行は作らないが、PUT 先の object key は original/ 配下）
		key, err := storage_key.GenerateForOriginal(in.PhotobookID, newID, srcFmt)
		if err != nil {
			return err
		}
		_ = variant_kind.Original() // 用途明示（compiler の dead-code 除去回避ではなく、設計補足）

		// R2 PresignPutObject
		presigned, err := u.r2Client.PresignPutObject(ctx, r2.PresignPutInput{
			StorageKey:    key.String(),
			ContentType:   in.ContentType,
			ContentLength: in.DeclaredByteSize,
			ExpiresIn:     u.presignTTL,
		})
		if err != nil {
			return fmt.Errorf("%w: %w", ErrPresignFailed, err)
		}
		out = IssueUploadIntentOutput{
			ImageID:         newID,
			UploadURL:       presigned.URL,
			RequiredHeaders: presigned.RequiredHeaders,
			StorageKey:      key,
			ExpiresAt:       presigned.ExpiresAt,
		}
		return nil
	})
	if err != nil {
		return IssueUploadIntentOutput{}, err
	}
	return out, nil
}

// consumeWithToken は raw token (base64url 43) を hash 化して
// PR20 Repository.ConsumeOne を呼ぶ。失敗は ErrUploadVerificationFailed に集約。
func consumeWithToken(
	ctx context.Context,
	repo *uploadrdb.UploadVerificationSessionRepository,
	rawToken string,
	pid photobook_id.PhotobookID,
	now time.Time,
) error {
	tok, err := parseUploadVerificationToken(rawToken)
	if err != nil {
		return ErrUploadVerificationFailed
	}
	if _, err := repo.ConsumeOne(ctx, tok.Hash(), pid, now); err != nil {
		if errors.Is(err, uploadrdb.ErrUploadVerificationFailed) {
			return ErrUploadVerificationFailed
		}
		return err
	}
	return nil
}

func validateUploadParams(contentType string, declaredSize int64, sourceFormat string) error {
	if _, ok := allowedContentTypes[contentType]; !ok {
		return fmt.Errorf("%w: content_type", ErrInvalidUploadParameters)
	}
	if declaredSize < 1 {
		return fmt.Errorf("%w: declared_byte_size < 1", ErrInvalidUploadParameters)
	}
	if declaredSize > MaxUploadByteSize {
		return fmt.Errorf("%w: declared_byte_size > 10MB", ErrInvalidUploadParameters)
	}
	if sourceFormat == "" {
		return fmt.Errorf("%w: source_format empty", ErrInvalidUploadParameters)
	}
	// byte_size VO 側でも上限検査（domain unit と一致確認）
	if _, err := byte_size.New(declaredSize); err != nil {
		return fmt.Errorf("%w: byte_size invalid: %w", ErrInvalidUploadParameters, err)
	}
	return nil
}
