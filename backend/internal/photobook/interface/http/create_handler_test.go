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
	"log/slog"
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

// TestClassifyUserAgent は UA 文字列を粗い 4 種に分類する helper の挙動を assert する。
// 個人特定可能な UA 全文を logs に残さないために enum を絞る設計の単体保証。
func TestClassifyUserAgent(t *testing.T) {
	tests := []struct {
		name        string
		description string
		ua          string
		want        string
	}{
		{
			name:        "正常_iPhone_Safari_iOS18.7はios-safari",
			description: "Given: iPhone OS 18_7 + Safari Version/26.4, When: classify, Then: ios-safari",
			ua:          "Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.4 Mobile/15E148 Safari/604.1",
			want:        "ios-safari",
		},
		{
			name:        "正常_iPhone_Chrome_CriOSはios-chromium",
			description: "Given: iPhone CriOS/148, When: classify, Then: ios-chromium",
			ua:          "Mozilla/5.0 (iPhone; CPU iPhone OS 26_4_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/148.0.7778.100 Mobile/15E148 Safari/604.1",
			want:        "ios-chromium",
		},
		{
			name:        "正常_iPad_Safariはios-safari",
			description: "Given: iPad Safari, When: classify, Then: ios-safari",
			ua:          "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			want:        "ios-safari",
		},
		{
			name:        "正常_macOS_Safariはmacos-safari",
			description: "Given: Macintosh + Safari/605, no Chrome, When: classify, Then: macos-safari",
			ua:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
			want:        "macos-safari",
		},
		{
			name:        "正常_macOS_Chromeはother",
			description: "Given: Macintosh + Chrome/120, When: classify, Then: other (macos-safari ではない)",
			ua:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			want:        "other",
		},
		{
			name:        "正常_macOS_Edgeはother",
			description: "Given: Macintosh + Edg/120, When: classify, Then: other",
			ua:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			want:        "other",
		},
		{
			name:        "正常_macOS_Firefoxはother",
			description: "Given: Macintosh + Firefox/, When: classify, Then: other",
			ua:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:120.0) Gecko/20100101 Firefox/120.0",
			want:        "other",
		},
		{
			name:        "正常_Windows_Chromeはother",
			description: "Given: Windows + Chrome, When: classify, Then: other",
			ua:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			want:        "other",
		},
		{
			name:        "正常_Android_Chromeはother",
			description: "Given: Android + Chrome, When: classify, Then: other",
			ua:          "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
			want:        "other",
		},
		{
			name:        "正常_空文字はother",
			description: "Given: \"\" (UA 不在), When: classify, Then: other",
			ua:          "",
			want:        "other",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := classifyUserAgent(tt.ua)
			if got != tt.want {
				t.Errorf("classifyUserAgent(%q) = %q, want %q\n  description: %s", tt.ua, got, tt.want, tt.description)
			}
		})
	}
}

