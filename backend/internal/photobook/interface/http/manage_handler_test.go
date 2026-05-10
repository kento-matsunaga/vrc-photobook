// ManageHandlers の HTTP layer テスト。
//
// 観点:
//   - 200: photobook 存在 → manage view JSON が返る（manage Cookie middleware 通過後の前提）
//   - 404: photobook_id 不一致 / parse 不能 / 不存在
//   - body に manage_url_token / hash 値が出ないこと
//
// 注意: middleware 部分（Cookie 不在 → 401）は session middleware のテストでカバー済。
// 本テストは「middleware 通過後の handler 動作」を見る。
package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

func setupManageRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	uc := usecase.NewGetManagePhotobook(pool)
	// M-1a: 既存 GetManagePhotobook 単体テスト用 setup。M-1a 追加 mutation はここでは
	// nil で渡す（GetManagePhotobook テストでは実行されない）。M-1a の mutation テストは
	// manage_actions_handler_test.go で個別 setup する。
	h := photobookhttp.NewManageHandlers(uc, nil, nil, nil, nil, nil)
	r := chi.NewRouter()
	r.Get("/api/manage/photobooks/{id}", h.GetManagePhotobook)
	return r
}

func TestGetManagePhotobookHandler(t *testing.T) {
	pool := dbPoolForHandler(t)

	t.Run("正常_existing_id_200", func(t *testing.T) {
		truncateAllForHandler(t, pool)
		// publish 済 photobook を seed して manage 経路でも見えることを確認
		const slugStr = "mh12pp34zz56gh78"
		seedPublishedWithPhoto(t, pool, slugStr, false, "unlisted")
		// pb の id を取り直す
		row := pool.QueryRow(context.Background(), "SELECT id FROM photobooks WHERE public_url_slug=$1", slugStr)
		var id [16]byte
		if err := row.Scan(&id); err != nil {
			t.Fatalf("Scan: %v", err)
		}
		idStr := uuidString(id)
		router := setupManageRouter(t, pool)
		req := httptest.NewRequest(http.MethodGet, "/api/manage/photobooks/"+idStr, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
		body, _ := io.ReadAll(rr.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// 必須フィールド
		for _, k := range []string{"photobook_id", "title", "status", "manage_url_token_version", "available_image_count"} {
			if _, has := payload[k]; !has {
				t.Errorf("payload missing %q", k)
			}
		}
		// 漏らしてはいけないフィールド
		for _, k := range []string{"manage_url_token", "manage_url_token_hash", "draft_edit_token", "draft_edit_token_hash"} {
			if _, has := payload[k]; has {
				t.Errorf("payload must not contain %q", k)
			}
		}
		if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("Cache-Control=%q", cc)
		}
		if rb := rr.Header().Get("X-Robots-Tag"); rb != "noindex, nofollow" {
			t.Errorf("X-Robots-Tag=%q", rb)
		}
	})

	t.Run("異常_invalid_uuid_404", func(t *testing.T) {
		truncateAllForHandler(t, pool)
		router := setupManageRouter(t, pool)
		req := httptest.NewRequest(http.MethodGet, "/api/manage/photobooks/not-a-uuid", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rr.Code)
		}
	})

	t.Run("異常_unknown_id_404", func(t *testing.T) {
		truncateAllForHandler(t, pool)
		router := setupManageRouter(t, pool)
		req := httptest.NewRequest(http.MethodGet, "/api/manage/photobooks/00000000-0000-0000-0000-000000000000", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
		body, _ := io.ReadAll(rr.Body)
		if !strings.Contains(string(body), "not_found") {
			t.Errorf("body missing not_found: %s", string(body))
		}
	})
}

// uuidString は [16]byte → 8-4-4-4-12 形式 string。
func uuidString(b [16]byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 36)
	idx := 0
	for i, x := range b {
		out[idx] = hex[x>>4]
		out[idx+1] = hex[x&0x0F]
		idx += 2
		if i == 3 || i == 5 || i == 7 || i == 9 {
			out[idx] = '-'
			idx++
		}
	}
	return string(out)
}
