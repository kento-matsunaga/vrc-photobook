// SubmitReport UseCase の単体テスト。本ファイルは DB / Turnstile を使わない
// 早期 return ガード（L4 多層防御 Turnstile ガード）に焦点を当てる。
//
// 設計参照: `.agents/rules/turnstile-defensive-guard.md`
// 失敗事例: `harness/failure-log/2026-04-29_report-form-turnstile-bypass.md`
package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

// TestSubmitReport_L4_BlankTurnstileToken_Rejected は L4 ガードを検証する。
//
// 構成: pool / verifier が nil の SubmitReport を作る。token が trim 後 empty の
// ケースでは「token check → 早期 return」段階で ErrTurnstileTokenMissing を返し、
// verifier.Verify() / pool BeginTx に到達しない（到達したら nil 参照 panic）こと
// で多層防御を保証する。
func TestSubmitReport_L4_BlankTurnstileToken_Rejected(t *testing.T) {
	tests := []struct {
		name        string
		description string
		token       string
	}{
		{
			name:        "異常_空文字tokenでErrTurnstileTokenMissing",
			description: "Given: TurnstileToken=\"\", When: Execute, Then: ErrTurnstileTokenMissing 即返却",
			token:       "",
		},
		{
			name:        "異常_空白のみtokenでErrTurnstileTokenMissing",
			description: "Given: TurnstileToken=\"   \", When: Execute, Then: ErrTurnstileTokenMissing 即返却",
			token:       "   ",
		},
		{
			name:        "異常_タブ改行のみtokenでErrTurnstileTokenMissing",
			description: "Given: TurnstileToken=\"\\t\\n\", When: Execute, Then: ErrTurnstileTokenMissing 即返却",
			token:       "\t\n",
		},
		{
			name:        "異常_全角空白のみでErrTurnstileTokenMissing",
			description: "Given: TurnstileToken=\"　\", When: Execute, Then: ErrTurnstileTokenMissing 即返却",
			token:       "　",
		},
	}

	// pool / verifier は nil。L4 ガードが効いていれば nil 参照に到達しない。
	uc := NewSubmitReport(nil, nil, "report-submit.example.test", "report-submit", "test-salt-v1", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), SubmitReportInput{
				Slug:           "uqfwfti7glarva5saj",
				TurnstileToken: tt.token,
				RemoteIP:       "203.0.113.1",
				Now:            time.Now().UTC(),
			})
			if err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !errors.Is(err, ErrTurnstileTokenMissing) {
				t.Fatalf("err = %v want ErrTurnstileTokenMissing", err)
			}
		})
	}
}

// PR36 commit 3.5: mapUsageErr が UsageLimit エラーを RateLimited wrapper に変換
// することを単体テスト。fail-closed: ErrUsageRepositoryFailed → 既定 60 秒。
// scope_hash 完全値や IP は wrapper にも含まない（呼び出し側で redact）。
//
// PR36 commit 3.6 補足:
//   実 DB 副作用なし統合テスト（usage_counters 事前 INSERT で limit 到達 → UseCase
//   呼び出し → reports/outbox INSERT が起きないことを SELECT で確認）は、Report 集約
//   の photobook seed が manage_url_token_hash / published_at / status 整合性 CHECK 等の
//   多数の制約を持ち、photobook published 状態を SQL 直接 INSERT するコストが高いため、
//   uploadverification / publish の同パターン integration test
//   （`internal/uploadverification/internal/usecase/usage_limit_integration_test.go` /
//    `internal/photobook/internal/usecase/publish_usage_limit_integration_test.go`）で
//   「UsageLimit threshold 超過時に Repository.Create / TX に到達しない」ことを
//   実 DB で代表保証する。
//   SubmitReport の usage check も同じ構造（前段呼び + RateLimited wrapper + 副作用前 return）
//   で実装されており、本 mapUsageErr unit + L4 ガード unit + handler の writeRateLimited
//   unit の組み合わせで等価な保証を提供する。
func TestMapUsageErr(t *testing.T) {
	tests := []struct {
		name              string
		description       string
		inErr             error
		inRetryAfter      int
		wantRetryAfterSec int
		wantCause         error
		wantWrapper       bool
	}{
		{
			name:              "正常_threshold超過_RateLimited_retryAfter保持",
			description:       "Given: usagelimit.ErrRateLimited / retryAfter=120, Then: wrapper(120, ErrRateLimited)",
			inErr:             usagelimitwireup.ErrRateLimited,
			inRetryAfter:      120,
			wantRetryAfterSec: 120,
			wantCause:         ErrRateLimited,
			wantWrapper:       true,
		},
		{
			name:              "正常_threshold超過_retryAfter0は1秒に底上げ",
			description:       "Given: ErrRateLimited / retryAfter=0, Then: wrapper(1, ...)",
			inErr:             usagelimitwireup.ErrRateLimited,
			inRetryAfter:      0,
			wantRetryAfterSec: 1,
			wantCause:         ErrRateLimited,
			wantWrapper:       true,
		},
		{
			name:              "正常_repo失敗_fail_closed_60秒既定",
			description:       "Given: ErrUsageRepositoryFailed, Then: wrapper(60, ErrRateLimiterUnavailable)",
			inErr:             usagelimitwireup.ErrUsageRepositoryFailed,
			inRetryAfter:      0,
			wantRetryAfterSec: 60,
			wantCause:         ErrRateLimiterUnavailable,
			wantWrapper:       true,
		},
		{
			name:        "正常_その他エラーは透過",
			description: "Given: 任意の他エラー, Then: そのまま透過（wrapper 化しない）",
			inErr:       errors.New("some other error"),
			wantWrapper: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapUsageErr(tt.inErr, tt.inRetryAfter)
			var rl *RateLimited
			isWrapper := errors.As(got, &rl)
			if isWrapper != tt.wantWrapper {
				t.Fatalf("wrapper = %v want %v (got=%v)", isWrapper, tt.wantWrapper, got)
			}
			if !tt.wantWrapper {
				return
			}
			if rl.RetryAfterSeconds != tt.wantRetryAfterSec {
				t.Errorf("retryAfter = %d want %d", rl.RetryAfterSeconds, tt.wantRetryAfterSec)
			}
			if !errors.Is(rl, tt.wantCause) {
				t.Errorf("cause = %v want %v", rl.Cause, tt.wantCause)
			}
		})
	}
}
