// Package ogp_status は photobook_ogp_images.status の VO。
//
// 設計参照:
//   - docs/design/cross-cutting/ogp-generation.md §3.1 / §4
//   - docs/plan/m2-ogp-generation-plan.md §6
//
// CHECK 制約と一致する 5 値:
//   - pending   生成ジョブ投入後、未完了
//   - generated 生成成功（image_id 必須）
//   - failed    生成失敗（failed_at 必須、failure_reason は worker が sanitize）
//   - fallback  既定 OGP を永続的に使用することが決定（運営判断 / Reconcile）
//   - stale     Photobook が更新され、再生成待ち
package ogp_status

import (
	"errors"
	"fmt"
)

// ErrInvalidOgpStatus は未知の status 文字列を渡したときのエラー。
var ErrInvalidOgpStatus = errors.New("invalid ogp status")

type OgpStatus struct {
	v string
}

const (
	rawPending   = "pending"
	rawGenerated = "generated"
	rawFailed    = "failed"
	rawFallback  = "fallback"
	rawStale     = "stale"
)

func Pending() OgpStatus   { return OgpStatus{v: rawPending} }
func Generated() OgpStatus { return OgpStatus{v: rawGenerated} }
func Failed() OgpStatus    { return OgpStatus{v: rawFailed} }
func Fallback() OgpStatus  { return OgpStatus{v: rawFallback} }
func Stale() OgpStatus     { return OgpStatus{v: rawStale} }

// Parse は DB からの文字列を VO に復元する。
func Parse(s string) (OgpStatus, error) {
	switch s {
	case rawPending, rawGenerated, rawFailed, rawFallback, rawStale:
		return OgpStatus{v: s}, nil
	default:
		return OgpStatus{}, fmt.Errorf("%w: %q", ErrInvalidOgpStatus, s)
	}
}

func (s OgpStatus) String() string             { return s.v }
func (s OgpStatus) Equal(other OgpStatus) bool { return s.v == other.v }
func (s OgpStatus) IsZero() bool               { return s.v == "" }

func (s OgpStatus) IsPending() bool   { return s.v == rawPending }
func (s OgpStatus) IsGenerated() bool { return s.v == rawGenerated }
func (s OgpStatus) IsFailed() bool    { return s.v == rawFailed }
func (s OgpStatus) IsFallback() bool  { return s.v == rawFallback }
func (s OgpStatus) IsStale() bool     { return s.v == rawStale }
