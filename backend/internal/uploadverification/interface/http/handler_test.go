// upload-verifications HTTP handler test。
//
// 実 DB + Turnstile fake で 4 つの主要パスを検証:
//   - 正常: 201 + response body
//   - Turnstile failure: 403
//   - Cloudflare 障害: 503
//   - draft session photobook_id 不一致: 401
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend test ./internal/uploadverification/interface/http/...
package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	authmiddleware "vrcpb/backend/internal/auth/session/middleware"
	"vrcpb/backend/internal/auth/session/sessionintegration"
	photobookdomaintests "vrcpb/backend/internal/photobook/domain/tests"
	photobookmarshaller "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/marshaller"
	photobooksqlc "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/sqlcgen"
	"vrcpb/backend/internal/uploadverification/infrastructure/turnstile"
	uvhttp "vrcpb/backend/internal/uploadverification/interface/http"
	uvwireup "vrcpb/backend/internal/uploadverification/wireup"
	uvtests "vrcpb/backend/internal/uploadverification/tests"

	authphotobookid "vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
)

func dbPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(),
		"TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}

// buildRouter は handler + draft session middleware を組み立てた最小 chi router を返す。
func buildRouter(t *testing.T, pool *pgxpool.Pool, fake *uvtests.FakeTurnstile) http.Handler {
	t.Helper()
	verifier := fake
	handlers := uvwireup.BuildHandlers(pool, verifier, uvwireup.Config{
		Hostname: "app.vrc-photobook.com",
		Action:   "upload",
	}, uvhttp.SystemClock{})

	validator := sessionintegration.NewSessionValidator(pool)

	r := chi.NewRouter()
	r.Route("/api/photobooks/{id}/upload-verifications", func(sub chi.Router) {
		sub.Use(authmiddleware.RequireDraftSession(validator, func(req *http.Request) (authphotobookid.PhotobookID, error) {
			raw := chi.URLParam(req, "id")
			u, err := uuid.Parse(raw)
			if err != nil {
				return authphotobookid.PhotobookID{}, err
			}
			return authphotobookid.FromUUID(u)
		}))
		sub.Post("/", handlers.IssueUploadVerification)
	})
	return r
}

