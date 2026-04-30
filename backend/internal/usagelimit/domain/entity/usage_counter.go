// Package entity は UsageLimit の集約ルート Entity を提供する。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §6 / §8
//   - migrations/00018_create_usage_counters.sql
//
// UsageCounter は「(scope_type, scope_hash, action, window_start) で一意な
// fixed window のカウンター」を表す。Repository は INSERT ON CONFLICT DO UPDATE で
// race-free に増分する想定で、本 entity 自体は **読み取り view** として扱う。
//
// セキュリティ:
//   - scope_hash 完全値はログ / chat / work-log に出さない（Redacted() を使う）
//   - 本 entity は raw IP / token / Cookie / manage URL / storage_key を保持しない
package entity

import (
	"errors"
	"fmt"
	"time"

	"vrcpb/backend/internal/usagelimit/domain/vo/action"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_hash"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_type"
)

// ErrInvalidUsageCounter は entity 不変条件違反。
var ErrInvalidUsageCounter = errors.New("invalid usage counter entity")

// UsageCounter は usage_counters 1 行に対応する集約ルート。
type UsageCounter struct {
	scopeType       scope_type.ScopeType
	scopeHash       scope_hash.ScopeHash
	action          action.Action
	windowStart     time.Time
	windowSeconds   int
	count           int
	limitAtCreation int
	createdAt       time.Time
	updatedAt       time.Time
	expiresAt       time.Time
}

// NewParams は UsageCounter を組み立てる入力。
type NewParams struct {
	ScopeType       scope_type.ScopeType
	ScopeHash       scope_hash.ScopeHash
	Action          action.Action
	WindowStart     time.Time
	WindowSeconds   int
	Count           int
	LimitAtCreation int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ExpiresAt       time.Time
}

// New は不変条件をチェックして UsageCounter を返す。
func New(p NewParams) (UsageCounter, error) {
	if p.ScopeType.IsZero() {
		return UsageCounter{}, fmt.Errorf("%w: scope_type missing", ErrInvalidUsageCounter)
	}
	if p.ScopeHash.IsZero() {
		return UsageCounter{}, fmt.Errorf("%w: scope_hash missing", ErrInvalidUsageCounter)
	}
	if p.Action.IsZero() {
		return UsageCounter{}, fmt.Errorf("%w: action missing", ErrInvalidUsageCounter)
	}
	if p.WindowStart.IsZero() {
		return UsageCounter{}, fmt.Errorf("%w: window_start missing", ErrInvalidUsageCounter)
	}
	if p.WindowSeconds <= 0 {
		return UsageCounter{}, fmt.Errorf("%w: window_seconds=%d", ErrInvalidUsageCounter, p.WindowSeconds)
	}
	if p.Count < 0 {
		return UsageCounter{}, fmt.Errorf("%w: count=%d", ErrInvalidUsageCounter, p.Count)
	}
	if p.LimitAtCreation <= 0 {
		return UsageCounter{}, fmt.Errorf("%w: limit_at_creation=%d", ErrInvalidUsageCounter, p.LimitAtCreation)
	}
	if !p.ExpiresAt.After(p.WindowStart) {
		return UsageCounter{}, fmt.Errorf("%w: expires_at must be after window_start", ErrInvalidUsageCounter)
	}
	return UsageCounter{
		scopeType:       p.ScopeType,
		scopeHash:       p.ScopeHash,
		action:          p.Action,
		windowStart:     p.WindowStart.UTC(),
		windowSeconds:   p.WindowSeconds,
		count:           p.Count,
		limitAtCreation: p.LimitAtCreation,
		createdAt:       p.CreatedAt.UTC(),
		updatedAt:       p.UpdatedAt.UTC(),
		expiresAt:       p.ExpiresAt.UTC(),
	}, nil
}

// Getter 群（不変、セキュリティ観点で raw scope_hash を出す経路は限定する）。

// ScopeType returns the scope type VO.
func (c UsageCounter) ScopeType() scope_type.ScopeType { return c.scopeType }

// ScopeHash returns the scope hash VO.
//
// **完全値は logs / chat / work-log に出さない方針**。
// 表示用には ScopeHashRedacted() を使う。
func (c UsageCounter) ScopeHash() scope_hash.ScopeHash { return c.scopeHash }

// ScopeHashRedacted returns the redacted form for display.
func (c UsageCounter) ScopeHashRedacted() string { return c.scopeHash.Redacted() }

// Action returns the action VO.
func (c UsageCounter) Action() action.Action { return c.action }

// WindowStart returns the start of the fixed window (UTC).
func (c UsageCounter) WindowStart() time.Time { return c.windowStart }

// WindowSeconds returns the window length in seconds.
func (c UsageCounter) WindowSeconds() int { return c.windowSeconds }

// Count returns the current count in this window.
func (c UsageCounter) Count() int { return c.count }

// LimitAtCreation returns the limit snapshot stored at INSERT time.
func (c UsageCounter) LimitAtCreation() int { return c.limitAtCreation }

// CreatedAt returns the creation timestamp.
func (c UsageCounter) CreatedAt() time.Time { return c.createdAt }

// UpdatedAt returns the last-update timestamp.
func (c UsageCounter) UpdatedAt() time.Time { return c.updatedAt }

// ExpiresAt returns the cleanup target timestamp.
func (c UsageCounter) ExpiresAt() time.Time { return c.expiresAt }

// WindowEnd returns the (exclusive) end of the window.
func (c UsageCounter) WindowEnd() time.Time {
	return c.windowStart.Add(time.Duration(c.windowSeconds) * time.Second)
}

// IsOverLimit は現在閾値（引数で渡す）を超過しているかを返す。
//
// limit_at_creation はあくまで INSERT 時点のスナップショットで、運用中に閾値を変えた
// 場合は呼び出し側が現行値を渡す。
func (c UsageCounter) IsOverLimit(currentLimit int) bool {
	return c.count > currentLimit
}
