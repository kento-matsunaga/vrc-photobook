// Package r2 は Cloudflare R2（S3 互換）への最小クライアント。
// M1 PoC 専用。本実装には流用しない（M2 でドメイン構造に沿って書き直す）。
//
// セキュリティ方針:
//   - presigned URL / Bucket 名 / AccountID / Access Key / Secret はログに出さない
//   - エラーメッセージはサーバ内ログにのみ残し、HTTP レスポンスには分類キーのみを返す
package r2

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appconfig "vrcpb/spike-backend/internal/config"
)

// ErrNotConfigured は R2 設定が未注入であることを示す。
// /sandbox/r2-* ハンドラはこれを 503 に変換する。
var ErrNotConfigured = errors.New("r2: not configured")

// Client は R2 への最小操作を提供するラッパー。
type Client struct {
	s3      *s3.Client
	presign *s3.PresignClient
	bucket  string
}

// NewClient は R2 用の S3 互換クライアントを生成する。
// 設定が未注入のとき (config.IsConfigured() == false) は ErrNotConfigured を返す。
func NewClient(ctx context.Context, cfg appconfig.R2Config) (*Client, error) {
	if !cfg.IsConfigured() {
		return nil, ErrNotConfigured
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID, cfg.SecretAccessKey, "",
			),
		),
	)
	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		// R2 は virtual-hosted 形式を推奨。本 PoC では path-style を使わない。
		o.UsePathStyle = false
	})

	return &Client{
		s3:      s3Client,
		presign: s3.NewPresignClient(s3Client),
		bucket:  cfg.BucketName,
	}, nil
}

// HeadBucket は R2 の HeadBucket を呼んで接続確認する。
func (c *Client) HeadBucket(ctx context.Context) error {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.bucket),
	})
	return err
}

// ListedObject は ListObjects の戻り値（Key と Size のみ）。
type ListedObject struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

// ListObjects は最大 maxKeys 件のオブジェクトを列挙する。
// 大量列挙を防ぐため maxKeys には妥当な上限（例: 100）を呼び出し側で適用すること。
func (c *Client) ListObjects(ctx context.Context, maxKeys int32) ([]ListedObject, error) {
	if maxKeys <= 0 {
		maxKeys = 5
	}
	out, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(c.bucket),
		MaxKeys: aws.Int32(maxKeys),
	})
	if err != nil {
		return nil, err
	}

	result := make([]ListedObject, 0, len(out.Contents))
	for _, obj := range out.Contents {
		result = append(result, ListedObject{
			Key:  aws.ToString(obj.Key),
			Size: aws.ToInt64(obj.Size),
		})
	}
	return result, nil
}

// PresignPut は PUT 用の署名付き URL を発行する。expires は最大 7日（S3 仕様）。
func (c *Client) PresignPut(
	ctx context.Context,
	key string,
	contentType string,
	contentLength int64,
	expires time.Duration,
) (string, error) {
	req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(contentLength),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// HeadResult は HeadObject の戻り値（必要最小）。
type HeadResult struct {
	ContentLength int64  `json:"content_length"`
	ContentType   string `json:"content_type"`
	ETag          string `json:"etag"`
}

// HeadObject はオブジェクトの存在 + ContentLength / ContentType / ETag を返す。
// 存在しない場合は SDK のエラーがそのまま返る（呼び出し側で NoSuchKey 判定）。
func (c *Client) HeadObject(ctx context.Context, key string) (*HeadResult, error) {
	out, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return &HeadResult{
		ContentLength: aws.ToInt64(out.ContentLength),
		ContentType:   aws.ToString(out.ContentType),
		ETag:          aws.ToString(out.ETag),
	}, nil
}
