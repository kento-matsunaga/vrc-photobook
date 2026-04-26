// Package image_id は Image 集約の DB 内部 ID（UUIDv7）を表す VO。
//
// 設計参照:
//   - docs/design/aggregates/image/データモデル設計.md §3
//   - docs/design/aggregates/image/ドメイン設計.md §4
//   - docs/adr/0001-tech-stack.md（UUIDv7 採用）
package image_id

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidImageID は nil UUID 等を渡したときのエラー。
var ErrInvalidImageID = errors.New("invalid image id")

// ImageID は Image 集約の DB 内部 ID（UUIDv7）。
type ImageID struct {
	v uuid.UUID
}

// New は新しい ImageID を UUIDv7 で生成する。
func New() (ImageID, error) {
	v, err := uuid.NewV7()
	if err != nil {
		return ImageID{}, err
	}
	return ImageID{v: v}, nil
}

// FromUUID は既存の uuid.UUID を ImageID として受け取る。
func FromUUID(v uuid.UUID) (ImageID, error) {
	if v == uuid.Nil {
		return ImageID{}, ErrInvalidImageID
	}
	return ImageID{v: v}, nil
}

// MustParse はテスト用ヘルパ。
func MustParse(s string) ImageID {
	return ImageID{v: uuid.MustParse(s)}
}

// UUID は内部の uuid.UUID を返す。永続化層との境界でのみ使用する。
func (i ImageID) UUID() uuid.UUID {
	return i.v
}

// Equal は値による等価判定。
func (i ImageID) Equal(other ImageID) bool {
	return i.v == other.v
}

// String は UUID 文字列を返す（DB 内部 ID なのでログ出力可）。
func (i ImageID) String() string {
	return i.v.String()
}
