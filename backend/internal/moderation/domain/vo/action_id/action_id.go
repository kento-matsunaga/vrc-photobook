// Package action_id は ModerationAction の DB 内部 ID（UUIDv7）を表す VO。
//
// 設計参照:
//   - docs/design/aggregates/moderation/ドメイン設計.md §4.5
//   - docs/adr/0001-tech-stack.md（UUIDv7 採用）
package action_id

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidActionID は nil UUID 等を渡したときのエラー。
var ErrInvalidActionID = errors.New("invalid moderation action id")

// ActionID は ModerationAction の DB 内部 ID（UUIDv7）。
type ActionID struct {
	v uuid.UUID
}

// New は新しい ActionID を UUIDv7 で生成する。
func New() (ActionID, error) {
	v, err := uuid.NewV7()
	if err != nil {
		return ActionID{}, err
	}
	return ActionID{v: v}, nil
}

// FromUUID は既存の uuid.UUID を ActionID として受け取る。
func FromUUID(v uuid.UUID) (ActionID, error) {
	if v == uuid.Nil {
		return ActionID{}, ErrInvalidActionID
	}
	return ActionID{v: v}, nil
}

// MustParse はテスト用ヘルパ。
func MustParse(s string) ActionID {
	return ActionID{v: uuid.MustParse(s)}
}

// UUID は内部の uuid.UUID を返す。永続化層との境界でのみ使用する。
func (a ActionID) UUID() uuid.UUID { return a.v }

// Equal は値による等価判定。
func (a ActionID) Equal(other ActionID) bool { return a.v == other.v }

// String は UUID 文字列を返す（DB 内部 ID なのでログ出力可）。
func (a ActionID) String() string { return a.v.String() }

// IsZero は未初期化判定。
func (a ActionID) IsZero() bool { return a.v == uuid.Nil }
