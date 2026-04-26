// Package r2 は Cloudflare R2 (S3 互換 API) client の interface を提供する。
//
// 設計参照:
//   - docs/plan/m2-r2-presigned-url-plan.md §4
//   - docs/adr/0005-image-upload-flow.md
//   - harness/spike/backend/README.md M1 R2 PoC（Content-Length 署名仕様）
//
// セキュリティ:
//   - presigned URL / R2 credentials はログに出さない
//   - storage_key は logs に出さない
package r2

import (
	"context"
	"errors"
	"time"
)

// エラー。
var (
	// ErrObjectNotFound は HeadObject で 404 が返ったとき。
	ErrObjectNotFound = errors.New("r2 object not found")
	// ErrUnavailable は R2 endpoint への接続失敗等。
	ErrUnavailable = errors.New("r2 client unavailable")
)

// PresignPutInput は PresignPutObject 呼び出しの引数。
//
// ContentLength は申告サイズ。aws-sdk-go-v2 の presign は Content-Length を
// SignedHeaders に含めるため、宣言値と実 PUT 時の body サイズが一致しないと R2 が
// 403 SignatureDoesNotMatch を返す（ADR-0005 §M1 PoC で実証済）。
type PresignPutInput struct {
	StorageKey    string
	ContentType   string
	ContentLength int64
	ExpiresIn     time.Duration
}

// PresignPutOutput は PresignPutObject の結果。
type PresignPutOutput struct {
	URL              string
	RequiredHeaders  map[string]string
	ExpiresAt        time.Time
}

// HeadObjectOutput は HeadObject の結果。
type HeadObjectOutput struct {
	ContentLength int64
	ContentType   string
	ETag          string
}

// Client は R2 への最小操作を抽象化する。
//
// PR21 Step A は本 interface を fake で置き換えてテストし、AWS SDK v2 実装は実
// Secret 注入後 (Step D) に動作確認する。
type Client interface {
	PresignPutObject(ctx context.Context, in PresignPutInput) (PresignPutOutput, error)
	HeadObject(ctx context.Context, key string) (HeadObjectOutput, error)
	DeleteObject(ctx context.Context, key string) error
}
