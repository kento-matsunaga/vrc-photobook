// STOP P-2: Phase A 核 3 endpoint (UpdatePageCaption / SplitPage / MovePhoto)
// HTTP layer test。
//
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §7.3 (handler test matrix)
//
// 観点:
//   - 200 正常系
//   - 400 invalid JSON / missing expected_version / invalid position / 不正 UUID
//   - 404 photobookId / pageId / photoId 不存在 (or invalid)
//   - 409 reason mapping: version_conflict / page_limit_exceeded /
//     split_would_create_empty_page
//   - response shape: A 方式 (caption は {"version": N+1}) / B 方式 (split/move は EditView)
//   - raw token / Cookie / Secret が response / log に出ない
//
// 注意: middleware (draft session) は別 test でカバー済の前提、本 test は handler 直接呼出。
package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	imageuploadtests "vrcpb/backend/internal/imageupload/tests"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

func intToStr(n int) string {
	return strconv.Itoa(n)
}

func domainPageIDFromString(s string) (page_id.PageID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return page_id.PageID{}, err
	}
	return page_id.FromUUID(u)
}

// setupPageSplitMoveRouter は本 test 専用 router (handler 直接呼出、middleware なし)。
func setupPageSplitMoveRouter(pool *pgxpool.Pool, fakeR2 r2.Client) http.Handler {
	h := photobookhttp.NewEditHandlers(
		usecase.NewGetEditView(pool, fakeR2),
		nil, nil, nil, // updatePhotoCaption / bulkReorder / updateSettings = nil
		nil, nil, nil, // addPage / removePage / removePhoto = nil
		nil, nil, // setCover / clearCover = nil
		nil,                                  // attachAvailableImages = nil
		usecase.NewUpdatePageCaption(pool),   // STOP P-2
		usecase.NewSplitPage(pool),           // STOP P-2
		usecase.NewMovePhotoBetweenPages(pool), // STOP P-2
		nil, nil, // STOP P-3: mergePages / reorderPages = nil (本 test では未使用)
	)
	r := chi.NewRouter()
	r.Patch("/api/photobooks/{id}/pages/{pageId}/caption", h.UpdatePageCaption)
	r.Post("/api/photobooks/{id}/pages/{pageId}/split", h.SplitPage)
	r.Patch("/api/photobooks/{id}/photos/{photoId}/move", h.MovePhoto)
	return r
}

