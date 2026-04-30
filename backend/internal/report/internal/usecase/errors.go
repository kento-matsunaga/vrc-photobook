// Package usecase: Report UseCase 群（PR35b）。
//
// 設計参照:
//   - docs/plan/m2-report-plan.md §5
//   - docs/design/aggregates/report/ドメイン設計.md §6
//
// セキュリティ:
//   - reporter_contact / detail / source_ip_hash は Outbox payload に入れない
//   - 公開 endpoint への外部応答に内部理由を漏らさない（敵対者対策で 404 / 403 / 400 / 500 のみ）
//   - source_ip_hash 完全値は log / chat に出さない
package usecase

import "errors"

// 共通エラー（HTTP layer で 400 / 403 / 404 / 500 へ変換）。
var (
	// ErrTargetNotEligibleForReport は対象 photobook が不在 / 公開対象外。
	// 内部分岐は draft / private / hidden / deleted / purged を区別しない（外部応答 404）。
	ErrTargetNotEligibleForReport = errors.New("report: target photobook not eligible")

	// ErrTurnstileTokenMissing は token が空。
	ErrTurnstileTokenMissing = errors.New("report: turnstile token missing")

	// ErrTurnstileVerificationFailed は siteverify が success=false / 不一致。
	ErrTurnstileVerificationFailed = errors.New("report: turnstile verification failed")

	// ErrTurnstileUnavailable は Cloudflare 接続失敗等。
	ErrTurnstileUnavailable = errors.New("report: turnstile unavailable")

	// ErrInvalidSlug は slug 文字列が VO 仕様外。
	ErrInvalidSlug = errors.New("report: invalid slug")

	// ErrSaltNotConfigured は REPORT_IP_HASH_SALT_V1 未設定（Secret Manager 注入忘れ）。
	ErrSaltNotConfigured = errors.New("report: REPORT_IP_HASH_SALT_V1 not configured")

	// ErrRateLimited は UsageLimit 閾値超過（HTTP 429）。
	// 同一 IP × 同一 photobook の短時間連投、または同一 IP の長期累積で発火。
	ErrRateLimited = errors.New("report: rate limited")

	// ErrRateLimiterUnavailable は UsageLimit Repository 失敗（fail-closed で HTTP 429）。
	// 内部 log には「usage repository failed」程度のみ、scope_hash 完全値は出さない。
	ErrRateLimiterUnavailable = errors.New("report: rate limiter unavailable")
)

// RateLimited は ErrRateLimited / ErrRateLimiterUnavailable を返すときに、
// HTTP 層が Retry-After header に渡す秒数（>=1）を伴って投げるためのラップ型。
type RateLimited struct {
	RetryAfterSeconds int
	// Cause は ErrRateLimited（threshold 超過）または ErrRateLimiterUnavailable（fail-closed）。
	Cause error
}

// Error は error interface。
func (e *RateLimited) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "report: rate limited"
}

// Unwrap は errors.Is で Cause を判定できるようにする。
func (e *RateLimited) Unwrap() error { return e.Cause }
