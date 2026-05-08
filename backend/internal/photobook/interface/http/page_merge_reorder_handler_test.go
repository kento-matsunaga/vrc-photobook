// STOP P-3: Phase A 補強 2 endpoint (MergePages / ReorderPages) HTTP layer test。
//
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §7.3
//
// 観点:
//   - 200 正常系 (B 方式: 更新後 EditView を返す)
//   - 400 invalid JSON / 不正 UUID / invalid_reorder_assignments / 空 assignments
//   - 404 photobookId / pageId / targetPageId 不存在
//   - 409 reason mapping: version_conflict / merge_into_self / cannot_remove_last_page
//   - response shape: B 方式 (EditView)
//   - raw token / Cookie / Secret が response に出ない (defensive)
//
// 注意: middleware (draft session) は別 test でカバー済の前提、本 test は handler 直接呼出。
package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	imageuploadtests "vrcpb/backend/internal/imageupload/tests"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// setupPageMergeReorderRouter は本 test 専用 router (handler 直接呼出、middleware なし)。
func setupPageMergeReorderRouter(pool *pgxpool.Pool, fakeR2 r2.Client) http.Handler {
	h := photobookhttp.NewEditHandlers(
		usecase.NewGetEditView(pool, fakeR2),
		nil, nil, nil, // updatePhotoCaption / bulkReorder / updateSettings = nil
		nil, nil, nil, // addPage / removePage / removePhoto = nil
		nil, nil, // setCover / clearCover = nil
		nil,                                  // attachAvailableImages = nil
		nil, nil, nil, // STOP P-2: updatePageCaption / splitPage / movePhoto = nil (未使用)
		usecase.NewMergePages(pool),   // STOP P-3
		usecase.NewReorderPages(pool), // STOP P-3
	)
	r := chi.NewRouter()
	r.Post("/api/photobooks/{id}/pages/{pageId}/merge-into/{targetPageId}", h.MergePages)
	r.Patch("/api/photobooks/{id}/pages/reorder", h.ReorderPages)
	return r
}

// ============================================================================
// POST /pages/{pageId}/merge-into/{targetPageId}
// ============================================================================

func TestMergePagesHandler(t *testing.T) {
	pool := dbPoolForHandler(t)
	r := setupPageMergeReorderRouter(pool, &imageuploadtests.FakeR2Client{})

	t.Run("正常_200_EditView返却_source削除_target統合", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		pageB, _ := addPageViaUC(t, pool, pb)
		// pageA に photo 1 件、pageB に photo 1 件
		_, _ = addPhotoViaUC(t, pool, pb, pageA)
		_, vAfterPhB := addPhotoViaUC(t, pool, pb, pageB)

		body := []byte(`{"expected_version": ` + intToStr(vAfterPhB) + `}`)
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/pages/"+pageB+"/merge-into/"+pageA, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s want 200", rec.Code, rec.Body.String())
		}
		// B 方式: EditView 全体を返す。photobook_id / version / pages array 等が含まれる。
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if resp["photobook_id"] != pb.ID().String() {
			t.Errorf("photobook_id mismatch in response")
		}
		// pages 配列が 1 件 (pageB merged into pageA、pageB 削除済)
		pages, ok := resp["pages"].([]any)
		if !ok {
			t.Fatalf("pages not array")
		}
		if len(pages) != 1 {
			t.Errorf("pages count=%d want 1", len(pages))
		}

		// raw secret / token / Cookie が response に出ていないことの defensive guard
		bodyStr := rec.Body.String()
		for _, leak := range []string{"manage_url", "draft_token", "Set-Cookie", "session_token"} {
			if strings.Contains(bodyStr, leak) {
				t.Errorf("response contains secret marker %q", leak)
			}
		}
	})

	t.Run("異常_409_self_merge_reason_merge_into_self", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		_, vAfter := addPageViaUC(t, pool, pb)

		body := []byte(`{"expected_version": ` + intToStr(vAfter) + `}`)
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/pages/"+pageA+"/merge-into/"+pageA, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d want 409", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"reason":"merge_into_self"`) {
			t.Errorf("body=%s want reason=merge_into_self", rec.Body.String())
		}
	})

	t.Run("異常_409_sole_page_reason_cannot_remove_last_page", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, vAfter := addPageViaUC(t, pool, pb)

		// fake target で sole page reject (UseCase 内部で sole page 判定が先に走る)
		fakeTarget := "00000000-0000-0000-0000-000000000001"
		body := []byte(`{"expected_version": ` + intToStr(vAfter) + `}`)
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/pages/"+pageA+"/merge-into/"+fakeTarget, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s want 409", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"reason":"cannot_remove_last_page"`) {
			t.Errorf("body=%s want reason=cannot_remove_last_page", rec.Body.String())
		}
	})

	t.Run("異常_409_version_conflict_oldVersion", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		pageB, _ := addPageViaUC(t, pool, pb)

		// expected_version=0 (古い)、bumpVersion で OCC 違反
		body := []byte(`{"expected_version": 0}`)
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/pages/"+pageB+"/merge-into/"+pageA, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s want 409", rec.Code, rec.Body.String())
		}
		// OCC 違反は reason なし (敵対者観測抑止)
		body2 := rec.Body.String()
		if !strings.Contains(body2, `"status":"version_conflict"`) {
			t.Errorf("body=%s want status=version_conflict", body2)
		}
		if strings.Contains(body2, `"reason"`) {
			t.Errorf("body=%s should NOT include reason for OCC", body2)
		}
	})

	t.Run("異常_400_malformed_JSON", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		pageB, _ := addPageViaUC(t, pool, pb)

		body := []byte(`{not_json`)
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/pages/"+pageB+"/merge-into/"+pageA, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status=%d want 400", rec.Code)
		}
	})

	t.Run("異常_404_invalid_photobookId_UUID", func(t *testing.T) {
		body := []byte(`{"expected_version": 0}`)
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/not-a-uuid/pages/00000000-0000-0000-0000-000000000001/merge-into/00000000-0000-0000-0000-000000000002",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status=%d want 404", rec.Code)
		}
	})
}

