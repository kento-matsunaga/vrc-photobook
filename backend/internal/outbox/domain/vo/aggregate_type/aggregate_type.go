// Package aggregate_type は Outbox aggregate_type の VO。
//
// 設計参照:
//   - docs/plan/m2-outbox-plan.md §3.3
//   - docs/design/cross-cutting/outbox.md
//
// CHECK 制約と一致: photobook / image / report / moderation / manage_url_delivery（5 種）。
// PR30 で実体投入するのは photobook / image のみ。残り 3 種は将来 PR の拡張余地。
package aggregate_type

import (
	"errors"
	"fmt"
)

// ErrInvalidAggregateType は未知の値を渡したときのエラー。
var ErrInvalidAggregateType = errors.New("invalid outbox aggregate type")

// AggregateType は outbox_events.aggregate_type に対応する VO。
type AggregateType struct {
	v string
}

const (
	rawPhotobook          = "photobook"
	rawImage              = "image"
	rawReport             = "report"
	rawModeration         = "moderation"
	rawManageUrlDelivery  = "manage_url_delivery"
)

func Photobook() AggregateType         { return AggregateType{v: rawPhotobook} }
func Image() AggregateType             { return AggregateType{v: rawImage} }
func Report() AggregateType            { return AggregateType{v: rawReport} }
func Moderation() AggregateType        { return AggregateType{v: rawModeration} }
func ManageUrlDelivery() AggregateType { return AggregateType{v: rawManageUrlDelivery} }

// Parse は DB / 入力からの文字列を AggregateType に復元する。
func Parse(s string) (AggregateType, error) {
	switch s {
	case rawPhotobook, rawImage, rawReport, rawModeration, rawManageUrlDelivery:
		return AggregateType{v: s}, nil
	default:
		return AggregateType{}, fmt.Errorf("%w: %q", ErrInvalidAggregateType, s)
	}
}

func (a AggregateType) String() string                 { return a.v }
func (a AggregateType) Equal(other AggregateType) bool { return a.v == other.v }
func (a AggregateType) IsZero() bool                   { return a.v == "" }