// addPageViaUC は test 用 page seed (UseCase 経由)。
func addPageViaUC(t *testing.T, pool *pgxpool.Pool, pb domain.Photobook) (pageID string, newVersion int) {
	t.Helper()
	addPage := usecase.NewAddPage(pool)
	repo := photobookrdb.NewPhotobookRepository(pool)
	pbCur, _ := repo.FindByID(context.Background(), pb.ID())
	out, err := addPage.Execute(context.Background(), usecase.AddPageInput{
		PhotobookID: pb.ID(), ExpectedVersion: pbCur.Version(), Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("AddPage: %v", err)
	}
	pbAfter, _ := repo.FindByID(context.Background(), pb.ID())
	return out.Page.ID().String(), pbAfter.Version()
}

// addPhotoViaUC は test 用 photo seed (UseCase 経由)。pageID は文字列 (URL param 互換)。
func addPhotoViaUC(t *testing.T, pool *pgxpool.Pool, pb domain.Photobook, pageIDStr string) (photoID string, newVersion int) {
	t.Helper()
	img := seedAvailableImageForAttachHandler(t, pool, pb.ID())
	repo := photobookrdb.NewPhotobookRepository(pool)
	pbCur, _ := repo.FindByID(context.Background(), pb.ID())
	addPhoto := usecase.NewAddPhoto(pool)
	pidVO, err := domainPageIDFromString(pageIDStr)
	if err != nil {
		t.Fatalf("page id parse: %v", err)
	}
	out, err := addPhoto.Execute(context.Background(), usecase.AddPhotoInput{
		PhotobookID: pb.ID(), PageID: pidVO,
		ImageID: img.ID(), ExpectedVersion: pbCur.Version(), Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("AddPhoto: %v", err)
	}
	pbAfter, _ := repo.FindByID(context.Background(), pb.ID())
	return out.Photo.ID().String(), pbAfter.Version()
}

// ============================================================================
// PATCH /pages/{pageId}/caption (A 方式: {"version": N+1})
// ============================================================================

func TestUpdatePageCaptionHandler(t *testing.T) {
	pool := dbPoolForHandler(t)
	r := setupPageSplitMoveRouter(pool, &imageuploadtests.FakeR2Client{})

	t.Run("正常_200_caption設定_version_N+1_返却", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageID, vAfterPage := addPageViaUC(t, pool, pb)

		body := []byte(`{"caption": "hello", "expected_version": ` + intToStr(vAfterPage) + `}`)
		req := httptest.NewRequest(http.MethodPatch, "/api/photobooks/"+pb.ID().String()+"/pages/"+pageID+"/caption", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Version int `json:"version"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Version != vAfterPage+1 {
			t.Errorf("response version=%d want %d", resp.Version, vAfterPage+1)
		}
	})

	t.Run("正常_null_caption_clear", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageID, vAfterPage := addPageViaUC(t, pool, pb)
		body := []byte(`{"caption": null, "expected_version": ` + intToStr(vAfterPage) + `}`)
		req := httptest.NewRequest(http.MethodPatch, "/api/photobooks/"+pb.ID().String()+"/pages/"+pageID+"/caption", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("異常_400_invalid_JSON", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageID, _ := addPageViaUC(t, pool, pb)
		req := httptest.NewRequest(http.MethodPatch, "/api/photobooks/"+pb.ID().String()+"/pages/"+pageID+"/caption", bytes.NewReader([]byte("{not json")))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status=%d want 400 / body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("異常_404_invalid_pageId_UUID", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		body := []byte(`{"caption": "x", "expected_version": 0}`)
		req := httptest.NewRequest(http.MethodPatch, "/api/photobooks/"+pb.ID().String()+"/pages/not-a-uuid/caption", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status=%d want 404", rec.Code)
		}
	})

	t.Run("異常_409_version_conflict", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageID, _ := addPageViaUC(t, pool, pb)
		// 古い version (= pb.Version() = 0) を使う、AddPage で +1 になっているはず
		body := []byte(`{"caption": "x", "expected_version": ` + intToStr(pb.Version()) + `}`)
		req := httptest.NewRequest(http.MethodPatch, "/api/photobooks/"+pb.ID().String()+"/pages/"+pageID+"/caption", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d want 409 body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "version_conflict") {
			t.Errorf("body=%s want version_conflict", rec.Body.String())
		}
	})
}

// ============================================================================
// POST /pages/{pageId}/split (B 方式: 更新後 EditView)
// ============================================================================

func TestSplitPageHandler(t *testing.T) {
	pool := dbPoolForHandler(t)
	r := setupPageSplitMoveRouter(pool, &imageuploadtests.FakeR2Client{})

	t.Run("正常_200_EditView_返却_pages_2件_含む", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageID, _ := addPageViaUC(t, pool, pb)
		ph1, _ := addPhotoViaUC(t, pool, pb, pageID)
		_, vAfter := addPhotoViaUC(t, pool, pb, pageID)
		// split at ph1 (= index 0) → source: 1 photo, new: 1 photo
		body := []byte(`{"photo_id": "` + ph1 + `", "expected_version": ` + intToStr(vAfter) + `}`)
		req := httptest.NewRequest(http.MethodPost, "/api/photobooks/"+pb.ID().String()+"/pages/"+pageID+"/split", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// EditView shape を確認
		pages, ok := resp["pages"].([]any)
		if !ok || len(pages) != 2 {
			t.Errorf("pages len mismatch in response (got %v)", resp["pages"])
		}
		// version は vAfter + 1
		v, _ := resp["version"].(float64)
		if int(v) != vAfter+1 {
			t.Errorf("response version=%d want %d", int(v), vAfter+1)
		}
	})

	t.Run("異常_409_split_would_create_empty_page_reason", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageID, _ := addPageViaUC(t, pool, pb)
		_, _ = addPhotoViaUC(t, pool, pb, pageID)
		// 末尾 photo で split → 空 page を作るので reject
		ph2, vAfter := addPhotoViaUC(t, pool, pb, pageID)
		body := []byte(`{"photo_id": "` + ph2 + `", "expected_version": ` + intToStr(vAfter) + `}`)
		req := httptest.NewRequest(http.MethodPost, "/api/photobooks/"+pb.ID().String()+"/pages/"+pageID+"/split", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d want 409 body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"reason":"split_would_create_empty_page"`) {
			t.Errorf("body=%s want split_would_create_empty_page", rec.Body.String())
		}
	})

	t.Run("異常_400_invalid_photo_id_uuid", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageID, vAfter := addPageViaUC(t, pool, pb)
		body := []byte(`{"photo_id": "not-uuid", "expected_version": ` + intToStr(vAfter) + `}`)
		req := httptest.NewRequest(http.MethodPost, "/api/photobooks/"+pb.ID().String()+"/pages/"+pageID+"/split", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status=%d want 400", rec.Code)
		}
	})

	t.Run("異常_400_invalid_JSON", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageID, _ := addPageViaUC(t, pool, pb)
		req := httptest.NewRequest(http.MethodPost, "/api/photobooks/"+pb.ID().String()+"/pages/"+pageID+"/split", bytes.NewReader([]byte("{not json")))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status=%d want 400", rec.Code)
		}
	})
}

