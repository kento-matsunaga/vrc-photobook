// handler test は、本物の token 検証フローを通すために実 PostgreSQL を使う。
//
// PR9c では fake repository / fake issuer を handler に注入する経路は **作らない**
// （interface 越しに UseCase を渡す形になっており、fake 注入は技術的に可能だが、
//  本物 token 検証経路の確認のほうが本 PR の主目的）。
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//	go -C backend test ./internal/photobook/interface/http/...
package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/photobook/domain/vo/opening_style"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_layout"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_type"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/infrastructure/session_adapter"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

func dbPoolForHandler(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set; skipping handler test (set to run)")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}

func buildHandlers(pool *pgxpool.Pool) *photobookhttp.Handlers {
	repo := photobookrdb.NewPhotobookRepository(pool)
	di := session_adapter.NewDraftIssuer(pool)
	mi := session_adapter.NewManageIssuer(pool)
	de := usecase.NewExchangeDraftTokenForSession(repo, di)
	me := usecase.NewExchangeManageTokenForSession(repo, mi)
	return photobookhttp.NewHandlers(de, me, 7*24*time.Hour, photobookhttp.SystemClock{})
}

// createDraftAndPublish は draft + published の Photobook を 2 件用意し、
// (draft の raw token, published の raw manage token) を返す。
func createDraftAndPublish(t *testing.T, pool *pgxpool.Pool) (string, string) {
	t.Helper()
	ctx := context.Background()
	repo := photobookrdb.NewPhotobookRepository(pool)
	now := time.Now().UTC()

	in := usecase.CreateDraftPhotobookInput{
		Type:               photobook_type.Memory(),
		Title:              "Test",
		Layout:             photobook_layout.Simple(),
		OpeningStyle:       opening_style.Light(),
		Visibility:         visibility.Unlisted(),
		CreatorDisplayName: "Tester",
		RightsAgreed:       true,
		Now:                now,
		DraftTTL:           24 * time.Hour,
	}
	// draft 用 photobook
	draftOut, err := usecase.NewCreateDraftPhotobook(repo).Execute(ctx, in)
	if err != nil {
		t.Fatalf("CreateDraft draft target: %v", err)
	}
	// published 用 photobook（別 instance）
	pubOut, err := usecase.NewCreateDraftPhotobook(repo).Execute(ctx, in)
	if err != nil {
		t.Fatalf("CreateDraft publish target: %v", err)
	}
	publish := usecase.NewPublishFromDraft(
		pool,
		session_adapter.NewPhotobookTxRepositoryFactory(),
		session_adapter.NewDraftRevokerFactory(),
		usecase.NewMinimalSlugGenerator(),
		nil, // PR36: test 経路は UsageLimit skip
		nil, // M-2: OGP pending ensurer (test 経路は OGP 同期 skip)
		nil, // M-2: OGP sync generator (test 経路は OGP 同期 skip)
		nil, // logger (nil → slog.Default())
	)
	pub, err := publish.Execute(ctx, usecase.PublishFromDraftInput{
		PhotobookID:     pubOut.Photobook.ID(),
		ExpectedVersion: pubOut.Photobook.Version(),
		RightsAgreed:    true, // 2026-05-03 STOP α P0 v2: publish 時同意必須
		Now:             now,
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	return draftOut.RawDraftToken.Encode(), pub.RawManageToken.Encode()
}

// === Draft session exchange ===

func TestDraftSessionExchange(t *testing.T) {
	pool := dbPoolForHandler(t)
	h := buildHandlers(pool)
	draftRaw, manageRaw := createDraftAndPublish(t, pool)

	cases := []struct {
		name         string
		description  string
		body         string
		wantStatus   int
		wantBodySub  string // 成功は session_token を含む / 失敗は固定文言
		wantNotInBody []string
	}{
		{
			name:        "正常_有効draft_token",
			description: "Given: 有効な draft_edit_token, When: POST, Then: 200 OK + session_token / photobook_id / expires_at",
			body:        `{"draft_edit_token":"` + draftRaw + `"}`,
			wantStatus:  http.StatusOK,
			wantBodySub: `"session_token":`,
			// raw draft token は response body に出ない
			wantNotInBody: []string{`"draft_edit_token"`, draftRaw},
		},
		{
			name:        "異常_未知のtoken",
			description: "Given: 別 generate された draft token（DB に無い）, When: POST, Then: 401 unauthorized",
			body:        `{"draft_edit_token":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`,
			wantStatus:  http.StatusUnauthorized,
			wantBodySub: `"unauthorized"`,
		},
		{
			name:        "異常_published用のmanage_token",
			description: "Given: published photobook の manage token を draft 経路に投げる, When: POST, Then: 401",
			body:        `{"draft_edit_token":"` + manageRaw + `"}`,
			wantStatus:  http.StatusUnauthorized,
			wantBodySub: `"unauthorized"`,
		},
		{
			name:        "異常_token形式不正",
			description: "Given: 短すぎる token, When: POST, Then: 401 unauthorized",
			body:        `{"draft_edit_token":"short"}`,
			wantStatus:  http.StatusUnauthorized,
			wantBodySub: `"unauthorized"`,
		},
		{
			name:        "異常_token空文字",
			description: "Given: 空 token, When: POST, Then: 401 unauthorized",
			body:        `{"draft_edit_token":""}`,
			wantStatus:  http.StatusUnauthorized,
			wantBodySub: `"unauthorized"`,
		},
		{
			name:        "異常_空body",
			description: "Given: 空 body, When: POST, Then: 400 bad_request",
			body:        ``,
			wantStatus:  http.StatusBadRequest,
			wantBodySub: `"bad_request"`,
		},
		{
			name:        "異常_壊れたJSON",
			description: "Given: 壊れた JSON, When: POST, Then: 400 bad_request",
			body:        `{not json`,
			wantStatus:  http.StatusBadRequest,
			wantBodySub: `"bad_request"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/draft-session-exchange", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.DraftSessionExchange(rec, req)

			if got := rec.Code; got != tc.wantStatus {
				t.Errorf("status = %d, want %d (body=%s)", got, tc.wantStatus, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, tc.wantBodySub) {
				t.Errorf("body %q must contain %q", body, tc.wantBodySub)
			}
			for _, ng := range tc.wantNotInBody {
				if strings.Contains(body, ng) {
					t.Errorf("body must NOT contain %q (leak): %s", ng, body)
				}
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control must be no-store, got %q", rec.Header().Get("Cache-Control"))
			}
			if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
				t.Errorf("Content-Type should be application/json, got %q", got)
			}
			// Set-Cookie が出ていないこと
			if v := rec.Header().Values("Set-Cookie"); len(v) > 0 {
				t.Errorf("Set-Cookie must not be sent, got %v", v)
			}

			if tc.wantStatus == http.StatusOK {
				// 成功時は session_token / photobook_id / expires_at が JSON で返る
				var resp struct {
					SessionToken string    `json:"session_token"`
					PhotobookID  string    `json:"photobook_id"`
					ExpiresAt    time.Time `json:"expires_at"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Unmarshal: %v", err)
				}
				if resp.SessionToken == "" {
					t.Errorf("session_token must not be empty")
				}
				if resp.SessionToken == draftRaw {
					t.Errorf("session_token must NOT equal draft_edit_token (raw leak)")
				}
				if resp.PhotobookID == "" {
					t.Errorf("photobook_id must not be empty")
				}
				if resp.ExpiresAt.IsZero() {
					t.Errorf("expires_at must not be zero")
				}
			}
		})
	}
}

