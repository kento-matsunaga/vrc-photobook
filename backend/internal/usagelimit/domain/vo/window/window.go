// Package window は UsageLimit の固定窓（fixed window）値オブジェクト。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §6（fixed window 方式）
//   - migrations/00018_create_usage_counters.sql
//
// fixed window の意味:
//   - 同一 (scope, action) の使用回数を、長さ `seconds` 秒の連続する窓で集計する
//   - WindowStartFor(now) で「now が属する窓の開始時刻」を確定的に返す
//   - 同窓内では count がインクリメント、別窓に移ると新しい行として扱う
//
// MVP は **UTC 基準で seconds 単位の floor**（now を seconds 単位に切り捨て）で窓を決める。
// これにより:
//   - clock skew があっても windowStart の境界が tick 単位に揃う
//   - 全 instance / DB が同じ windowStart に合意可能
//   - 攻撃者は境界をまたぐタイミングを狙えるが、MVP では許容（§17 リスク）
package window

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidWindowSeconds は seconds <= 0 を渡されたとき。
var ErrInvalidWindowSeconds = errors.New("invalid window seconds (must be > 0)")

// Window は固定窓のメタデータ（長さ秒）。
type Window struct {
	seconds int64
}

// New は seconds 秒の固定窓 VO を生成する。
func New(seconds int) (Window, error) {
	if seconds <= 0 {
		return Window{}, fmt.Errorf("%w: %d", ErrInvalidWindowSeconds, seconds)
	}
	return Window{seconds: int64(seconds)}, nil
}

// MustNew はエラーがあれば panic する。コンスト初期化用。
func MustNew(seconds int) Window {
	w, err := New(seconds)
	if err != nil {
		panic(err)
	}
	return w
}

// Seconds は窓の長さ（秒）。
func (w Window) Seconds() int { return int(w.seconds) }

// Duration は窓の長さ（time.Duration）。
func (w Window) Duration() time.Duration { return time.Duration(w.seconds) * time.Second }

// IsZero は VO 未初期化判定。
func (w Window) IsZero() bool { return w.seconds == 0 }

// StartFor は now が属する窓の開始時刻（UTC、seconds 単位 floor）を返す。
//
// 例: seconds=3600（1 時間）/ now=2026-04-30T05:37:42Z → 2026-04-30T05:00:00Z
func (w Window) StartFor(now time.Time) time.Time {
	if w.IsZero() {
		return time.Time{}
	}
	utc := now.UTC()
	floored := utc.Unix() / w.seconds * w.seconds
	return time.Unix(floored, 0).UTC()
}

// EndFor は now が属する窓の終了時刻（exclusive、UTC）を返す。
//
// EndFor = StartFor + seconds
func (w Window) EndFor(now time.Time) time.Time {
	start := w.StartFor(now)
	if start.IsZero() {
		return time.Time{}
	}
	return start.Add(w.Duration())
}

// RetryAfterSeconds は now から窓終了までの残り秒数（最低 1 秒、切り上げ）を返す。
//
// HTTP 429 Retry-After header に渡すための値。
// 既に窓終了を過ぎていれば 1 秒を返す（負値を返さない）。
func (w Window) RetryAfterSeconds(now time.Time) int {
	end := w.EndFor(now)
	if end.IsZero() {
		return 0
	}
	diff := end.Sub(now.UTC())
	if diff <= 0 {
		return 1
	}
	// 切り上げ（4.2 秒 → 5 秒）
	secs := int(diff / time.Second)
	if diff%time.Second > 0 {
		secs++
	}
	if secs < 1 {
		secs = 1
	}
	return secs
}