// ============================================================================
// PATCH /photos/{photoId}/move (B 方式: 更新後 EditView)
// ============================================================================

func TestMovePhotoHandler(t *testing.T) {
	pool := dbPoolForHandler(t)
	r := setupPageSplitMoveRouter(pool, &imageuploadtests.FakeR2Client{})

	t.Run("正常_200_別page_end_へ移動_EditView返却", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		pageB, _ := addPageViaUC(t, pool, pb)
		photoA, _ := addPhotoViaUC(t, pool, pb, pageA)
		_, vAfter := addPhotoViaUC(t, pool, pb, pageB)

		body := []byte(`{"target_page_id": "` + pageB + `", "position": "end", "expected_version": ` + intToStr(vAfter) + `}`)
		req := httptest.NewRequest(http.MethodPatch, "/api/photobooks/"+pb.ID().String()+"/photos/"+photoA+"/move", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// version +1
		v, _ := resp["version"].(float64)
		if int(v) != vAfter+1 {
			t.Errorf("version=%d want %d", int(v), vAfter+1)
		}
	})

	t.Run("異常_400_invalid_position", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		photoA, vAfter := addPhotoViaUC(t, pool, pb, pageA)
		body := []byte(`{"target_page_id": "` + pageA + `", "position": "middle", "expected_version": ` + intToStr(vAfter) + `}`)
		req := httptest.NewRequest(http.MethodPatch, "/api/photobooks/"+pb.ID().String()+"/photos/"+photoA+"/move", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"reason":"invalid_position"`) {
			t.Errorf("body=%s want invalid_position", rec.Body.String())
		}
	})

	t.Run("異常_400_invalid_target_page_id_uuid", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		photoA, vAfter := addPhotoViaUC(t, pool, pb, pageA)
		body := []byte(`{"target_page_id": "not-uuid", "position": "end", "expected_version": ` + intToStr(vAfter) + `}`)
		req := httptest.NewRequest(http.MethodPatch, "/api/photobooks/"+pb.ID().String()+"/photos/"+photoA+"/move", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status=%d want 400", rec.Code)
		}
	})

	t.Run("異常_409_version_conflict", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		photoA, _ := addPhotoViaUC(t, pool, pb, pageA)
		// 古い version
		body := []byte(`{"target_page_id": "` + pageA + `", "position": "end", "expected_version": 0}`)
		req := httptest.NewRequest(http.MethodPatch, "/api/photobooks/"+pb.ID().String()+"/photos/"+photoA+"/move", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Errorf("status=%d want 409 body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("異常_response_に_raw_token_Cookie_が_含まれない", func(t *testing.T) {
		// 200 path response shape の安全性確認 (B 方式 EditView は presigned URL を含むが、
		// それは expected output。token / Cookie / Secret 文字列が response に出ないことを確認)
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		pageA, _ := addPageViaUC(t, pool, pb)
		pageB, _ := addPageViaUC(t, pool, pb)
		photoA, _ := addPhotoViaUC(t, pool, pb, pageA)
		_, vAfter := addPhotoViaUC(t, pool, pb, pageB)
		body := []byte(`{"target_page_id": "` + pageB + `", "position": "end", "expected_version": ` + intToStr(vAfter) + `}`)
		req := httptest.NewRequest(http.MethodPatch, "/api/photobooks/"+pb.ID().String()+"/photos/"+photoA+"/move", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		bodyStr := rec.Body.String()
		for _, forbidden := range []string{
			"draft_edit_token", "manage_url_token", "session_token",
			"Set-Cookie", "Bearer ", "DATABASE_URL", "TURNSTILE_SECRET",
		} {
			if strings.Contains(bodyStr, forbidden) {
				t.Errorf("response body contains %q (raw value leak)", forbidden)
			}
		}
	})
}
