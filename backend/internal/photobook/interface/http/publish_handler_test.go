// PublishHandlers の HTTP layer テスト。
//
// 観点:
//   - 200: draft photobook を publish → response に slug / public_url_path /
//     manage_url_path / published_at が入る
//   - 409: 既に published / version 不一致
//   - 400: body 不正
//   - body に raw token / hash 値が含まれない（manage_url_path の path に raw token を含むのは
//     仕様、ただし body 経由 1 回のみ提示で再表示しないことが work-log / UI 側のルール）
//   - Cache-Control: no-store / X-Robots-Tag: noindex,nofollow
package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/infrastructure/session_adapter"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
	photobooktests "vrcpb/backend/internal/photobook/domain/tests"
)

func setupPublishRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	uc := usecase.NewPublishFromDraft(
		pool,
		session_adapter.NewPhotobookTxRepositoryFactory(),
		session_adapter.NewDraftRevokerFactory(),
		usecase.NewMinimalSlugGenerator(),
		nil, // PR36: test 経路は UsageLimit skip
	)
	h := photobookhttp.NewPublishHandlers(uc, "" /* ipHashSalt: PR36 test 経路は salt 空で UsageLimit skip */)
	r := chi.NewRouter()
	r.Post("/api/photobooks/{id}/publish", h.Publish)
	return r
}

func seedDraftDirect(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	pb := photobooktests.NewPhotobookBuilder().Build(t)
	repo := photobookrdb.NewPhotobookRepository(pool)
	if err := repo.CreateDraft(context.Background(), pb); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	return pb.ID().String()
}

func TestPublishHandler(t *testing.T) {
	pool := dbPoolForHandler(t)

	t.Run("正常_draft_を_publish", func(t *testing.T) {
		truncateAllForHandler(t, pool)
		idStr := seedDraftDirect(t, pool)
		router := setupPublishRouter(t, pool)
		body, _ := json.Marshal(map[string]any{"expected_version": 0})
		req := httptest.NewRequest(http.MethodPost, "/api/photobooks/"+idStr+"/publish",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
		respBody, _ := io.ReadAll(rr.Body)
		var payload map[string]any
		if err := json.Unmarshal(respBody, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		for _, k := range []string{"photobook_id", "slug", "public_url_path", "manage_url_path", "published_at"} {
			if _, has := payload[k]; !has {
				t.Errorf("payload missing %q", k)
			}
		}
		// 漏らしてはいけない field
		for _, k := range []string{"manage_url_token_hash", "draft_edit_token", "draft_edit_token_hash", "expected_version"} {
			if _, has := payload[k]; has {
				t.Errorf("payload must not contain %q", k)
			}
		}
		// public_url_path / manage_url_path の form
		if pp, _ := payload["public_url_path"].(string); !strings.HasPrefix(pp, "/p/") {
			t.Errorf("public_url_path = %q want /p/...", pp)
		}
		if mp, _ := payload["manage_url_path"].(string); !strings.HasPrefix(mp, "/manage/token/") {
			t.Errorf("manage_url_path = %q want /manage/token/...", mp)
		}
		// 共通ヘッダ
		if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("Cache-Control=%q", cc)
		}
		if rb := rr.Header().Get("X-Robots-Tag"); rb != "noindex, nofollow" {
			t.Errorf("X-Robots-Tag=%q", rb)
		}
	})

	t.Run("異常_already_published_は_409", func(t *testing.T) {
		truncateAllForHandler(t, pool)
		idStr := seedDraftDirect(t, pool)
		router := setupPublishRouter(t, pool)
		// 1 度目: 200
		body0, _ := json.Marshal(map[string]any{"expected_version": 0})
		req0 := httptest.NewRequest(http.MethodPost, "/api/photobooks/"+idStr+"/publish",
			bytes.NewReader(body0))
		req0.Header.Set("Content-Type", "application/json")
		rr0 := httptest.NewRecorder()
		router.ServeHTTP(rr0, req0)
		if rr0.Code != http.StatusOK {
			t.Fatalf("first publish status=%d", rr0.Code)
		}
		// 2 度目: 409 (status は既に published、version も進んでいる)
		body1, _ := json.Marshal(map[string]any{"expected_version": 1})
		req1 := httptest.NewRequest(http.MethodPost, "/api/photobooks/"+idStr+"/publish",
			bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		rr1 := httptest.NewRecorder()
		router.ServeHTTP(rr1, req1)
		if rr1.Code != http.StatusConflict {
			t.Errorf("second publish status=%d want 409", rr1.Code)
		}
	})

	t.Run("異常_version_mismatch_409", func(t *testing.T) {
		truncateAllForHandler(t, pool)
		idStr := seedDraftDirect(t, pool)
		router := setupPublishRouter(t, pool)
		body, _ := json.Marshal(map[string]any{"expected_version": 999})
		req := httptest.NewRequest(http.MethodPost, "/api/photobooks/"+idStr+"/publish",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusConflict {
			t.Errorf("status=%d want 409", rr.Code)
		}
	})

	t.Run("異常_invalid_uuid_404", func(t *testing.T) {
		truncateAllForHandler(t, pool)
		router := setupPublishRouter(t, pool)
		body, _ := json.Marshal(map[string]any{"expected_version": 0})
		req := httptest.NewRequest(http.MethodPost, "/api/photobooks/not-a-uuid/publish",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("status=%d want 404", rr.Code)
		}
	})

	t.Run("異常_invalid_body_400", func(t *testing.T) {
		truncateAllForHandler(t, pool)
		idStr := seedDraftDirect(t, pool)
		router := setupPublishRouter(t, pool)
		req := httptest.NewRequest(http.MethodPost, "/api/photobooks/"+idStr+"/publish",
			bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status=%d want 400", rr.Code)
		}
	})
}
