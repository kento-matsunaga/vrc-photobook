// Package turnstile は Cloudflare Turnstile siteverify の抽象。
//
// 設計参照:
//   - docs/plan/m2-upload-verification-plan.md §5
//   - docs/adr/0005-image-upload-flow.md §Turnstile 検証
//
// セキュリティ:
//   - response body / error-codes / remoteip / hostname / action はログに出さない
//   - fail-closed（Cloudflare 障害時は ErrUnavailable を返し、上位で 503 案内）
package turnstile

import (
	"context"
	"errors"
	"time"
)

// エラー。
var (
	// ErrVerificationFailed は siteverify で success=false が返ったときの内部エラー。
	// 上位は外部に詳細を出さず、403 系で reason=`turnstile_verification_failed` 相当を返す。
	ErrVerificationFailed = errors.New("turnstile verification failed")

	// ErrUnavailable は Cloudflare API への接続失敗（timeout / network エラー / 5xx）。
	// 上位は外部に 503 + `turnstile_unavailable` 相当を返す（fail-closed）。
	ErrUnavailable = errors.New("turnstile siteverify unavailable")

	// ErrHostnameMismatch / ErrActionMismatch / ErrChallengeStale は内部用の分類。
	// 外部レスポンスでは ErrVerificationFailed と一括扱い（bot 学習防止）。
	ErrHostnameMismatch = errors.New("turnstile hostname mismatch")
	ErrActionMismatch   = errors.New("turnstile action mismatch")
	ErrChallengeStale   = errors.New("turnstile challenge timestamp too old")
)

// VerifyInput は Verify 呼び出しの引数。
//
// Token は widget が返した response token。RemoteIP は X-Forwarded-For 末尾 /
// Cf-Connecting-IP から取った値（任意）。
type VerifyInput struct {
	Token    string
	RemoteIP string
	Action   string // 期待値 e.g. "upload"
	Hostname string // 期待値 e.g. "app.vrc-photobook.com"
}

// VerifyOutput は siteverify response の正常系を VO 化したもの。
//
// ErrorCodes は外部に出さない方針だが、内部 logs / metrics でカテゴリ化したい
// 場合のために配列で保持する。
type VerifyOutput struct {
	Success     bool
	ChallengeTs time.Time
	Hostname    string
	Action      string
	ErrorCodes  []string
}

// Verifier は Cloudflare Turnstile siteverify を抽象化する。
//
// 本番は CloudflareVerifier 実装、テストは Fake を使う。
type Verifier interface {
	Verify(ctx context.Context, in VerifyInput) (VerifyOutput, error)
}
