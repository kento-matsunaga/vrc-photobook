// Package scope_hash は UsageLimit の scope_hash 値オブジェクト。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §6.4 / §13
//   - .agents/rules/security-guard.md（hash 値の取り扱い）
//
// scope_hash は scope_type ごとに以下の方法で算出された不変識別子:
//   - source_ip_hash    : salt+sha256（既存 `internal/report/usecase.HashSourceIP` と
//                         REPORT_IP_HASH_SALT_V1 を共有）→ 32 byte → hex 64 文字
//   - draft_session_id  : sessions.id（UUID v7 の hex 化、salt なし）
//   - manage_session_id : 同上
//   - photobook_id      : UUID v7 の hex 化、salt なし
//
// 本 VO は **保存・比較に使う識別子**であり、復元できないことを期待しない（hash でない
// scope_type の場合は単なる id 文字列の保持に過ぎない）。
//
// セキュリティ:
//   - 完全値を logs / chat / work-log に出さない運用（cmd/ops は Redacted() / Prefix() を使う）
//   - 空文字 / 異常に長い文字列は拒否（誤代入防止）
package scope_hash

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidScopeHash は scope_hash の形式違反。
var ErrInvalidScopeHash = errors.New("invalid usagelimit scope_hash")

const (
	// minLen / maxLen は誤代入防止のための緩い境界。
	// - source_ip_hash 64 hex / UUID 32 hex（dash 抜き）/ UUID 36 char（dash 含む）
	//   いずれも収まる範囲を選ぶ。
	minLen = 8
	maxLen = 128

	// redactPrefixLen は cmd/ops 等の表示で使う先頭文字数。
	redactPrefixLen = 8
)

// ScopeHash は usage_counters.scope_hash 値オブジェクト。
type ScopeHash struct{ v string }

// Parse は文字列を ScopeHash に変換する。境界チェック + trim。
func Parse(s string) (ScopeHash, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return ScopeHash{}, fmt.Errorf("%w: empty", ErrInvalidScopeHash)
	}
	if len(t) < minLen {
		return ScopeHash{}, fmt.Errorf("%w: too short (len=%d)", ErrInvalidScopeHash, len(t))
	}
	if len(t) > maxLen {
		return ScopeHash{}, fmt.Errorf("%w: too long (len=%d)", ErrInvalidScopeHash, len(t))
	}
	return ScopeHash{v: t}, nil
}

// String は DB / Repository 引数として使う完全値。
//
// **chat / log / work-log への出力には使わないこと**。
// 表示には Redacted() / Prefix() を使う。
func (h ScopeHash) String() string { return h.v }

// Equal は 2 つの scope_hash を比較する。
func (h ScopeHash) Equal(o ScopeHash) bool { return h.v == o.v }

// IsZero は未初期化判定。
func (h ScopeHash) IsZero() bool { return h.v == "" }

// Prefix は先頭 N 文字（既定 8）を返す。cmd/ops / log 表示用。
func (h ScopeHash) Prefix() string {
	if len(h.v) <= redactPrefixLen {
		return h.v
	}
	return h.v[:redactPrefixLen]
}

// Redacted は redact 表示「<prefix>...」を返す。cmd/ops / log 表示用。
//
//	hash の完全値は出さない方針（`.agents/rules/security-guard.md`）。
func (h ScopeHash) Redacted() string {
	if h.IsZero() {
		return "<empty>"
	}
	return h.Prefix() + "..."
}
