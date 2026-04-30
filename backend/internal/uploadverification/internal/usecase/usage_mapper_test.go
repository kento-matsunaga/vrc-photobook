// MapUsageErr の unit test。実 DB / pool 不要。
package usecase_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/uploadverification/internal/usecase"
	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

func TestMapUsageErr(t *testing.T) {
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
			name:              "正常_threshold超過_RateLimited",
			description:       "Given: ErrRateLimited / retryAfter=900, Then: wrapper(900, ErrRateLimited)",
			inErr:             usagelimitwireup.ErrRateLimited,
			inRetryAfter:      900,
			wantWrapper:       true,
			wantRetryAfterSec: 900,
			wantCause:         usecase.ErrRateLimited,
		},
		{
			name:              "正常_retryAfter0は1秒底上げ",
			description:       "Given: ErrRateLimited / retryAfter=0, Then: 1秒",
			inErr:             usagelimitwireup.ErrRateLimited,
			inRetryAfter:      0,
			wantWrapper:       true,
			wantRetryAfterSec: 1,
			wantCause:         usecase.ErrRateLimited,
		},
		{
			name:              "正常_repo失敗_fail_closed_60秒",
			description:       "Given: ErrUsageRepositoryFailed, Then: wrapper(60, ErrRateLimiterUnavailable)",
			inErr:             usagelimitwireup.ErrUsageRepositoryFailed,
			inRetryAfter:      0,
			wantWrapper:       true,
			wantRetryAfterSec: 60,
			wantCause:         usecase.ErrRateLimiterUnavailable,
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
			got := usecase.MapUsageErr(tt.inErr, tt.inRetryAfter)
			var rl *usecase.RateLimited
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
