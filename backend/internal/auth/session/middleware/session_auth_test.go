package middleware_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/auth/session/cookie"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
	"vrcpb/backend/internal/auth/session/internal/usecase"
	"vrcpb/backend/internal/auth/session/internal/usecase/tests"
	"vrcpb/backend/internal/auth/session/middleware"
)

// fixedExtractor は固定 photobook_id を返す PhotobookIDExtractor。
func fixedExtractor(pid photobook_id.PhotobookID) middleware.PhotobookIDExtractor {
	return func(r *http.Request) (photobook_id.PhotobookID, error) {
		return pid, nil
	}
}

// echoSessionHandler は SessionFromContext で取り出した Session を文字列で返す（テスト用）。
//
// 本ハンドラはテスト内 httptest.NewServer でのみ使う。本番 router には接続しない。
func echoSessionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := middleware.SessionFromContext(r.Context())
		if !ok {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok:" + s.ID().String()))
	})
}

func setupDraftSession(t *testing.T, repo *tests.FakeRepository, pid photobook_id.PhotobookID, now time.Time, ttl time.Duration) session_token.SessionToken {
	t.Helper()
	out, err := usecase.NewIssueDraftSession(repo).Execute(context.Background(), usecase.IssueDraftSessionInput{
		PhotobookID: pid,
		Now:         now,
		ExpiresAt:   now.Add(ttl),
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return out.RawToken
}

func TestRequireDraftSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	pid, _ := photobook_id.FromUUID(uuid.New())
	policy := cookie.Policy{}

	cases := []struct {
		name        string
		description string
		prepare     func(t *testing.T) (*tests.FakeRepository, *http.Request)
		wantStatus  int
	}{
		{
			name:        "正常_有効cookie",
			description: "Given: 発行直後の cookie を持つ request, When: middleware 通過, Then: 200 + body にセッションID",
			prepare: func(t *testing.T) (*tests.FakeRepository, *http.Request) {
				repo := tests.NewFakeRepository()
				repo.Now = func() time.Time { return now }
				raw := setupDraftSession(t, repo, pid, now, 24*time.Hour)

				req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
				c, err := policy.BuildIssue(session_type.Draft(), pid, raw, now, now.Add(24*time.Hour))
				if err != nil {
					t.Fatalf("BuildIssue: %v", err)
				}
				req.AddCookie(c)
				return repo, req
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "異常_cookie無し",
			description: "Given: cookie 無し, When: middleware, Then: 401",
			prepare: func(t *testing.T) (*tests.FakeRepository, *http.Request) {
				repo := tests.NewFakeRepository()
				repo.Now = func() time.Time { return now }
				return repo, httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:        "異常_不正フォーマットtoken",
			description: "Given: 43 文字でない token を持つ cookie, When: middleware, Then: 401",
			prepare: func(t *testing.T) (*tests.FakeRepository, *http.Request) {
				repo := tests.NewFakeRepository()
				req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
				req.AddCookie(&http.Cookie{
					Name:  cookie.Name(session_type.Draft(), pid),
					Value: "tooshort",
				})
				return repo, req
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:        "異常_未知のtoken",
			description: "Given: 別 photobook の cookie 名で正しい長さの token, When: middleware, Then: 401",
			prepare: func(t *testing.T) (*tests.FakeRepository, *http.Request) {
				repo := tests.NewFakeRepository()
				repo.Now = func() time.Time { return now }
				stranger, _ := session_token.Generate()
				req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
				c, err := policy.BuildIssue(session_type.Draft(), pid, stranger, now, now.Add(24*time.Hour))
				if err != nil {
					t.Fatalf("BuildIssue: %v", err)
				}
				req.AddCookie(c)
				return repo, req
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:        "異常_revokedされたsession",
			description: "Given: 発行後に RevokeAllDrafts, When: middleware, Then: 401",
			prepare: func(t *testing.T) (*tests.FakeRepository, *http.Request) {
				repo := tests.NewFakeRepository()
				repo.Now = func() time.Time { return now }
				raw := setupDraftSession(t, repo, pid, now, 24*time.Hour)
				if _, err := repo.RevokeAllDrafts(context.Background(), pid); err != nil {
					t.Fatalf("revoke: %v", err)
				}
				req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
				c, err := policy.BuildIssue(session_type.Draft(), pid, raw, now, now.Add(24*time.Hour))
				if err != nil {
					t.Fatalf("BuildIssue: %v", err)
				}
				req.AddCookie(c)
				return repo, req
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:        "異常_draft_manage取り違え",
			description: "Given: draft session の cookie を manage 名で送る, When: RequireDraftSession, Then: 401（cookie 名不一致で取り出せない）",
			prepare: func(t *testing.T) (*tests.FakeRepository, *http.Request) {
				repo := tests.NewFakeRepository()
				repo.Now = func() time.Time { return now }
				raw := setupDraftSession(t, repo, pid, now, 24*time.Hour)
				req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
				// manage 名で発行（draft 用 middleware は draft 名を探すので Cookie 取り出しに失敗）
				c, err := policy.BuildIssue(session_type.Manage(), pid, raw, now, now.Add(24*time.Hour))
				if err != nil {
					t.Fatalf("BuildIssue: %v", err)
				}
				req.AddCookie(c)
				return repo, req
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:        "異常_extractor失敗",
			description: "Given: extractor がエラー, When: middleware, Then: 401",
			prepare: func(t *testing.T) (*tests.FakeRepository, *http.Request) {
				repo := tests.NewFakeRepository()
				return repo, httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
			},
			wantStatus: http.StatusUnauthorized,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, req := tc.prepare(t)
			validator := usecase.NewValidateSession(repo)

			extractor := fixedExtractor(pid)
			if tc.name == "異常_extractor失敗" {
				extractor = func(r *http.Request) (photobook_id.PhotobookID, error) {
					return photobook_id.PhotobookID{}, errors.New("no pid")
				}
			}

			h := middleware.RequireDraftSession(validator, extractor)(echoSessionHandler())
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got := rec.Code; got != tc.wantStatus {
				t.Errorf("status = %d, want %d", got, tc.wantStatus)
			}
			body, _ := io.ReadAll(rec.Body)
			if tc.wantStatus == http.StatusUnauthorized {
				// 401 body は固定文言で、原因詳細を含まない
				if !strings.Contains(string(body), `"unauthorized"`) {
					t.Errorf("body = %q, want contains \"unauthorized\"", body)
				}
				if rec.Header().Get("Cache-Control") != "no-store" {
					t.Errorf("Cache-Control header missing")
				}
			}
			// Cookie 値が body / header に出ていないことを確認（簡易）
			if c, _ := req.Cookie(cookie.Name(session_type.Draft(), pid)); c != nil {
				if strings.Contains(string(body), c.Value) {
					t.Errorf("Cookie value leaked into response body")
				}
				for _, vs := range rec.Header() {
					for _, v := range vs {
						if strings.Contains(v, c.Value) {
							t.Errorf("Cookie value leaked into response header")
						}
					}
				}
			}
		})
	}
}

func TestRequireManageSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	pid, _ := photobook_id.FromUUID(uuid.New())
	policy := cookie.Policy{}

	t.Run("正常_manage_cookie", func(t *testing.T) {
		// Given: manage session 発行, When: RequireManageSession, Then: 200
		repo := tests.NewFakeRepository()
		repo.Now = func() time.Time { return now }

tv, err := token_version_at_issue.New(1)
		if err != nil {
			t.Fatalf("token_version: %v", err)
		}
		out, err := usecase.NewIssueManageSession(repo).Execute(context.Background(), usecase.IssueManageSessionInput{
			PhotobookID:         pid,
			TokenVersionAtIssue: tv,
			Now:                 now,
			ExpiresAt:           now.Add(7 * 24 * time.Hour),
		})
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		c, err := policy.BuildIssue(session_type.Manage(), pid, out.RawToken, now, now.Add(7*24*time.Hour))
		if err != nil {
			t.Fatalf("BuildIssue: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
		req.AddCookie(c)

		validator := usecase.NewValidateSession(repo)
		h := middleware.RequireManageSession(validator, fixedExtractor(pid))(echoSessionHandler())
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("異常_manage_validatorに_draft_cookie", func(t *testing.T) {
		// Given: draft session 発行 + draft cookie 名で送信, When: RequireManageSession, Then: 401
		repo := tests.NewFakeRepository()
		repo.Now = func() time.Time { return now }
		raw := setupDraftSession(t, repo, pid, now, 24*time.Hour)
		c, err := policy.BuildIssue(session_type.Draft(), pid, raw, now, now.Add(24*time.Hour))
		if err != nil {
			t.Fatalf("BuildIssue: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
		req.AddCookie(c)

		validator := usecase.NewValidateSession(repo)
		h := middleware.RequireManageSession(validator, fixedExtractor(pid))(echoSessionHandler())
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestSessionFromContext_NotPresent(t *testing.T) {
	t.Parallel()
	// Given: middleware を通っていない context, When: SessionFromContext, Then: ok=false
	if _, ok := middleware.SessionFromContext(context.Background()); ok {
		t.Errorf("must be false when middleware did not set")
	}
}

