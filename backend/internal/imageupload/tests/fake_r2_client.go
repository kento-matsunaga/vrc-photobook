// Package tests は imageupload のテストヘルパ。
//
// FakeR2Client は r2.Client の関数 field 差し替え式 test double。
// 実 Secret なしでも UseCase / handler テストが完結するようにする。
package tests

import (
	"bytes"
	"context"
	"io"
	"time"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
)

// FakeR2Client は r2.Client interface を実装する test double。
//
// 各メソッドは関数 field で差し替え可能。差し替えなしの場合の既定値は:
//   - PresignPutObject: 固定 URL "https://fake.r2.test/{key}" を返す
//   - HeadObject: ContentLength=in input or 1024、ContentType=image/jpeg
//   - DeleteObject: nil 成功
//   - GetObject: 空 body を返す（image-processor テストでは Fn 必須）
//   - PutObject: nil 成功
type FakeR2Client struct {
	PresignPutObjectFn func(ctx context.Context, in r2.PresignPutInput) (r2.PresignPutOutput, error)
	HeadObjectFn       func(ctx context.Context, key string) (r2.HeadObjectOutput, error)
	DeleteObjectFn     func(ctx context.Context, key string) error
	GetObjectFn        func(ctx context.Context, key string) (r2.GetObjectOutput, error)
	PutObjectFn        func(ctx context.Context, in r2.PutObjectInput) error
	ListObjectsFn      func(ctx context.Context, prefix string) (r2.ListObjectsOutput, error)
}

// PresignPutObject は PresignPutObjectFn を呼び出す。
func (f *FakeR2Client) PresignPutObject(ctx context.Context, in r2.PresignPutInput) (r2.PresignPutOutput, error) {
	if f.PresignPutObjectFn != nil {
		return f.PresignPutObjectFn(ctx, in)
	}
	return r2.PresignPutOutput{
		URL: "https://fake.r2.test/" + in.StorageKey,
		RequiredHeaders: map[string]string{
			"Content-Type":   in.ContentType,
			"Content-Length": strFromInt64(in.ContentLength),
		},
		ExpiresAt: time.Now().Add(in.ExpiresIn),
	}, nil
}

// HeadObject は HeadObjectFn を呼び出す。
func (f *FakeR2Client) HeadObject(ctx context.Context, key string) (r2.HeadObjectOutput, error) {
	if f.HeadObjectFn != nil {
		return f.HeadObjectFn(ctx, key)
	}
	return r2.HeadObjectOutput{
		ContentLength: 1024,
		ContentType:   "image/jpeg",
		ETag:          "fake-etag",
	}, nil
}

// DeleteObject は DeleteObjectFn を呼び出す。
func (f *FakeR2Client) DeleteObject(ctx context.Context, key string) error {
	if f.DeleteObjectFn != nil {
		return f.DeleteObjectFn(ctx, key)
	}
	return nil
}

// GetObject は GetObjectFn を呼び出す。差し替えなしの既定値は空 body。
func (f *FakeR2Client) GetObject(ctx context.Context, key string) (r2.GetObjectOutput, error) {
	if f.GetObjectFn != nil {
		return f.GetObjectFn(ctx, key)
	}
	return r2.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(nil)),
		ContentLength: 0,
		ContentType:   "application/octet-stream",
		ETag:          "fake-etag",
	}, nil
}

// PutObject は PutObjectFn を呼び出す。
func (f *FakeR2Client) PutObject(ctx context.Context, in r2.PutObjectInput) error {
	if f.PutObjectFn != nil {
		return f.PutObjectFn(ctx, in)
	}
	return nil
}

// ListObjects は ListObjectsFn を呼び出す。差し替えなしの既定値は空 list。
func (f *FakeR2Client) ListObjects(ctx context.Context, prefix string) (r2.ListObjectsOutput, error) {
	if f.ListObjectsFn != nil {
		return f.ListObjectsFn(ctx, prefix)
	}
	return r2.ListObjectsOutput{Keys: []string{}}, nil
}

func strFromInt64(v int64) string {
	// strconv 依存を避けるための小さなヘルパ。
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
