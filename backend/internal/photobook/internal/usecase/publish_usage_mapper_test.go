// MapPublishUsageErr の unit test。実 DB / pool 不要。
package usecase

import (
	"errors"
	"testing"

	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

func TestMapPublishUsageErr(t *testing.T) {
	tests := []struct {
		name              string
		description       string
		inErr             error
		inRetryAfter      int
		wantWrapper       bool
		wantRetryAfterSec int
		wantCause         error
	}{
		{
			name:              "正常_threshold超過_RateLimited保持",
			description:       "Given: ErrRateLimited / retryAfter=1800, Then: wrapper(1800, ErrPublishRateLimited)",
			inErr:             usagelimitwireup.ErrRateLimited,
			inRetryAfter:      1800,
			wantWrapper:       true,
			wantRetryAfterSec: 1800,
			wantCause:         ErrPublishRateLimited,
		},
		{
			name:              "正常_retryAfter0は1秒底上げ",
			description:       "Given: ErrRateLimited / retryAfter=0, Then: 1秒",
			inErr:             usagelimitwireup.ErrRateLimited,
			inRetryAfter:      0,
			wantWrapper:       true,
			wantRetryAfterSec: 1,
			wantCause:         ErrPublishRateLimited,
		},
		{
			name:              "正常_repo失敗_fail_closed_60秒",
			description:       "Given: ErrUsageRepositoryFailed, Then: wrapper(60, ErrPublishRateLimiterUnavailable)",
			inErr:             usagelimitwireup.ErrUsageRepositoryFailed,
			inRetryAfter:      0,
			wantWrapper:       true,
			wantRetryAfterSec: 60,
			wantCause:         ErrPublishRateLimiterUnavailable,
		},
		{
			name:        "正常_その他は透過",
			description: "Given: 任意エラー, Then: wrapper 化しない",
			inErr:       errors.New("other"),
			wantWrapper: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapPublishUsageErr(tt.inErr, tt.inRetryAfter)
			var rl *PublishRateLimited
			isW := errors.As(got, &rl)
			if isW != tt.wantWrapper {
				t.Fatalf("wrapper = %v want %v (got=%v)", isW, tt.wantWrapper, got)
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
