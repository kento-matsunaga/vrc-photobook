// window の単体テスト。テーブル駆動 + description（`.agents/rules/testing.md`）。
package window

import (
	"errors"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		description string
		seconds     int
		wantErr     bool
	}{
		{
			name:        "正常_300秒5分窓",
			description: "Given: 300, When: New, Then: 成功",
			seconds:     300,
		},
		{
			name:        "正常_3600秒1時間窓",
			description: "Given: 3600, When: New, Then: 成功",
			seconds:     3600,
		},
		{
			name:        "異常_0秒",
			description: "Given: 0, When: New, Then: ErrInvalidWindowSeconds",
			seconds:     0,
			wantErr:     true,
		},
		{
			name:        "異常_負",
			description: "Given: -1, When: New, Then: ErrInvalidWindowSeconds",
			seconds:     -1,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := New(tt.seconds)
			if tt.wantErr {
				if err == nil || !errors.Is(err, ErrInvalidWindowSeconds) {
					t.Fatalf("err = %v want ErrInvalidWindowSeconds", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v want nil", err)
			}
			if w.Seconds() != tt.seconds {
				t.Errorf("Seconds = %d want %d", w.Seconds(), tt.seconds)
			}
			if w.Duration() != time.Duration(tt.seconds)*time.Second {
				t.Errorf("Duration mismatch")
			}
			if w.IsZero() {
				t.Errorf("IsZero = true want false")
			}
		})
	}
}

func TestStartFor(t *testing.T) {
	tests := []struct {
		name        string
		description string
		seconds     int
		now         time.Time
		wantStart   time.Time
	}{
		{
			name:        "正常_3600秒_5時37分は5時00分から",
			description: "Given: 3600s window, now=05:37:42Z, Then: start=05:00:00Z",
			seconds:     3600,
			now:         time.Date(2026, 4, 30, 5, 37, 42, 0, time.UTC),
			wantStart:   time.Date(2026, 4, 30, 5, 0, 0, 0, time.UTC),
		},
		{
			name:        "正常_3600秒_境界00分00秒はそのまま",
			description: "Given: 3600s window, now=05:00:00Z, Then: start=05:00:00Z",
			seconds:     3600,
			now:         time.Date(2026, 4, 30, 5, 0, 0, 0, time.UTC),
			wantStart:   time.Date(2026, 4, 30, 5, 0, 0, 0, time.UTC),
		},
		{
			name:        "正常_300秒_5分窓",
			description: "Given: 300s window, now=12:34:56Z, Then: start=12:30:00Z",
			seconds:     300,
			now:         time.Date(2026, 4, 30, 12, 34, 56, 0, time.UTC),
			wantStart:   time.Date(2026, 4, 30, 12, 30, 0, 0, time.UTC),
		},
		{
			name:        "正常_JST入力でもUTCで計算",
			description: "Given: 3600s window, now=05:37 +09:00 → UTC=20:37, Then: start=20:00 UTC",
			seconds:     3600,
			now:         time.Date(2026, 4, 30, 5, 37, 0, 0, time.FixedZone("JST", 9*3600)),
			wantStart:   time.Date(2026, 4, 29, 20, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := MustNew(tt.seconds)
			got := w.StartFor(tt.now)
			if !got.Equal(tt.wantStart) {
				t.Errorf("StartFor = %v want %v", got, tt.wantStart)
			}
		})
	}
}

func TestEndForAndRetryAfterSeconds(t *testing.T) {
	tests := []struct {
		name              string
		description       string
		seconds           int
		now               time.Time
		wantEnd           time.Time
		wantRetryAfterSec int
	}{
		{
			name:              "正常_3600秒_5時37分残り23分=1380秒",
			description:       "Given: 3600s, now=05:37:00Z, end=06:00:00Z, retryAfter=1380s",
			seconds:           3600,
			now:               time.Date(2026, 4, 30, 5, 37, 0, 0, time.UTC),
			wantEnd:           time.Date(2026, 4, 30, 6, 0, 0, 0, time.UTC),
			wantRetryAfterSec: 1380,
		},
		{
			name:              "正常_300秒_境界即時",
			description:       "Given: 300s, now=12:34:59Z, end=12:35:00Z, retryAfter=1s",
			seconds:           300,
			now:               time.Date(2026, 4, 30, 12, 34, 59, 0, time.UTC),
			wantEnd:           time.Date(2026, 4, 30, 12, 35, 0, 0, time.UTC),
			wantRetryAfterSec: 1,
		},
		{
			name:              "正常_切り上げ_500ms残り",
			description:       "Given: 3600s, now=05:59:59.500Z, end=06:00:00Z, retryAfter=1s（切り上げ）",
			seconds:           3600,
			now:               time.Date(2026, 4, 30, 5, 59, 59, 500_000_000, time.UTC),
			wantEnd:           time.Date(2026, 4, 30, 6, 0, 0, 0, time.UTC),
			wantRetryAfterSec: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := MustNew(tt.seconds)
			if got := w.EndFor(tt.now); !got.Equal(tt.wantEnd) {
				t.Errorf("EndFor = %v want %v", got, tt.wantEnd)
			}
			if got := w.RetryAfterSeconds(tt.now); got != tt.wantRetryAfterSec {
				t.Errorf("RetryAfterSeconds = %d want %d", got, tt.wantRetryAfterSec)
			}
		})
	}
}

func TestRetryAfterSeconds_PastWindowReturnsOne(t *testing.T) {
	// 窓終了を過ぎた now でも 1 秒以上を返す（負値を返さない）
	w := MustNew(60)
	end := w.EndFor(time.Date(2026, 4, 30, 5, 0, 0, 0, time.UTC))
	pastNow := end.Add(time.Second) // 窓終了 1 秒後
	got := w.RetryAfterSeconds(pastNow)
	if got < 1 {
		t.Errorf("RetryAfterSeconds = %d want >= 1", got)
	}
}