// seedPhotobookAndDraftSession は draft photobook を 1 つ作り、draft session を発行して
// raw token を返す。
func seedPhotobookAndDraftSession(t *testing.T, pool *pgxpool.Pool) (photobookID uuid.UUID, rawSessionToken string) {
	t.Helper()
	pb := photobookdomaintests.NewPhotobookBuilder().Build(t)
	params, err := photobookmarshaller.ToCreateParams(pb)
	if err != nil {
		t.Fatalf("ToCreateParams: %v", err)
	}
	if err := photobooksqlc.New(pool).CreateDraftPhotobook(context.Background(), params); err != nil {
		t.Fatalf("CreateDraftPhotobook: %v", err)
	}
	now := time.Now().UTC()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer tx.Rollback(context.Background())
	tok, _, err := sessionintegration.IssueDraftWithTx(context.Background(), tx, pb.ID().UUID(), now, now.Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("IssueDraftWithTx: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return pb.ID().UUID(), tok.Encode()
}

// withDraftCookie は cookie name = vrcpb_draft_<photobook_id> として draft Cookie を付与する。
func withDraftCookie(req *http.Request, photobookID uuid.UUID, rawToken string) {
	req.AddCookie(&http.Cookie{
		Name:  "vrcpb_draft_" + photobookID.String(),
		Value: rawToken,
	})
}

func TestIssueUploadVerification_Success(t *testing.T) {
	pool := dbPool(t)
	pid, rawTok := seedPhotobookAndDraftSession(t, pool)
	router := buildRouter(t, pool, &uvtests.FakeTurnstile{})

	body, _ := json.Marshal(map[string]string{"turnstile_token": "valid"})
	req := httptest.NewRequest(http.MethodPost,
		"/api/photobooks/"+pid.String()+"/upload-verifications/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withDraftCookie(req, pid, rawTok)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d want 201, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Token string `json:"upload_verification_token"`
		Exp   string `json:"expires_at"`
		AC    int    `json:"allowed_intent_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Token) != 43 {
		t.Errorf("token length = %d want 43", len(resp.Token))
	}
	if resp.AC != 20 {
		t.Errorf("allowed_intent_count = %d want 20", resp.AC)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control mismatch")
	}
}

func TestIssueUploadVerification_TurnstileFailure(t *testing.T) {
	pool := dbPool(t)
	pid, rawTok := seedPhotobookAndDraftSession(t, pool)
	fake := &uvtests.FakeTurnstile{
		VerifyFn: func(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error) {
			return turnstile.VerifyOutput{Success: false}, turnstile.ErrVerificationFailed
		},
	}
	router := buildRouter(t, pool, fake)

	body, _ := json.Marshal(map[string]string{"turnstile_token": "bad"})
	req := httptest.NewRequest(http.MethodPost,
		"/api/photobooks/"+pid.String()+"/upload-verifications/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withDraftCookie(req, pid, rawTok)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d want 403, body=%s", rec.Code, rec.Body.String())
	}
}

func TestIssueUploadVerification_TurnstileUnavailable(t *testing.T) {
	pool := dbPool(t)
	pid, rawTok := seedPhotobookAndDraftSession(t, pool)
	fake := &uvtests.FakeTurnstile{
		VerifyFn: func(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error) {
			return turnstile.VerifyOutput{}, turnstile.ErrUnavailable
		},
	}
	router := buildRouter(t, pool, fake)

	body, _ := json.Marshal(map[string]string{"turnstile_token": "x"})
	req := httptest.NewRequest(http.MethodPost,
		"/api/photobooks/"+pid.String()+"/upload-verifications/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withDraftCookie(req, pid, rawTok)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d want 503", rec.Code)
	}
}

func TestIssueUploadVerification_DraftSessionMissing(t *testing.T) {
	pool := dbPool(t)
	pid, _ := seedPhotobookAndDraftSession(t, pool)
	router := buildRouter(t, pool, &uvtests.FakeTurnstile{})

	body, _ := json.Marshal(map[string]string{"turnstile_token": "x"})
	req := httptest.NewRequest(http.MethodPost,
		"/api/photobooks/"+pid.String()+"/upload-verifications/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Cookie 無し
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d want 401", rec.Code)
	}
}

func TestIssueUploadVerification_EmptyTurnstileToken(t *testing.T) {
	pool := dbPool(t)
	pid, rawTok := seedPhotobookAndDraftSession(t, pool)
	router := buildRouter(t, pool, &uvtests.FakeTurnstile{})

	body, _ := json.Marshal(map[string]string{"turnstile_token": ""})
	req := httptest.NewRequest(http.MethodPost,
		"/api/photobooks/"+pid.String()+"/upload-verifications/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withDraftCookie(req, pid, rawTok)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d want 400", rec.Code)
	}
}

func TestIssueUploadVerification_MissingTurnstileTokenField(t *testing.T) {
	pool := dbPool(t)
	pid, rawTok := seedPhotobookAndDraftSession(t, pool)
	router := buildRouter(t, pool, &uvtests.FakeTurnstile{})

	// turnstile_token フィールド欠落 → JSON decode は成功するが空文字列扱いで 400
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost,
		"/api/photobooks/"+pid.String()+"/upload-verifications/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withDraftCookie(req, pid, rawTok)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d want 400", rec.Code)
	}
}

func TestIssueUploadVerification_PhotobookMismatch(t *testing.T) {
	pool := dbPool(t)
	pid, rawTok := seedPhotobookAndDraftSession(t, pool)
	other, _ := seedPhotobookAndDraftSession(t, pool)
	router := buildRouter(t, pool, &uvtests.FakeTurnstile{})

	// pid の Cookie で other の URL にアクセス → middleware が 401（Cookie name と URL pid が不一致のため）
	body, _ := json.Marshal(map[string]string{"turnstile_token": "x"})
	req := httptest.NewRequest(http.MethodPost,
		"/api/photobooks/"+other.String()+"/upload-verifications/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withDraftCookie(req, pid, rawTok)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d want 401", rec.Code)
	}
}