// TestCreatePhotobook_TurnstileFailure_LogsObservability は Turnstile siteverify 失敗時に
// 観測 log（error_codes / hostname / action / ua_class）が JSON で出力され、
// raw token / Cookie / IP / UA 全文が含まれないことを assert する。
//
// 起点: harness/work-logs/2026-05-10_safari-turnstile-403-investigation.md（予定）
// 目的: iPhone Safari 403 の原因を Cloud Run logs で診断するため、Cloudflare 公開 enum の
// error_codes を logs に出すように handler を改修した（本 PR）。
func TestCreatePhotobook_TurnstileFailure_LogsObservability(t *testing.T) {
	tests := []struct {
		name              string
		description       string
		verifyOut         turnstile.VerifyOutput
		verifyErr         error
		ua                string
		wantStatus        int
		wantUAClass       string
		wantErrorCodes    []string
		wantGotHostname   string
		wantGotAction     string
		wantNoLog         bool // ErrUnavailable は log を出さない（503 path）
	}{
		{
			name:        "異常_Safari_timeout-or-duplicateはerror_codes付きでwarn",
			description: "Given: iPhone Safari + ErrVerificationFailed + error_codes=[timeout-or-duplicate], When: POST, Then: 403 + warn log に error_codes / ua_class=ios-safari",
			verifyOut: turnstile.VerifyOutput{
				Success:    false,
				Hostname:   "app.vrc-photobook.com",
				Action:     "photobook-create",
				ErrorCodes: []string{"timeout-or-duplicate"},
			},
			verifyErr:       turnstile.ErrVerificationFailed,
			ua:              "Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.4 Mobile/15E148 Safari/604.1",
			wantStatus:      http.StatusForbidden,
			wantUAClass:     "ios-safari",
			wantErrorCodes:  []string{"timeout-or-duplicate"},
			wantGotHostname: "app.vrc-photobook.com",
			wantGotAction:   "photobook-create",
		},
		{
			name:        "異常_iOS_Chrome_invalid-input-responseでもwarnが出る",
			description: "Given: CriOS + ErrVerificationFailed + error_codes=[invalid-input-response], When: POST, Then: 403 + warn log に ua_class=ios-chromium",
			verifyOut: turnstile.VerifyOutput{
				Success:    false,
				Hostname:   "app.vrc-photobook.com",
				Action:     "photobook-create",
				ErrorCodes: []string{"invalid-input-response"},
			},
			verifyErr:       turnstile.ErrVerificationFailed,
			ua:              "Mozilla/5.0 (iPhone; CPU iPhone OS 26_4_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/148.0.7778.100 Mobile/15E148 Safari/604.1",
			wantStatus:      http.StatusForbidden,
			wantUAClass:     "ios-chromium",
			wantErrorCodes:  []string{"invalid-input-response"},
			wantGotHostname: "app.vrc-photobook.com",
			wantGotAction:   "photobook-create",
		},
		{
			name:        "異常_ErrUnavailableは503でwarnを出さない",
			description: "Given: ErrUnavailable, When: POST, Then: 503 + warn log なし（503 path は既存仕様維持）",
			verifyOut:   turnstile.VerifyOutput{},
			verifyErr:   turnstile.ErrUnavailable,
			ua:          "Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.4 Mobile/15E148 Safari/604.1",
			wantStatus:  http.StatusServiceUnavailable,
			wantNoLog:   true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			verifier := &fakeVerifier{
				verifyFn: func(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error) {
					return tt.verifyOut, tt.verifyErr
				},
			}
			h := NewCreateHandlers(nil, verifier, "app.vrc-photobook.com", "photobook-create", nil)

			// JSON Logger を inject して log 行を捕捉。
			var buf bytes.Buffer
			h.logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

			body, _ := json.Marshal(createRequest{
				Type:           "memory",
				TurnstileToken: "synthetic-test-token",
			})
			req := httptest.NewRequest(http.MethodPost, "/api/photobooks", bytes.NewReader(body))
			req.Header.Set("User-Agent", tt.ua)
			req.Header.Set("Cookie", "session=synthetic-cookie-value-DO-NOT-LOG")
			req.RemoteAddr = "203.0.113.42:54321" // RFC5737 documentation IP
			rec := httptest.NewRecorder()

			h.CreatePhotobook(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%q)", rec.Code, tt.wantStatus, rec.Body.String())
			}

			logOutput := buf.String()

			if tt.wantNoLog {
				// ErrUnavailable は 503 path で warn を出さない。turnstile_verify_failed イベントが
				// log に存在しないことを確認。
				if strings.Contains(logOutput, "turnstile_verify_failed") {
					t.Errorf("ErrUnavailable で turnstile_verify_failed log が出ている: %s", logOutput)
				}
				return
			}

			// log に turnstile_verify_failed イベントが出ていること
			if !strings.Contains(logOutput, "turnstile_verify_failed") {
				t.Fatalf("turnstile_verify_failed log が出ていない: %s", logOutput)
			}

			// log を JSON parse して各 field を assert
			var entry map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(logOutput)), &entry); err != nil {
				t.Fatalf("log JSON parse failed: %v\nraw: %s", err, logOutput)
			}

			if got, _ := entry["event"].(string); got != "turnstile_verify_failed" {
				t.Errorf("event = %q, want turnstile_verify_failed", got)
			}
			if got, _ := entry["route"].(string); got != "/api/photobooks" {
				t.Errorf("route = %q, want /api/photobooks", got)
			}
			if got, _ := entry["ua_class"].(string); got != tt.wantUAClass {
				t.Errorf("ua_class = %q, want %q", got, tt.wantUAClass)
			}
			if got, _ := entry["got_hostname"].(string); got != tt.wantGotHostname {
				t.Errorf("got_hostname = %q, want %q", got, tt.wantGotHostname)
			}
			if got, _ := entry["got_action"].(string); got != tt.wantGotAction {
				t.Errorf("got_action = %q, want %q", got, tt.wantGotAction)
			}

			// error_codes を assert（[]any として deserialize される）
			if codes, ok := entry["error_codes"].([]any); !ok {
				t.Errorf("error_codes 不在 or 型不一致: %v", entry["error_codes"])
			} else {
				if len(codes) != len(tt.wantErrorCodes) {
					t.Errorf("error_codes len = %d, want %d", len(codes), len(tt.wantErrorCodes))
				}
				for i, c := range codes {
					if i >= len(tt.wantErrorCodes) {
						break
					}
					if got, _ := c.(string); got != tt.wantErrorCodes[i] {
						t.Errorf("error_codes[%d] = %q, want %q", i, got, tt.wantErrorCodes[i])
					}
				}
			}

			// 漏洩防止 assert: log に raw token / Cookie / IP / UA 全文が含まれないこと
			forbidden := []string{
				"synthetic-test-token",                  // turnstile token raw
				"synthetic-cookie-value-DO-NOT-LOG",     // Cookie value
				"203.0.113.42",                          // RemoteAddr IP
				"54321",                                 // RemoteAddr port
				"AppleWebKit/605.1.15",                  // UA 全文の特徴的部分
				"Mobile/15E148",                         // UA 全文の特徴的部分
				"CriOS/148.0.7778.100",                  // UA 全文の特徴的部分
				"Version/26.4",                          // UA 全文の特徴的部分
			}
			for _, p := range forbidden {
				if strings.Contains(logOutput, p) {
					t.Errorf("log に禁止パターン %q が含まれている: log=%s", p, logOutput)
				}
			}
		})
	}
}
