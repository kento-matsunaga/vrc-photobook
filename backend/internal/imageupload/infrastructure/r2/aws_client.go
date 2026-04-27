// Package r2 (AWS SDK v2 implementation).
//
// 設計参照:
//   - docs/plan/m2-r2-presigned-url-plan.md §4
//   - harness/spike/backend M1 PoC: Content-Length signature 仕様
//
// PR21 Step A では本実装は Secret 未注入のため、起動時 client init 失敗時は nil 返却で
// 起動継続を許容する（main.go 側で IsR2Configured() を確認）。
package r2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

// AWSConfig は AWS SDK v2 ベースの R2 client 設定。
type AWSConfig struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	Endpoint        string // 例: https://<account>.r2.cloudflarestorage.com
}

// AWSClient は AWS SDK v2 S3 client を Cloudflare R2 endpoint に向けたもの。
type AWSClient struct {
	bucket    string
	s3        *s3.Client
	presigner *s3.PresignClient
}

// NewAWSClient は AWS SDK v2 ベースの R2 Client を組み立てる。
//
// region は R2 では auto/apac/wnam 等を取りうるが、s3 互換 API では何でも良い
// （Cloudflare 側で routing）。"auto" を使う。
func NewAWSClient(cfg AWSConfig) (*AWSClient, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" ||
		cfg.BucketName == "" || cfg.Endpoint == "" {
		return nil, fmt.Errorf("%w: missing R2 config", ErrUnavailable)
	}
	awsCfg := aws.Config{
		Region: "auto",
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, ""),
	}
	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})
	presigner := s3.NewPresignClient(s3Client)
	return &AWSClient{
		bucket:    cfg.BucketName,
		s3:        s3Client,
		presigner: presigner,
	}, nil
}

// PresignPutObject は presigned PUT URL を生成する。
//
// Content-Length は SignedHeaders に含めるため、実 PUT 時に同じサイズで送る必要がある
// （M1 PoC で検証済）。
func (c *AWSClient) PresignPutObject(ctx context.Context, in PresignPutInput) (PresignPutOutput, error) {
	expiresIn := in.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 15 * time.Minute
	}
	req, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(in.StorageKey),
		ContentType:   aws.String(in.ContentType),
		ContentLength: aws.Int64(in.ContentLength),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return PresignPutOutput{}, fmt.Errorf("%w: presign put: %w", ErrUnavailable, err)
	}
	headers := map[string]string{
		"Content-Type":   in.ContentType,
		"Content-Length": strconv.FormatInt(in.ContentLength, 10),
	}
	for k, v := range req.SignedHeader {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return PresignPutOutput{
		URL:             req.URL,
		RequiredHeaders: headers,
		ExpiresAt:       time.Now().Add(expiresIn),
	}, nil
}

// HeadObject は object 存在 + メタを取得する。
func (c *AWSClient) HeadObject(ctx context.Context, key string) (HeadObjectOutput, error) {
	out, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		var apiErr smithy.APIError
		if errors.As(err, &notFound) || (errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound") {
			return HeadObjectOutput{}, ErrObjectNotFound
		}
		return HeadObjectOutput{}, fmt.Errorf("%w: head object: %w", ErrUnavailable, err)
	}
	res := HeadObjectOutput{}
	if out.ContentLength != nil {
		res.ContentLength = *out.ContentLength
	}
	if out.ContentType != nil {
		res.ContentType = *out.ContentType
	}
	if out.ETag != nil {
		res.ETag = *out.ETag
	}
	return res, nil
}

// DeleteObject は object を削除する（PR23 image-processor / cleanup で使用予定）。
func (c *AWSClient) DeleteObject(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("%w: delete object: %w", ErrUnavailable, err)
	}
	return nil
}

// GetObject は object を取得する（PR23 image-processor が original を読み込む）。
//
// 戻り値の Body は呼び出し側で必ず Close すること。404 のとき ErrObjectNotFound。
func (c *AWSClient) GetObject(ctx context.Context, key string) (GetObjectOutput, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var noSuchKey *types.NoSuchKey
		var apiErr smithy.APIError
		if errors.As(err, &noSuchKey) || (errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey") {
			return GetObjectOutput{}, ErrObjectNotFound
		}
		return GetObjectOutput{}, fmt.Errorf("%w: get object: %w", ErrUnavailable, err)
	}
	res := GetObjectOutput{Body: out.Body}
	if out.ContentLength != nil {
		res.ContentLength = *out.ContentLength
	}
	if out.ContentType != nil {
		res.ContentType = *out.ContentType
	}
	if out.ETag != nil {
		res.ETag = *out.ETag
	}
	return res, nil
}

// ListObjects は prefix 一致の object key を列挙する（image-processor が原本 key を解決する用途）。
//
// images table に storage_key を保持しないため、`photobooks/{pid}/images/{iid}/original/`
// prefix で 1 件特定する。CommonPrefixes は使わず、Contents を Key だけ取り出す。
func (c *AWSClient) ListObjects(ctx context.Context, prefix string) (ListObjectsOutput, error) {
	out, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return ListObjectsOutput{}, fmt.Errorf("%w: list objects: %w", ErrUnavailable, err)
	}
	keys := make([]string, 0, len(out.Contents))
	for _, obj := range out.Contents {
		if obj.Key != nil {
			keys = append(keys, *obj.Key)
		}
	}
	return ListObjectsOutput{Keys: keys}, nil
}

// PutObject は object を直接 PUT する（PR23 image-processor が variant を書き込む）。
//
// PresignPutObject と異なり、Backend が Cloud Run service account の credential で直接 PUT
// するため、Content-Length signature 問題（M1 PoC）は発生しない。
func (c *AWSClient) PutObject(ctx context.Context, in PutObjectInput) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(in.StorageKey),
		ContentType: aws.String(in.ContentType),
		Body:        bytes.NewReader(in.Body),
	})
	if err != nil {
		return fmt.Errorf("%w: put object: %w", ErrUnavailable, err)
	}
	return nil
}
