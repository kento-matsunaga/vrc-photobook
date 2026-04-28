// Package event_type は Outbox event_type の VO。
//
// 設計参照:
//   - docs/plan/m2-outbox-plan.md §4
//   - docs/design/cross-cutting/outbox.md
//   - docs/adr/0006-email-provider-and-manage-url-delivery.md
//
// 受け入れる event_type:
//   - photobook.published     (PR30 起点 / PR33d で OGP 生成 handler 接続)
//   - photobook.hidden        (PR34b / handler は no-op + log)
//   - photobook.unhidden      (PR34b / 同上)
//   - image.became_available  (PR30 / no-op)
//   - image.failed            (PR30 / no-op)
//
// CHECK 制約と一致（migration 00012 + 00015）。後続 PR で event を追加する都度、
// migration で CHECK を緩める同時に本 VO の値域も追加する。
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
	rawPhotobookHidden      = "photobook.hidden"
	rawPhotobookUnhidden    = "photobook.unhidden"
	rawImageBecameAvailable = "image.became_available"
	rawImageFailed          = "image.failed"
)

func PhotobookPublished() EventType   { return EventType{v: rawPhotobookPublished} }
func PhotobookHidden() EventType      { return EventType{v: rawPhotobookHidden} }
func PhotobookUnhidden() EventType    { return EventType{v: rawPhotobookUnhidden} }
func ImageBecameAvailable() EventType { return EventType{v: rawImageBecameAvailable} }
func ImageFailed() EventType          { return EventType{v: rawImageFailed} }

// Parse は DB / 入力からの文字列を EventType に復元する。
//
// 値域は 5 種（migration 00012 + 00015 の CHECK と一致）。後続 PR で event を追加する
// 際は本関数の switch / 上記 const を拡張し、同時に migration で event_type CHECK を緩める。
func Parse(s string) (EventType, error) {
	switch s {
	case rawPhotobookPublished, rawPhotobookHidden, rawPhotobookUnhidden,
		rawImageBecameAvailable, rawImageFailed:
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
