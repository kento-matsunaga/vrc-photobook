// CheckAndConsumeUsage は UsageLimit の核 UseCase。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §8 / §17.3
//
// 振る舞い:
//   1. Window.StartFor(now) で固定窓開始時刻を算出
//   2. Repository.UpsertAndIncrement で atomic +1
//   3. 戻り値 count > limit なら ErrRateLimited、以下なら成功
//   4. Repository 失敗時は **fail-closed**（ErrUsageRepositoryFailed → 上位 429 マップ）
//
// セキュリティ:
//   - エラーや log に scope_hash 完全値を出さない（呼び出し側が redact してログする）
//   - 本 UseCase は raw IP / token / Cookie / manage URL を扱わない
//     （scope_hash は呼び出し側が事前に算出して VO で渡す）
package usecase

import (
	"context"
	"time"

	"vrcpb/backend/internal/usagelimit/domain/vo/action"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_hash"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_type"
	"vrcpb/backend/internal/usagelimit/domain/vo/window"
	rdb "vrcpb/backend/internal/usagelimit/infrastructure/repository/rdb"
)

// UpsertRepo は CheckAndConsumeUsage が依存する最小 interface。
//
// テストでの fake / Repository 失敗注入を容易にする。
// 本番では `*rdb.UsageCounterRepository` がこの interface を満たす。
type UpsertRepo interface {
	UpsertAndIncrement(
		ctx context.Context,
		st scope_type.ScopeType,
		hash scope_hash.ScopeHash,
		act action.Action,
		windowStart time.Time,
		windowSecs int,
		limit int,
		expiresAt time.Time,
		now time.Time,
	) (rdb.UpsertResult, error)
}

// CheckInput は CheckAndConsume の入力。
type CheckInput struct {
	ScopeType            scope_type.ScopeType
	ScopeHash            scope_hash.ScopeHash
	Action               action.Action
	Now                  time.Time
	WindowSeconds        int
	Limit                int
	RetentionGraceSecs   int // expires_at = window_end + grace
}

// CheckOutput は CheckAndConsume の出力。
type CheckOutput struct {
	Count             int
	Limit             int
	WindowStart       time.Time
	ResetAt           time.Time // 窓終端（exclusive）
	RetryAfterSeconds int       // 窓終端までの残り秒数（>=1）
}

// CheckAndConsumeUsage は UsageLimit の核 UseCase。
//
// MVP: fail-closed（Repository 失敗時は ErrUsageRepositoryFailed を返し、上位 handler が
// 429 にマップする）。fail-open flag は PR36 範囲外（計画書 §17.3）。
type CheckAndConsumeUsage struct {
	repo UpsertRepo
}

// NewCheckAndConsumeUsage は UseCase を組み立てる。
func NewCheckAndConsumeUsage(repo UpsertRepo) *CheckAndConsumeUsage {
	return &CheckAndConsumeUsage{repo: repo}
}

// Execute は同 (scope_type, scope_hash, action, window_start) のカウンターを atomic に
// +1 し、limit 超過なら ErrRateLimited を返す。
func (u *CheckAndConsumeUsage) Execute(ctx context.Context, in CheckInput) (CheckOutput, error) {
	w, err := window.New(in.WindowSeconds)
	if err != nil {
		// fail-closed: 呼び出し側設定エラー（窓秒数 0 等）も deny に倒す
		return CheckOutput{}, ErrUsageRepositoryFailed
	}
	if in.Limit <= 0 {
		return CheckOutput{}, ErrUsageRepositoryFailed
	}
	windowStart := w.StartFor(in.Now)
	windowEnd := w.EndFor(in.Now)
	expiresAt := windowEnd.Add(time.Duration(in.RetentionGraceSecs) * time.Second)
	if !expiresAt.After(windowStart) {
		// retention grace が 0 で window_seconds=1 等の極端ケースで等値になり得る。
		// migration の CHECK 制約 (expires_at > window_start) を満たすため最低 1 秒底上げ
		expiresAt = windowStart.Add(time.Duration(in.WindowSeconds) * time.Second).Add(time.Second)
	}

	res, err := u.repo.UpsertAndIncrement(
		ctx,
		in.ScopeType,
		in.ScopeHash,
		in.Action,
		windowStart,
		in.WindowSeconds,
		in.Limit,
		expiresAt,
		in.Now.UTC(),
	)
	if err != nil {
		// fail-closed: Repository 失敗は安全側（deny）に倒す。
		// 呼び出し側で `errors.Is(err, ErrUsageRepositoryFailed)` をチェックして 429。
		// raw error を log に出すかは呼び出し側の責務（scope_hash 完全値を出さない方針）。
		return CheckOutput{}, ErrUsageRepositoryFailed
	}

	out := CheckOutput{
		Count:             res.Count,
		Limit:             in.Limit,
		WindowStart:       res.WindowStart,
		ResetAt:           windowEnd,
		RetryAfterSeconds: w.RetryAfterSeconds(in.Now),
	}
	if res.Count > in.Limit {
		return out, ErrRateLimited
	}
	return out, nil
}
