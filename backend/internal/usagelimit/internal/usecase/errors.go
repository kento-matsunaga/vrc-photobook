// Package usecase は UsageLimit UseCase 群を提供する。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §8.4
package usecase

import "errors"

var (
	// ErrRateLimited は閾値超過。HTTP 429 にマップ。
	ErrRateLimited = errors.New("usagelimit: rate limited")

	// ErrUsageRepositoryFailed は DB 障害等。MVP では fail-closed として呼び出し側が
	// 上位 429 にマップする。fail-open フラグは将来 PR で別途実装する。
	ErrUsageRepositoryFailed = errors.New("usagelimit: repository failed")
)
