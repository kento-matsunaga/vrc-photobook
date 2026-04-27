// Package photobook_id は session 機構内で使う PhotobookID VO を提供する。
//
// 同名の VO `internal/photobook/domain/vo/photobook_id/` が photobook 集約側にも存在し、
// 本 VO は session 集約の独立性を保つために**敢えて分離**している。両 VO は同じ
// uuid.UUID を表現し、middleware / handler の境界で型変換される
// （`internal/http/router.go` の photobookIDFromURL 等）。
//
// session 機構が photobook_id を「ただの UUID」として扱うための最小限の型安全性のみを提供する。
package photobook_id

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidPhotobookID は nil UUID 等を渡したときのエラー。
var ErrInvalidPhotobookID = errors.New("invalid photobook id")

// PhotobookID は Photobook 集約の DB 内部 ID（UUIDv7、ADR-0001）。
type PhotobookID struct {
	v uuid.UUID
}

// FromUUID は既存の uuid.UUID を PhotobookID として受け取る。
func FromUUID(v uuid.UUID) (PhotobookID, error) {
	if v == uuid.Nil {
		return PhotobookID{}, ErrInvalidPhotobookID
	}
	return PhotobookID{v: v}, nil
}

// MustParse はテスト用ヘルパ。本番コードからは呼ばない。
func MustParse(s string) PhotobookID {
	v := uuid.MustParse(s)
	return PhotobookID{v: v}
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
