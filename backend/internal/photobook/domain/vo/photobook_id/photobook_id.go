// Package photobook_id は Photobook 集約の DB 内部 ID（UUIDv7）を表す VO。
//
// 設計参照:
//   - docs/design/aggregates/photobook/データモデル設計.md §3
//   - docs/adr/0001-tech-stack.md（UUIDv7 採用）
//
// auth/session 側にも独立 VO `photobook_id` があり、両者は集約境界を保つために
// **敢えて分離**している。session UseCase / middleware の adapter で型変換する。
package photobook_id

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidPhotobookID は nil UUID 等を渡したときのエラー。
var ErrInvalidPhotobookID = errors.New("invalid photobook id")

// PhotobookID は Photobook 集約の DB 内部 ID（UUIDv7）。
type PhotobookID struct {
	v uuid.UUID
}

// New は新しい PhotobookID を UUIDv7 で生成する。
func New() (PhotobookID, error) {
	v, err := uuid.NewV7()
	if err != nil {
		return PhotobookID{}, err
	}
	return PhotobookID{v: v}, nil
}

// FromUUID は既存の uuid.UUID を PhotobookID として受け取る。
func FromUUID(v uuid.UUID) (PhotobookID, error) {
	if v == uuid.Nil {
		return PhotobookID{}, ErrInvalidPhotobookID
	}
	return PhotobookID{v: v}, nil
}

// MustParse はテスト用ヘルパ。
func MustParse(s string) PhotobookID {
	return PhotobookID{v: uuid.MustParse(s)}
}

// UUID は内部の uuid.UUID を返す。永続化層との境界でのみ使用する。
func (p PhotobookID) UUID() uuid.UUID {
	return p.v
}

// Equal は値による等価判定。
func (p PhotobookID) Equal(other PhotobookID) bool {
	return p.v == other.v
}

// String は UUID 文字列を返す（DB 内部 ID なのでログ出力可）。
func (p PhotobookID) String() string {
	return p.v.String()
}
