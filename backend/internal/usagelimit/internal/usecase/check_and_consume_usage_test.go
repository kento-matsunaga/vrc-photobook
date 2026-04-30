// CheckAndConsumeUsage の単体テスト（実 DB 不要、fake repo 利用）。
//
// 観点:
//   - threshold 内で成功（count <= limit）
//   - threshold 超過で ErrRateLimited
//   - 不正設定（windowSeconds<=0 / limit<=0）で fail-closed
//   - Repository 失敗で fail-closed（ErrUsageRepositoryFailed）
//   - RetryAfterSeconds が窓終端までの残り秒数（>=1）
//
// テーブル駆動 + description（`.agents/rules/testing.md`）。
package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"vrcpb/backend/internal/usagelimit/domain/vo/action"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_hash"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_type"
	rdb "vrcpb/backend/internal/usagelimit/infrastructure/repository/rdb"
)

// fakeUpsertRepo は UpsertRepo interface の test double。
//
// preset で count / err を返す。CallCount で呼び出し回数を確認できる。
type fakeUpsertRepo struct {
	nextCount     int
	nextLimit     int
	nextErr       error
	calledCount   int
	lastWindow    time.Time
	lastExpiresAt time.Time
}

func (f *fakeUpsertRepo) UpsertAndIncrement(
	_ context.Context,
	_ scope_type.ScopeType,
	_ scope_hash.ScopeHash,
	_ action.Action,
	windowStart time.Time,
	windowSecs int,
	_ int,
	expiresAt time.Time,
	_ time.Time,
) (rdb.UpsertResult, error) {
	f.calledCount++
	f.lastWindow = windowStart
	f.lastExpiresAt = expiresAt
	if f.nextErr != nil {
		return rdb.UpsertResult{}, f.nextErr
	}
	return rdb.UpsertResult{
		Count:           f.nextCount,
		LimitAtCreation: f.nextLimit,
		WindowStart:     windowStart,
		WindowSeconds:   windowSecs,
		ExpiresAt:       expiresAt,
	}, nil
}

func mustHash(t *testing.T, s string) scope_hash.ScopeHash {
	t.Helper()
	h, err := scope_hash.Parse(s)
	if err != nil {
		t.Fatalf("scope_hash.Parse: %v", err)
	}
	return h
}

func validInput(t *testing.T) CheckInput {
	t.Helper()
	return CheckInput{
		ScopeType:          scope_type.SourceIPHash(),
		ScopeHash:          mustHash(t, strings.Repeat("a", 64)),
		Action:             action.ReportSubmit(),
		Now:                time.Date(2026, 4, 30, 5, 30, 0, 0, time.UTC),
		WindowSeconds:      3600,
		Limit:              20,
		RetentionGraceSecs: 86400,
	}
}

func TestExecute(t *testing.T) {
	tests := []struct {
		name         string
		description  string
		fake         *fakeUpsertRepo
		mutateInput  func(*CheckInput)
		wantErr      error
		wantCount    int
	}{
		{
			name:        "正常_閾値内_成功",
			description: "Given: count 1 / limit 20, When: Execute, Then: 成功 / count=1",
			fake:        &fakeUpsertRepo{nextCount: 1, nextLimit: 20},
			wantCount:   1,
		},
		{
			name:        "正常_閾値ちょうど_成功",
			description: "Given: count 20 / limit 20, When: Execute, Then: 成功（>= ではなく > のみ deny）",
			fake:        &fakeUpsertRepo{nextCount: 20, nextLimit: 20},
			wantCount:   20,
		},
		{
			name:        "異常_閾値超過_RateLimited",
			description: "Given: count 21 / limit 20, When: Execute, Then: ErrRateLimited",
			fake:        &fakeUpsertRepo{nextCount: 21, nextLimit: 20},
			wantErr:     ErrRateLimited,
			wantCount:   21,
		},
		{
			name:        "異常_repo失敗_fail_closed",
			description: "Given: repo error, When: Execute, Then: ErrUsageRepositoryFailed",
			fake:        &fakeUpsertRepo{nextErr: errors.New("boom")},
			wantErr:     ErrUsageRepositoryFailed,
		},
		{
			name:        "異常_window_seconds_0_fail_closed",
			description: "Given: WindowSeconds=0, When: Execute, Then: ErrUsageRepositoryFailed",
			fake:        &fakeUpsertRepo{},
			mutateInput: func(in *CheckInput) { in.WindowSeconds = 0 },
			wantErr:     ErrUsageRepositoryFailed,
		},
		{
			name:        "異常_limit_0_fail_closed",
			description: "Given: Limit=0, When: Execute, Then: ErrUsageRepositoryFailed",
			fake:        &fakeUpsertRepo{},
			mutateInput: func(in *CheckInput) { in.Limit = 0 },
			wantErr:     ErrUsageRepositoryFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput(t)
			if tt.mutateInput != nil {
				tt.mutateInput(&in)
			}
			uc := NewCheckAndConsumeUsage(tt.fake)
			out, err := uc.Execute(context.Background(), in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v want %v", err, tt.wantErr)
				}
				if errors.Is(tt.wantErr, ErrRateLimited) && out.Count != tt.wantCount {
					t.Errorf("Count = %d want %d", out.Count, tt.wantCount)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v want nil", err)
			}
			if out.Count != tt.wantCount {
				t.Errorf("Count = %d want %d", out.Count, tt.wantCount)
			}
			if out.Limit != in.Limit {
				t.Errorf("Limit = %d want %d", out.Limit, in.Limit)
			}
			if out.RetryAfterSeconds < 1 {
				t.Errorf("RetryAfterSeconds = %d want >= 1", out.RetryAfterSeconds)
			}
		})
	}
}

func TestExecute_WindowAlignment(t *testing.T) {
	// Window.StartFor の floor が UseCase 内で正しく適用されることを確認。
	tests := []struct {
		name        string
		description string
		now         time.Time
		windowSecs  int
		wantStart   time.Time
	}{
		{
			name:        "正常_3600秒_5時37分は5時00分から",
			description: "Given: 3600s window, now=05:37:42Z, Then: windowStart=05:00:00Z",
			now:         time.Date(2026, 4, 30, 5, 37, 42, 0, time.UTC),
			windowSecs:  3600,
			wantStart:   time.Date(2026, 4, 30, 5, 0, 0, 0, time.UTC),
		},
		{
			name:        "正常_300秒_5分窓",
			description: "Given: 300s window, now=12:34:56Z, Then: windowStart=12:30:00Z",
			now:         time.Date(2026, 4, 30, 12, 34, 56, 0, time.UTC),
			windowSecs:  300,
			wantStart:   time.Date(2026, 4, 30, 12, 30, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeUpsertRepo{nextCount: 1, nextLimit: 10}
			uc := NewCheckAndConsumeUsage(fake)
			in := validInput(t)
			in.Now = tt.now
			in.WindowSeconds = tt.windowSecs
			_, err := uc.Execute(context.Background(), in)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if !fake.lastWindow.Equal(tt.wantStart) {
				t.Errorf("windowStart = %v want %v", fake.lastWindow, tt.wantStart)
			}
			// expires_at = window_end + retention_grace
			expectedExpire := tt.wantStart.Add(time.Duration(tt.windowSecs) * time.Second).Add(86400 * time.Second)
			if !fake.lastExpiresAt.Equal(expectedExpire) {
				t.Errorf("expires_at = %v want %v", fake.lastExpiresAt, expectedExpire)
			}
		})
	}
}
