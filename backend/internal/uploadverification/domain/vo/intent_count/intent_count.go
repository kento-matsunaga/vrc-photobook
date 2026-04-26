// Package intent_count は upload_verification_sessions.allowed_intent_count /
// used_intent_count の VO。
//
// 設計参照:
//   - docs/adr/0005-image-upload-flow.md §upload_verification_session
//   - docs/plan/m2-upload-verification-plan.md §3
//
// 不変条件:
//   - 0 以上
//   - allowed >= used が呼び出し側で保証される（domain entity 側で組み合わせ検証）
package intent_count

import (
	"errors"
	"fmt"
)

var (
	ErrNegativeCount = errors.New("intent count must be non-negative")
	ErrAllowedZero   = errors.New("allowed intent count must be > 0")
)

// IntentCount は intent 数を表す VO。
type IntentCount struct {
	v int
}

// New は int を IntentCount に変換する（0 以上）。
func New(v int) (IntentCount, error) {
	if v < 0 {
		return IntentCount{}, fmt.Errorf("%w: %d", ErrNegativeCount, v)
	}
	return IntentCount{v: v}, nil
}

// MustNew はテスト用ヘルパ。
func MustNew(v int) IntentCount {
	c, err := New(v)
	if err != nil {
		panic(err)
	}
	return c
}

// Zero は 0 を返す。
func Zero() IntentCount { return IntentCount{v: 0} }

// Default は MVP 既定値 20 を返す（ADR-0005）。
func Default() IntentCount { return IntentCount{v: 20} }

// Int は int 表現を返す。
func (c IntentCount) Int() int                    { return c.v }
func (c IntentCount) Equal(o IntentCount) bool    { return c.v == o.v }

// Increment は +1 した新しい値を返す（不変）。
func (c IntentCount) Increment() IntentCount {
	return IntentCount{v: c.v + 1}
}
