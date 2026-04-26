// Package storage_key は ImageVariant.storage_key の VO。
//
// 命名規則（ADR-0005 §storage_key、image データモデル §4 / 付録C P0-12）:
//
//	photobooks/{photobook_id}/images/{image_id}/{variant}/{random}.{ext}
//	photobooks/{photobook_id}/ogp/{ogp_id}/{random}.png
//
// 制約:
//   - bucket 名は含めない（DB に保存するのは bucket 内パスのみ）
//   - {random} は 12 byte 暗号論的乱数を base64url（padding なし）で 16 文字
//   - {photobook_id} / {image_id} は UUID 文字列
//   - original variant のみ {ext} に元拡張子（jpg / png / webp / heic）、
//     display / thumbnail / ogp は固定（webp / png）
//
// セキュリティ:
//   - storage_key はログ出力させない（presigned URL の前提情報になりうるため、
//     security-guard.md §logs に従う）
package storage_key

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"vrcpb/backend/internal/image/domain/vo/image_format"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// StorageKey は R2 / S3 内の object path（bucket 名含まず）。
type StorageKey struct {
	v string
}

// エラー。
var (
	ErrEmptyStorageKey       = errors.New("storage key must not be empty")
	ErrStorageKeyTooLong     = errors.New("storage key too long")
	ErrInvalidStorageKey     = errors.New("invalid storage key")
	ErrUnsupportedExtension  = errors.New("unsupported original extension")
)

// 上限はランタイムログ / DB ストレージ保護用の安全側数値。MVP では十分。
const maxLen = 512

// randomBytesLen は {random} 部の生バイト長。base64url で 16 文字になる。
const randomBytesLen = 12

// Parse は外部入力（DB 復元など）の storage_key を VO に変換する。
//
// 形式の最低限のチェック（先頭 prefix `photobooks/` / 長さ）を行うが、
// 構造解析は §generation 側に閉じ、ここでは保存可能性のみ検証する。
func Parse(s string) (StorageKey, error) {
	if s == "" {
		return StorageKey{}, ErrEmptyStorageKey
	}
	if len(s) > maxLen {
		return StorageKey{}, fmt.Errorf("%w: len=%d", ErrStorageKeyTooLong, len(s))
	}
	if !strings.HasPrefix(s, "photobooks/") {
		return StorageKey{}, fmt.Errorf("%w: must start with photobooks/", ErrInvalidStorageKey)
	}
	return StorageKey{v: s}, nil
}

// String は path 文字列を返す。
func (k StorageKey) String() string { return k.v }

// Equal は値による等価判定。
func (k StorageKey) Equal(other StorageKey) bool { return k.v == other.v }

// IsZero は未初期化判定。
func (k StorageKey) IsZero() bool { return k.v == "" }

// GenerateForVariant は通常 variant（display / thumbnail）の storage_key を生成する。
//
//	photobooks/{photobook_id}/images/{image_id}/{kind}/{random}.{ext}
//
// kind=display / thumbnail は webp 固定。
// kind=original は元拡張子が必要なため、本関数では受け付けない（GenerateForOriginal を使う）。
// kind=ogp は png 固定（GenerateForOgp を使う）。
func GenerateForVariant(
	pid photobook_id.PhotobookID,
	iid image_id.ImageID,
	kind variant_kind.VariantKind,
) (StorageKey, error) {
	switch {
	case kind.IsDisplay(), kind.IsThumbnail():
		return generate(fmt.Sprintf(
			"photobooks/%s/images/%s/%s",
			pid.String(), iid.String(), kind.String(),
		), "webp")
	case kind.IsOriginal():
		return StorageKey{}, fmt.Errorf("%w: use GenerateForOriginal", ErrInvalidStorageKey)
	case kind.IsOgp():
		return StorageKey{}, fmt.Errorf("%w: use GenerateForOgp", ErrInvalidStorageKey)
	default:
		return StorageKey{}, ErrInvalidStorageKey
	}
}

// GenerateForOriginal は original variant の storage_key を生成する。
//
//	photobooks/{photobook_id}/images/{image_id}/original/{random}.{source_ext}
//
// MVP では original variant は保持しない（v4 U9）が、将来の保持判断のために
// 関数だけ提供しておく。
func GenerateForOriginal(
	pid photobook_id.PhotobookID,
	iid image_id.ImageID,
	src image_format.ImageFormat,
) (StorageKey, error) {
	ext, err := extOf(src)
	if err != nil {
		return StorageKey{}, err
	}
	return generate(fmt.Sprintf(
		"photobooks/%s/images/%s/original",
		pid.String(), iid.String(),
	), ext)
}

// GenerateForOgp は OGP variant の storage_key を生成する（png 固定）。
//
//	photobooks/{photobook_id}/ogp/{ogp_id}/{random}.png
//
// PR18 では ogp_id は別集約（OGP 集約）の ID。便宜上 image_id を渡す形で受ける。
func GenerateForOgp(
	pid photobook_id.PhotobookID,
	ogpID image_id.ImageID,
) (StorageKey, error) {
	return generate(fmt.Sprintf(
		"photobooks/%s/ogp/%s",
		pid.String(), ogpID.String(),
	), "png")
}

func generate(prefix, ext string) (StorageKey, error) {
	r, err := randomToken()
	if err != nil {
		return StorageKey{}, err
	}
	return StorageKey{v: fmt.Sprintf("%s/%s.%s", prefix, r, ext)}, nil
}

func randomToken() (string, error) {
	var b [randomBytesLen]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func extOf(src image_format.ImageFormat) (string, error) {
	switch src {
	case image_format.Jpg():
		return "jpg", nil
	case image_format.Png():
		return "png", nil
	case image_format.Webp():
		return "webp", nil
	case image_format.Heic():
		return "heic", nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedExtension, src.String())
	}
}
