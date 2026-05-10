// manage_actions_handler_test.go: M-1a Manage safety baseline 用 mutation handler の
// 単体テスト（DB / UseCase 不要の早期 return path に絞る）。
//
// 観点:
//   - PATCH /visibility: public 拒否（reason=public_change_not_allowed）
//   - PATCH /visibility: invalid_payload（JSON 不正 / unknown visibility）
//   - PATCH /sensitive:  invalid_payload
//   - POST /draft-session: 不正 UUID → 404
//   - POST /session-revoke: middleware 経由 session が無いと 401
//
// 成功パス (200 / 409 OCC) は DB-backed integration test の責務（本ファイル外）。
//
// セキュリティ:
//   - raw token / Cookie / Secret を出さない
//   - dummy UUID は 00000000-... を使う
package http_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
)

// helper: ManageHandlers を最小構成で組み立てる（早期 return path のテスト用、UC は nil）。
func newMinimalManageHandlers(t *testing.T) *photobookhttp.ManageHandlers {
	t.Helper()
	// 早期 return path を試す test では UC は nil で OK。chi router 経由で URL param が
	// 取れること、JSON decode / visibility.Parse / public reject などを確認する。
	return photobookhttp.NewManageHandlers(nil, nil, nil, nil, nil, nil)
}

func setupManageActionsRouter(t *testing.T) http.Handler {
	t.Helper()
	h := newMinimalManageHandlers(t)
	r := chi.NewRouter()
	r.Patch("/api/manage/photobooks/{id}/visibility", h.UpdateVisibility)
	r.Patch("/api/manage/photobooks/{id}/sensitive", h.UpdateSensitive)
	r.Post("/api/manage/photobooks/{id}/draft-session", h.IssueDraftSession)
	r.Post("/api/manage/photobooks/{id}/session-revoke", h.RevokeCurrentSession)
	return r
}

// dummyPhotobookUUID は test 用の non-nil UUID。photobook_id.FromUUID は nil UUID を
// 拒否するため、all-zeros 以外の値を使う。
const dummyPhotobookUUID = "11111111-2222-3333-4444-555555555555"

func TestManageHandlers_UpdateVisibility_EarlyReturns(t *testing.T) {
	tests := []struct {
		name        string
		description string
		path        string
		body        string
		wantStatus  int
		wantBody    string
	}{
		{
			name:        "異常_public_は409でreason付き",
			description: "Given: visibility=public, When: PATCH /visibility, Then: 409 manage_precondition_failed (public_change_not_allowed) で UC 未呼出",
			path:        "/api/manage/photobooks/" + dummyPhotobookUUID + "/visibility",
			body:        `{"visibility":"public","expected_version":1}`,
			wantStatus:  http.StatusConflict,
			wantBody:    `"reason":"public_change_not_allowed"`,
		},
		{
			name:        "異常_未知visibility_400",
			description: "Given: visibility=invalid, When: PATCH, Then: 400 invalid_payload（visibility.Parse で reject）",
			path:        "/api/manage/photobooks/" + dummyPhotobookUUID + "/visibility",
			body:        `{"visibility":"unknown","expected_version":1}`,
			wantStatus:  http.StatusBadRequest,
			wantBody:    `"invalid_payload"`,
		},
		{
			name:        "異常_JSON不正_400",
			description: "Given: 不正 JSON, When: PATCH, Then: 400 invalid_payload",
			path:        "/api/manage/photobooks/" + dummyPhotobookUUID + "/visibility",
			body:        `not json`,
			wantStatus:  http.StatusBadRequest,
			wantBody:    `"invalid_payload"`,
		},
		{
			name:        "異常_path_id_不正_404",
			description: "Given: path id が UUID として parse 不能, When: PATCH, Then: 404 not_found",
			path:        "/api/manage/photobooks/not-a-uuid/visibility",
			body:        `{"visibility":"private","expected_version":1}`,
			wantStatus:  http.StatusNotFound,
			wantBody:    `"not_found"`,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			router := setupManageActionsRouter(t)
			req := httptest.NewRequest(http.MethodPatch, tt.path, bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want contain %q", rec.Body.String(), tt.wantBody)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
			}
			if rec.Header().Get("X-Robots-Tag") == "" {
				t.Errorf("X-Robots-Tag missing")
			}
		})
	}
}

func TestManageHandlers_UpdateSensitive_EarlyReturns(t *testing.T) {
	tests := []struct {
		name        string
		description string
		body        string
		wantStatus  int
		wantBody    string
	}{
		{
			name:        "異常_JSON不正_400",
			description: "Given: 不正 JSON, When: PATCH, Then: 400 invalid_payload",
			body:        `bogus`,
			wantStatus:  http.StatusBadRequest,
			wantBody:    `"invalid_payload"`,
		},
		{
			name:        "異常_DisallowUnknownFields_400",
			description: "Given: 余分な field, When: PATCH, Then: 400（DisallowUnknownFields）",
			body:        `{"sensitive":true,"expected_version":1,"extra_field":"x"}`,
			wantStatus:  http.StatusBadRequest,
			wantBody:    `"invalid_payload"`,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			router := setupManageActionsRouter(t)
			req := httptest.NewRequest(
				http.MethodPatch,
				"/api/manage/photobooks/"+dummyPhotobookUUID+"/sensitive",
				bytes.NewReader([]byte(tt.body)),
			)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want contain %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestManageHandlers_IssueDraftSession_EarlyReturns(t *testing.T) {
	tests := []struct {
		name        string
		description string
		path        string
		wantStatus  int
		wantBody    string
	}{
		{
			name:        "異常_path_id_不正_404",
			description: "Given: path id が UUID parse 不能, When: POST /draft-session, Then: 404 not_found（UC 未呼出）",
			path:        "/api/manage/photobooks/bad-uuid/draft-session",
			wantStatus:  http.StatusNotFound,
			wantBody:    `"not_found"`,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			router := setupManageActionsRouter(t)
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want contain %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestManageHandlers_RevokeCurrentSession_NoSessionIn401(t *testing.T) {
	// middleware を経由していないため context に Session が無い → 401 unauthorized。
	// 通常運用では middleware が manage Cookie を検証してから handler に来るため、
	// 本テストは「middleware 抜き path で来た場合の安全側挙動」を確認する。
	router := setupManageActionsRouter(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/manage/photobooks/"+dummyPhotobookUUID+"/session-revoke",
		nil,
	)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"unauthorized"`) {
		t.Errorf("body = %q, want contain unauthorized", rec.Body.String())
	}
}
