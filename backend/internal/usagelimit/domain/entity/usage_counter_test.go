// usage_counter entity の単体テスト。テーブル駆動 + description（`.agents/rules/testing.md`）。
package entity

import (
	"errors"
	"strings"
	"testing"
	"time"

	"vrcpb/backend/internal/usagelimit/domain/vo/action"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_hash"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_type"
)

func mustHash(t *testing.T, s string) scope_hash.ScopeHash {
	t.Helper()
	h, err := scope_hash.Parse(s)
	if err != nil {
		t.Fatalf("scope_hash.Parse: %v", err)
	}
	return h
}

func validParams(t *testing.T) NewParams {
	t.Helper()
	now := time.Date(2026, 4, 30, 5, 0, 0, 0, time.UTC)
	return NewParams{
		ScopeType:       scope_type.SourceIPHash(),
		ScopeHash:       mustHash(t, strings.Repeat("a", 64)),
		Action:          action.ReportSubmit(),
		WindowStart:     now,
		WindowSeconds:   3600,
		Count:           1,
		LimitAtCreation: 20,
		CreatedAt:       now,
		UpdatedAt:       now,
		ExpiresAt:       now.Add(2 * time.Hour),
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		description string
		mutate      func(p *NewParams)
		wantErr     bool
	}{
		{
			name:        "正常_全フィールド有効",
			description: "Given: 妥当な params, When: New, Then: 成功",
		},
		{
			name:        "異常_scope_type未設定",
			description: "Given: ScopeType=zero, When: New, Then: 失敗",
			mutate:      func(p *NewParams) { p.ScopeType = scope_type.ScopeType{} },
			wantErr:     true,
		},
		{
			name:        "異常_scope_hash未設定",
			description: "Given: ScopeHash=zero, When: New, Then: 失敗",
			mutate:      func(p *NewParams) { p.ScopeHash = scope_hash.ScopeHash{} },
			wantErr:     true,
		},
		{
			name:        "異常_action未設定",
			description: "Given: Action=zero, When: New, Then: 失敗",
			mutate:      func(p *NewParams) { p.Action = action.Action{} },
			wantErr:     true,
		},
		{
			name:        "異常_window_start未設定",
			description: "Given: WindowStart=zero, When: New, Then: 失敗",
			mutate:      func(p *NewParams) { p.WindowStart = time.Time{} },
			wantErr:     true,
		},
		{
			name:        "異常_window_seconds_0",
			description: "Given: WindowSeconds=0, When: New, Then: 失敗",
			mutate:      func(p *NewParams) { p.WindowSeconds = 0 },
			wantErr:     true,
		},
		{
			name:        "異常_count_negative",
			description: "Given: Count=-1, When: New, Then: 失敗",
			mutate:      func(p *NewParams) { p.Count = -1 },
			wantErr:     true,
		},
		{
			name:        "異常_limit_at_creation_0",
			description: "Given: LimitAtCreation=0, When: New, Then: 失敗",
			mutate:      func(p *NewParams) { p.LimitAtCreation = 0 },
			wantErr:     true,
		},
		{
			name:        "異常_expires_at_window_start以下",
			description: "Given: ExpiresAt=WindowStart, When: New, Then: 失敗",
			mutate: func(p *NewParams) {
				p.ExpiresAt = p.WindowStart
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validParams(t)
			if tt.mutate != nil {
				tt.mutate(&p)
			}
			c, err := New(p)
			if tt.wantErr {
				if err == nil || !errors.Is(err, ErrInvalidUsageCounter) {
					t.Fatalf("err = %v want ErrInvalidUsageCounter", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v want nil", err)
			}
			if c.Count() != p.Count {
				t.Errorf("Count = %d want %d", c.Count(), p.Count)
			}
			if c.WindowEnd().Sub(c.WindowStart()) != time.Duration(p.WindowSeconds)*time.Second {
				t.Errorf("WindowEnd mismatch")
			}
			// 完全値は assert ログに出さない（Redacted で確認）
			if c.ScopeHashRedacted() == "" {
				t.Errorf("ScopeHashRedacted = empty")
			}
		})
	}
}

func TestIsOverLimit(t *testing.T) {
	tests := []struct {
		name        string
		description string
		count       int
		limit       int
		want        bool
	}{
		{
			name:        "正常_count未満",
			description: "Given: count=10 / limit=20, Then: not over",
			count:       10, limit: 20, want: false,
		},
		{
			name:        "正常_count一致",
			description: "Given: count=20 / limit=20, Then: not over (>= 不可、> のみ)",
			count:       20, limit: 20, want: false,
		},
		{
			name:        "異常_count超過",
			description: "Given: count=21 / limit=20, Then: over",
			count:       21, limit: 20, want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validParams(t)
			p.Count = tt.count
			p.LimitAtCreation = tt.limit
			c, err := New(p)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if got := c.IsOverLimit(tt.limit); got != tt.want {
				t.Errorf("IsOverLimit = %v want %v", got, tt.want)
			}
		})
	}
}
