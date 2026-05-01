// CreatePhotobook handler のテスト。
//
// 観点:
//   - L4 Turnstile blank token / whitespace-only → 403 turnstile_failed
//   - JSON decode 失敗 → 400 invalid_payload
//   - type が enum 外 → 400 invalid_payload
//   - title / creator_display_name 過長 → 400 invalid_payload
//   - Turnstile siteverify 失敗 → 403 turnstile_failed
//   - Turnstile 利用不可（network 障害）→ 503 turnstile_unavailable
//   - 成功 path は usecase / repo を実 DB に依存するため本 unit test では網羅しない
//     （UsageLimit 緩和 PR と同方針、Safari STOP ε で end-to-end を担保）
//
// 参照: docs/plan/m2-create-entry-plan.md §11.1 / .agents/rules/turnstile-defensive-guard.md L4

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vrcpb/backend/internal/turnstile"
)

// fakeVerifier は turnstile.Verifier を満たす test 用 stub。
type fakeVerifier struct {
	verifyFn func(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error)
	called   int
}

func (f *fakeVerifier) Verify(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error) {
	f.called++
	if f.verifyFn == nil {
		return turnstile.VerifyOutput{Success: true}, nil
	}
	return f.verifyFn(ctx, in)
}

func TestCreatePhotobook_L4_BlankTurnstileToken_Rejected(t *testing.T) {
	tests := []struct {
		name        string
		description string
		token       string
	}{
		{
			name:        "異常_空文字tokenでturnstile_failed",
			description: "Given: turnstile_token=\"\", When: POST /api/photobooks, Then: 403 turnstile_failed + verifier 未呼出",
			token:       "",
		},
		{
			name:        "異常_空白のみtokenでturnstile_failed",
			description: "Given: turnstile_token=\"   \", When: POST, Then: 403 + verifier 未呼出",
			token:       "   ",
		},
		{
			name:        "異常_タブ改行のみtokenでturnstile_failed",
			description: "Given: turnstile_token=\"\\t\\n\", When: POST, Then: 403 + verifier 未呼出",
			token:       "\t\n",
		},
		{
			name:        "異常_全角空白のみでturnstile_failed",
			description: "Given: turnstile_token=\"　\", When: POST, Then: 403 + verifier 未呼出",
			token:       "　",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			verifier := &fakeVerifier{}
			h := NewCreateHandlers(nil, verifier, "test.example", "photobook-create", nil)

			body, _ := json.Marshal(createRequest{Type: "memory", TurnstileToken: tt.token})
			req := httptest.NewRequest(http.MethodPost, "/api/photobooks", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			h.CreatePhotobook(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
			if !strings.Contains(rec.Body.String(), "turnstile_failed") {
				t.Errorf("body = %q, want turnstile_failed", rec.Body.String())
			}
			if verifier.called != 0 {
				t.Errorf("verifier called = %d, want 0 (L4 早期 return で siteverify に到達しない)", verifier.called)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestCreatePhotobook_InvalidPayload(t *testing.T) {
	tests := []struct {
		name        string
		description string
		body        string
		wantStatus  int
		wantBody    string
	}{
		{
			name:        "異常_JSON_decode失敗で400",
			description: "Given: 不正 JSON, When: POST, Then: 400 invalid_payload",
			body:        "not json",
			wantStatus:  http.StatusBadRequest,
			wantBody:    "invalid_payload",
		},
		{
			name:        "異常_type_enum外で400",
			description: "Given: type=\"unknown\", When: POST, Then: 400 invalid_payload (Turnstile siteverify は呼ばれない)",
			body:        `{"type":"unknown","turnstile_token":"valid-token"}`,
			wantStatus:  http.StatusBadRequest,
			wantBody:    "invalid_payload",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			verifier := &fakeVerifier{}
			h := NewCreateHandlers(nil, verifier, "test.example", "photobook-create", nil)

			req := httptest.NewRequest(http.MethodPost, "/api/photobooks", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.CreatePhotobook(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want contain %q", rec.Body.String(), tt.wantBody)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestCreatePhotobook_LengthValidation(t *testing.T) {
	tests := []struct {
		name        string
		description string
		title       string
		creator     string
		wantStatus  int
	}{
		{
			name:        "異常_title過長で400",
			description: "Given: title が maxTitleLen 超過, When: POST, Then: 400",
			title:       strings.Repeat("a", maxTitleLen+1),
			creator:     "",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "異常_creator_display_name過長で400",
			description: "Given: creator_display_name が maxCreatorDisplayNameLen 超過, When: POST, Then: 400",
			title:       "",
			creator:     strings.Repeat("a", maxCreatorDisplayNameLen+1),
			wantStatus:  http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			verifier := &fakeVerifier{}
			h := NewCreateHandlers(nil, verifier, "test.example", "photobook-create", nil)

			body, _ := json.Marshal(createRequest{
				Type:               "memory",
				Title:              tt.title,
				CreatorDisplayName: tt.creator,
				TurnstileToken:     "valid-token",
			})
			req := httptest.NewRequest(http.MethodPost, "/api/photobooks", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			h.CreatePhotobook(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestCreatePhotobook_TurnstileVerifyFailures(t *testing.T) {
	tests := []struct {
		name        string
		description string
		verifyErr   error
		wantStatus  int
		wantBody    string
	}{
		{
			name:        "異常_siteverify失敗で403",
			description: "Given: ErrVerificationFailed, When: POST, Then: 403 turnstile_failed",
			verifyErr:   turnstile.ErrVerificationFailed,
			wantStatus:  http.StatusForbidden,
			wantBody:    "turnstile_failed",
		},
		{
			name:        "異常_hostname不一致で403",
			description: "Given: ErrHostnameMismatch, When: POST, Then: 403 turnstile_failed (理由を区別しない)",
			verifyErr:   turnstile.ErrHostnameMismatch,
			wantStatus:  http.StatusForbidden,
			wantBody:    "turnstile_failed",
		},
		{
			name:        "異常_action不一致で403",
			description: "Given: ErrActionMismatch, When: POST, Then: 403 turnstile_failed",
			verifyErr:   turnstile.ErrActionMismatch,
			wantStatus:  http.StatusForbidden,
			wantBody:    "turnstile_failed",
		},
		{
			name:        "異常_Cloudflare利用不可で503",
			description: "Given: ErrUnavailable, When: POST, Then: 503 turnstile_unavailable",
			verifyErr:   turnstile.ErrUnavailable,
			wantStatus:  http.StatusServiceUnavailable,
			wantBody:    "turnstile_unavailable",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			verifier := &fakeVerifier{
				verifyFn: func(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error) {
					return turnstile.VerifyOutput{}, tt.verifyErr
				},
			}
			h := NewCreateHandlers(nil, verifier, "test.example", "photobook-create", nil)

			body, _ := json.Marshal(createRequest{
				Type:           "memory",
				TurnstileToken: "valid-token",
			})
			req := httptest.NewRequest(http.MethodPost, "/api/photobooks", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			h.CreatePhotobook(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want contain %q", rec.Body.String(), tt.wantBody)
			}
			if verifier.called != 1 {
				t.Errorf("verifier called = %d, want 1", verifier.called)
			}
		})
	}
}
