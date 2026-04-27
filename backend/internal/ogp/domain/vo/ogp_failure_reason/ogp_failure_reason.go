// Package ogp_failure_reason は photobook_ogp_images.failure_reason の VO。
//
// 200 char 上限（DB CHECK 制約と一致）。worker / renderer は本 VO 経由で書くこと
// で sanitize と長さ制御を統一する。
//
// セキュリティ:
//   - 危険語（postgres:// / Bearer / Set-Cookie / DATABASE_URL / R2_SECRET /
//     TURNSTILE_SECRET / presigned）を含むメッセージは [REDACTED] で置換
//   - 200 char を超える場合は ... で truncate
package ogp_failure_reason

import (
	"errors"
	"fmt"
	"strings"
)

const MaxLen = 200

var ErrTooLong = errors.New("ogp failure reason exceeds 200 chars")

type OgpFailureReason struct {
	v string
}

// Sanitize は raw error message から OgpFailureReason を作る。
//
// - nil / 空文字 → 空 VO（IsZero=true）
// - 危険語含み → "[REDACTED] <error_type>" 形式
// - 200 超 → 197 文字 + "..."
func Sanitize(err error) OgpFailureReason {
	if err == nil {
		return OgpFailureReason{}
	}
	msg := err.Error()
	if msg == "" {
		return OgpFailureReason{}
	}
	for _, danger := range []string{
		"postgres://", "Bearer ", "Set-Cookie", "DATABASE_URL",
		"R2_SECRET", "TURNSTILE_SECRET", "presigned",
	} {
		if containsFold(msg, danger) {
			return OgpFailureReason{v: "[REDACTED] " + fmt.Sprintf("%T", err)}
		}
	}
	if len(msg) > MaxLen {
		return OgpFailureReason{v: msg[:MaxLen-3] + "..."}
	}
	return OgpFailureReason{v: msg}
}

// FromTrustedString は VO の単純な復元用（DB → application）。
// 200 char を超える場合はエラー（CHECK 制約で来ないはずだが防衛的に検査）。
func FromTrustedString(s string) (OgpFailureReason, error) {
	if len(s) > MaxLen {
		return OgpFailureReason{}, fmt.Errorf("%w: %d", ErrTooLong, len(s))
	}
	return OgpFailureReason{v: s}, nil
}

func (r OgpFailureReason) String() string { return r.v }
func (r OgpFailureReason) IsZero() bool   { return r.v == "" }

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
