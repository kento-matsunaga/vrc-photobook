// Package page_id は photobook_pages.id の VO（UUIDv7）。
//
// 設計参照:
//   - docs/design/aggregates/photobook/データモデル設計.md §4
//   - docs/adr/0001-tech-stack.md（UUIDv7 採用）
package page_id

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidPageID は nil UUID 等を渡したときのエラー。
var ErrInvalidPageID = errors.New("invalid page id")

// PageID は Photobook.Page の DB 内部 ID。
type PageID struct {
	v uuid.UUID
}

// New は新しい PageID を UUIDv7 で生成する。
func New() (PageID, error) {
	v, err := uuid.NewV7()
	if err != nil {
		return PageID{}, err
	}
	return PageID{v: v}, nil
}

// FromUUID は既存の uuid.UUID を PageID として受け取る。
func FromUUID(v uuid.UUID) (PageID, error) {
	if v == uuid.Nil {
		return PageID{}, ErrInvalidPageID
	}
	return PageID{v: v}, nil
}

// MustParse はテスト用ヘルパ。
func MustParse(s string) PageID {
	return PageID{v: uuid.MustParse(s)}
}

func (p PageID) UUID() uuid.UUID         { return p.v }
func (p PageID) Equal(o PageID) bool     { return p.v == o.v }
func (p PageID) String() string          { return p.v.String() }
