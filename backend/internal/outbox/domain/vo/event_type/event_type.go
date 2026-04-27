// Package event_type は Outbox event_type の VO。
//
// 設計参照:
//   - docs/plan/m2-outbox-plan.md §4
//   - docs/design/cross-cutting/outbox.md
//   - docs/adr/0006-email-provider-and-manage-url-delivery.md
//
// PR30 で実体投入する 3 種:
//   - photobook.published
//   - image.became_available
//   - image.failed
//
// CHECK 制約と一致。後続 PR で event を追加する都度、migration で CHECK を緩める
// 同時に本 VO の値域も追加する。
//
// セキュリティ:
//   - event_type は外部に出る（API response / log）が、token / Cookie / Secret は含まない
package event_type

import (
	"errors"
	"fmt"
)

// ErrInvalidEventType は未知の event_type 文字列を渡したときのエラー。
var ErrInvalidEventType = errors.New("invalid outbox event type")

// EventType は outbox_events.event_type に対応する VO。
type EventType struct {
	v string
}

const (
	rawPhotobookPublished   = "photobook.published"
	rawImageBecameAvailable = "image.became_available"
	rawImageFailed          = "image.failed"
)

func PhotobookPublished() EventType   { return EventType{v: rawPhotobookPublished} }
func ImageBecameAvailable() EventType { return EventType{v: rawImageBecameAvailable} }
func ImageFailed() EventType          { return EventType{v: rawImageFailed} }

// Parse は DB / 入力からの文字列を EventType に復元する。
//
// PR30 で許容するのは 3 種のみ。後続 PR で event を追加する際は本関数の switch を
// 拡張し、同時に migration で event_type CHECK を緩める。
func Parse(s string) (EventType, error) {
	switch s {
	case rawPhotobookPublished, rawImageBecameAvailable, rawImageFailed:
		return EventType{v: s}, nil
	default:
		return EventType{}, fmt.Errorf("%w: %q", ErrInvalidEventType, s)
	}
}

// String は文字列表現を返す。
func (e EventType) String() string { return e.v }

// Equal は値による等価判定。
func (e EventType) Equal(other EventType) bool { return e.v == other.v }

// IsZero は未初期化判定。
func (e EventType) IsZero() bool { return e.v == "" }
