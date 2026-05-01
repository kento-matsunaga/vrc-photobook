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

	"vrcpb/backend/internal/photobook/internal/usecase"
	usecasetests "vrcpb/backend/internal/photobook/internal/usecase/tests"
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

// TestCreatePhotobook_Success_OptionalFields は title / creator_display_name が任意項目
// であり、空文字 / 境界長で submit したとき HTTP 201 が返ることを検証する。
//
// 起点: docs/plan/m2-create-entry-optional-fields-fix-plan.md §1
// 既存の TestCreatePhotobook_LengthValidation（過長 → 400）と対をなし、handler が
// optional fields を正しく UseCase に渡し domain も空文字を許容することを担保する。
func TestCreatePhotobook_Success_OptionalFields(t *testing.T) {
	tests := []struct {
		name        string
		description string
		title       string
		creator     string
	}{
		{
			name:        "正常_title空_creator空でも201",
			description: "Given: title='' / creator='' (任意項目を未入力), When: POST + Turnstile siteverify 成功, Then: 201 Created（domain validation 通過）",
			title:       "",
			creator:     "",
		},
		{
			name:        "正常_title空のみ_201",
			description: "Given: title='' / creator=値あり, When: POST, Then: 201 Created",
			title:       "",
			creator:     "Tester",
		},
		{
			name:        "正常_creator空のみ_201",
			description: "Given: title=値あり / creator='', When: POST, Then: 201 Created",
			title:       "smoke title",
			creator:     "",
		},
		{
			name:        "正常_境界_title80_creator50で201",
			description: "Given: title=80 文字 / creator=50 文字 (それぞれ上限ちょうど), When: POST, Then: 201 Created",
			title:       strings.Repeat("a", maxTitleLen),
			creator:     strings.Repeat("b", maxCreatorDisplayNameLen),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			verifier := &fakeVerifier{}
			repo := usecasetests.NewFakePhotobookRepository()
			uc := usecase.NewCreateDraftPhotobook(repo)
			h := NewCreateHandlers(uc, verifier, "test.example", "photobook-create", nil)

			body, _ := json.Marshal(createRequest{
				Type:               "memory",
				Title:              tt.title,
				CreatorDisplayName: tt.creator,
				TurnstileToken:     "valid-token",
			})
			req := httptest.NewRequest(http.MethodPost, "/api/photobooks", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			h.CreatePhotobook(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusCreated, rec.Body.String())
			}
			if repo.CreateCalls != 1 {
				t.Errorf("repo.CreateCalls = %d, want 1", repo.CreateCalls)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
			}
			// raw draft_edit_token は response body にだけ 1 度返る（呼び出し側が即 replace
			// する設計）。本 test では token 形式のみ確認、値は assert しない。
			var resp createResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.PhotobookID == "" || resp.DraftEditToken == "" || resp.DraftEditURLPath == "" {
				t.Errorf("response missing required fields (photobook_id / draft_edit_token / draft_edit_url_path)")
			}
		})
	}
}