// ============================================================================
// PATCH /pages/reorder
// ============================================================================

func TestReorderPagesHandler(t *testing.T) {
	pool := dbPoolForHandler(t)
	r := setupPageMergeReorderRouter(pool, &imageuploadtests.FakeR2Client{})

	t.Run("正常_200_3page_reorder_EditView返却", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		pageB, _ := addPageViaUC(t, pool, pb)
		pageC, vAfter := addPageViaUC(t, pool, pb)

		// C(0), A(1), B(2) に reorder
		body := []byte(`{
			"assignments": [
				{"page_id": "` + pageC + `", "display_order": 0},
				{"page_id": "` + pageA + `", "display_order": 1},
				{"page_id": "` + pageB + `", "display_order": 2}
			],
			"expected_version": ` + intToStr(vAfter) + `
		}`)
		req := httptest.NewRequest(http.MethodPatch,
			"/api/photobooks/"+pb.ID().String()+"/pages/reorder", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s want 200", rec.Code, rec.Body.String())
		}

		// EditView の pages array が新しい順序 (C, A, B) で並ぶこと
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		pages, ok := resp["pages"].([]any)
		if !ok || len(pages) != 3 {
			t.Fatalf("pages count=%d want 3", len(pages))
		}
		// 順序検証は DB 側 (display_order ASC) で確定済 → handler test では
		// repo 経由で page id 順序を assert する
		repo := photobookrdb.NewPhotobookRepository(pool)
		pagesDB, _ := repo.ListPagesByPhotobookID(context.Background(), pb.ID())
		want := []string{pageC, pageA, pageB}
		for i, w := range want {
			if pagesDB[i].ID().String() != w {
				t.Errorf("pages[%d] id=%s want %s", i, pagesDB[i].ID().String(), w)
			}
		}
	})

	t.Run("異常_400_空assignments_reason_invalid_reorder_assignments", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)

		body := []byte(`{"assignments": [], "expected_version": 0}`)
		req := httptest.NewRequest(http.MethodPatch,
			"/api/photobooks/"+pb.ID().String()+"/pages/reorder", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"reason":"invalid_reorder_assignments"`) {
			t.Errorf("body=%s want reason=invalid_reorder_assignments", rec.Body.String())
		}
	})

	t.Run("異常_400_count_mismatch_reason_invalid_reorder_assignments", func(t *testing.T) {
		// 3 page あるのに 2 件の assignments
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		pageB, _ := addPageViaUC(t, pool, pb)
		_, vAfter := addPageViaUC(t, pool, pb)

		body := []byte(`{
			"assignments": [
				{"page_id": "` + pageA + `", "display_order": 0},
				{"page_id": "` + pageB + `", "display_order": 1}
			],
			"expected_version": ` + intToStr(vAfter) + `
		}`)
		req := httptest.NewRequest(http.MethodPatch,
			"/api/photobooks/"+pb.ID().String()+"/pages/reorder", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s want 400", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"reason":"invalid_reorder_assignments"`) {
			t.Errorf("body=%s want reason=invalid_reorder_assignments", rec.Body.String())
		}
	})

	t.Run("異常_400_invalid_page_id_UUID", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)

		body := []byte(`{
			"assignments": [
				{"page_id": "not-a-uuid", "display_order": 0}
			],
			"expected_version": 0
		}`)
		req := httptest.NewRequest(http.MethodPatch,
			"/api/photobooks/"+pb.ID().String()+"/pages/reorder", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status=%d want 400", rec.Code)
		}
	})

	t.Run("異常_409_version_conflict_oldVersion", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		pageB, _ := addPageViaUC(t, pool, pb)

		// expected_version=0 で OCC 違反、ただし validateReorderAssignments を
		// 通すため display_order=0,1 の 2 件を送る (page 数とも一致)
		body := []byte(`{
			"assignments": [
				{"page_id": "` + pageA + `", "display_order": 1},
				{"page_id": "` + pageB + `", "display_order": 0}
			],
			"expected_version": 0
		}`)
		req := httptest.NewRequest(http.MethodPatch,
			"/api/photobooks/"+pb.ID().String()+"/pages/reorder", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s want 409", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"status":"version_conflict"`) {
			t.Errorf("body=%s want status=version_conflict", rec.Body.String())
		}
	})

	t.Run("異常_404_invalid_photobookId_UUID", func(t *testing.T) {
		body := []byte(`{"assignments": [], "expected_version": 0}`)
		req := httptest.NewRequest(http.MethodPatch,
			"/api/photobooks/not-a-uuid/pages/reorder", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status=%d want 404", rec.Code)
		}
	})
}

// time import を維持するための reference (本ファイル内で時刻 helper を使う将来拡張用)
var _ = time.Now