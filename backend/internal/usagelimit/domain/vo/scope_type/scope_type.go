// Package scope_type は UsageLimit の scope_type 値オブジェクト。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §5 / §6.2
//   - migrations/00018_create_usage_counters.sql の CHECK 制約と同期
//
// 4 種:
//   - source_ip_hash    : 同一作成元 IP の hash（salt+sha256、PR35b と salt 共有）
//   - draft_session_id  : draft session 単位
//   - manage_session_id : manage session 単位（MVP 未使用、将来用）
//   - photobook_id      : photobook 単位
package scope_type

import (
	"errors"
	"fmt"
)

// ErrInvalidScopeType は許容外の scope_type 文字列を渡されたとき。
var ErrInvalidScopeType = errors.New("invalid usagelimit scope_type")

// ScopeType は usage_counters.scope_type 値オブジェクト。
type ScopeType struct{ v string }

const (
	rawSourceIPHash    = "source_ip_hash"
	rawDraftSessionID  = "draft_session_id"
	rawManageSessionID = "manage_session_id"
	rawPhotobookID     = "photobook_id"
)

// SourceIPHash は同一作成元 IP の hash scope。
func SourceIPHash() ScopeType { return ScopeType{v: rawSourceIPHash} }

// DraftSessionID は draft session 単位の scope。
func DraftSessionID() ScopeType { return ScopeType{v: rawDraftSessionID} }

// ManageSessionID は manage session 単位の scope（MVP 未使用、将来用）。
func ManageSessionID() ScopeType { return ScopeType{v: rawManageSessionID} }

// PhotobookID は photobook 単位の scope。
func PhotobookID() ScopeType { return ScopeType{v: rawPhotobookID} }

// Parse は DB / 入力からの文字列を ScopeType に復元する。
func Parse(s string) (ScopeType, error) {
	switch s {
	case rawSourceIPHash, rawDraftSessionID, rawManageSessionID, rawPhotobookID:
		return ScopeType{v: s}, nil
	default:
		return ScopeType{}, fmt.Errorf("%w: %q", ErrInvalidScopeType, s)
	}
}

// String は DB / 出力用の文字列表現。
func (s ScopeType) String() string { return s.v }

// Equal は 2 つの ScopeType を比較する。
func (s ScopeType) Equal(o ScopeType) bool { return s.v == o.v }

// IsZero は VO が未初期化（ゼロ値）かを返す。
func (s ScopeType) IsZero() bool { return s.v == "" }
