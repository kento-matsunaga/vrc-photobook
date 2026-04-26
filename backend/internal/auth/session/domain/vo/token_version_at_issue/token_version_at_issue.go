// Package token_version_at_issue は TokenVersionAtIssue 値オブジェクトを提供する。
//
// TokenVersionAtIssue は session 発行時の Photobook.manage_url_token_version の snapshot。
// manage_url_token 再発行（reissueManageUrl）時に、旧 version 以下の session を一括 revoke するために使う。
//
// 設計参照: docs/design/auth/session/ドメイン設計.md / docs/adr/0003-frontend-token-session-flow.md
//
// 不変条件:
//   - 0 以上（CHECK sessions_token_version_nonneg_check）
//   - draft session では常に 0（CHECK sessions_draft_token_version_zero_check）
//
// draft 用の制約は Session entity 側で組み立て時に保証する。本 VO は単に「0 以上の整数」を担保する。
package token_version_at_issue

import (
	"errors"
	"fmt"
)

// ErrNegativeVersion は負の値を渡したときのエラー。
var ErrNegativeVersion = errors.New("token version must be non-negative")

// TokenVersionAtIssue は manage_url_token_version の snapshot。
type TokenVersionAtIssue struct {
	v int
}

// New は 0 以上の int を TokenVersionAtIssue として受け取る。
func New(v int) (TokenVersionAtIssue, error) {
	if v < 0 {
		return TokenVersionAtIssue{}, fmt.Errorf("%w: %d", ErrNegativeVersion, v)
	}
	return TokenVersionAtIssue{v: v}, nil
}

// Zero は 0 値（draft 用の固定値）を返す。
func Zero() TokenVersionAtIssue {
	return TokenVersionAtIssue{v: 0}
}

// Int は int 表現を返す。永続化層との境界でのみ使用する。
func (t TokenVersionAtIssue) Int() int {
	return t.v
}

// IsZero は 0 かどうかを返す。
func (t TokenVersionAtIssue) IsZero() bool {
	return t.v == 0
}

// Equal は値による等価判定。
func (t TokenVersionAtIssue) Equal(other TokenVersionAtIssue) bool {
	return t.v == other.v
}