// === Manage session exchange ===

func TestManageSessionExchange(t *testing.T) {
	pool := dbPoolForHandler(t)
	h := buildHandlers(pool)
	draftRaw, manageRaw := createDraftAndPublish(t, pool)

	cases := []struct {
		name         string
		description  string
		body         string
		wantStatus   int
		wantBodySub  string
		wantNotInBody []string
	}{
		{
			name:          "正常_有効manage_token",
			description:   "Given: 有効な manage_url_token, When: POST, Then: 200 + session_token + token_version_at_issue=0",
			body:          `{"manage_url_token":"` + manageRaw + `"}`,
			wantStatus:    http.StatusOK,
			wantBodySub:   `"token_version_at_issue":0`,
			wantNotInBody: []string{`"manage_url_token"`, manageRaw},
		},
		{
			name:        "異常_未知のtoken",
			description: "Given: 別 token, When: POST, Then: 401",
			body:        `{"manage_url_token":"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"}`,
			wantStatus:  http.StatusUnauthorized,
			wantBodySub: `"unauthorized"`,
		},
		{
			name:        "異常_draft_tokenを送る",
			description: "Given: draft 用 token を manage 経路に投げる, When: POST, Then: 401",
			body:        `{"manage_url_token":"` + draftRaw + `"}`,
			wantStatus:  http.StatusUnauthorized,
			wantBodySub: `"unauthorized"`,
		},
		{
			name:        "異常_token形式不正",
			description: "Given: 短すぎる token, When: POST, Then: 401",
			body:        `{"manage_url_token":"short"}`,
			wantStatus:  http.StatusUnauthorized,
			wantBodySub: `"unauthorized"`,
		},
		{
			name:        "異常_空body",
			description: "Given: 空 body, When: POST, Then: 400",
			body:        ``,
			wantStatus:  http.StatusBadRequest,
			wantBodySub: `"bad_request"`,
		},
		{
			name:        "異常_壊れたJSON",
			description: "Given: 壊れた JSON, When: POST, Then: 400",
			body:        `{not json`,
			wantStatus:  http.StatusBadRequest,
			wantBodySub: `"bad_request"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/manage-session-exchange", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ManageSessionExchange(rec, req)

			if got := rec.Code; got != tc.wantStatus {
				t.Errorf("status = %d, want %d (body=%s)", got, tc.wantStatus, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, tc.wantBodySub) {
				t.Errorf("body %q must contain %q", body, tc.wantBodySub)
			}
			for _, ng := range tc.wantNotInBody {
				if strings.Contains(body, ng) {
					t.Errorf("body must NOT contain %q (leak): %s", ng, body)
				}
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control must be no-store")
			}
			if v := rec.Header().Values("Set-Cookie"); len(v) > 0 {
				t.Errorf("Set-Cookie must not be sent, got %v", v)
			}

			if tc.wantStatus == http.StatusOK {
				var resp struct {
					SessionToken        string    `json:"session_token"`
					PhotobookID         string    `json:"photobook_id"`
					ExpiresAt           time.Time `json:"expires_at"`
					TokenVersionAtIssue int       `json:"token_version_at_issue"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Unmarshal: %v", err)
				}
				if resp.SessionToken == "" {
					t.Errorf("session_token must not be empty")
				}
				if resp.SessionToken == manageRaw {
					t.Errorf("session_token must NOT equal manage_url_token (raw leak)")
				}
				if resp.TokenVersionAtIssue != 0 {
					t.Errorf("token_version_at_issue = %d want 0", resp.TokenVersionAtIssue)
				}
			}
		})
	}
}

// === router 経由の確認（NewRouter 経由で endpoint が登録されること）===
//
// router は internal/http にあるので import の関係で本テストは別ファイルでの試行を割愛。
// 代わりに handler 単体（DraftSessionExchange / ManageSessionExchange）を直接呼ぶことで
// 同等の挙動を確認する。

// === ボディ投入抑止チェック ===
//
// Body が nil（GET 等で発生）の場合に 400 になることを確認。
func TestDraftSessionExchange_NilBody(t *testing.T) {
	pool := dbPoolForHandler(t)
	h := buildHandlers(pool)

	t.Run("異常_nil_body", func(t *testing.T) {
		// Given: body 無し, When: POST, Then: 400
		req := httptest.NewRequest(http.MethodPost, "/api/auth/draft-session-exchange", nil)
		rec := httptest.NewRecorder()
		h.DraftSessionExchange(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d want 400", rec.Code)
		}
	})
}

// helper: io.ReadAll の結果を error と一緒に返す（body 検証用）
func mustReadBody(t *testing.T, r io.Reader) string {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(b)
}

var _ = bytes.NewReader
var _ = errors.New
var _ = mustReadBody
