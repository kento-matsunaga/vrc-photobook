// AttachPrepareImages handler の HTTP layer test。
//
// 設計参照: docs/plan/m2-prepare-resilience-and-throughput-plan.md §3.4 / §5
//
// 観点（user 指示 sub-step 2-9 完了条件）:
//   - 200 正常: count-only response、raw ID 非露出、headers
//   - 400 invalid JSON / invalid expected_version
//   - 404 photobook 不存在 / invalid UUID
//   - 409 status != draft / OCC mismatch
//   - 503 usecase 未注入
//   - route が draft session 認可下にあることを router-level で確認
//
// 注意: middleware（draft session Cookie の検証）は session middleware の test でカバー
// 済とし、本 test は middleware 通過後の handler 動作を検証する。401 verification は
// 専用 router test で middleware 配線下にあることを assert する。

package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	imagedomain "vrcpb/backend/internal/image/domain"
	imagebuilders "vrcpb/backend/internal/image/domain/tests"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/normalized_format"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/domain"
	photobooktests "vrcpb/backend/internal/photobook/domain/tests"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

const truncateAllForAttachHandlerSQL = "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"

func truncateAllForAttachHandler(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), truncateAllForAttachHandlerSQL); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
}

func seedDraftPhotobookForAttachHandler(t *testing.T, pool *pgxpool.Pool) domain.Photobook {
	t.Helper()
	pb := photobooktests.NewPhotobookBuilder().Build(t)
	repo := photobookrdb.NewPhotobookRepository(pool)
	if err := repo.CreateDraft(context.Background(), pb); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	return pb
}

func seedAvailableImageForAttachHandler(
	t *testing.T,
	pool *pgxpool.Pool,
	ownerID photobook_id.PhotobookID,
) imagedomain.Image {
	t.Helper()
	repo := imagerdb.NewImageRepository(pool)
	img := imagebuilders.NewImageBuilder().WithOwnerPhotobookID(ownerID).Build(t)
	ctx := context.Background()
	if err := repo.CreateUploading(ctx, img); err != nil {
		t.Fatalf("CreateUploading: %v", err)
	}
	now := time.Now().UTC()
	processed, _ := img.MarkProcessing(now)
	if err := repo.MarkProcessing(ctx, processed); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}
	dims, _ := image_dimensions.New(800, 600)
	bs, _ := byte_size.New(50_000)
	avail, err := processed.MarkAvailable(imagedomain.MarkAvailableParams{
		NormalizedFormat:   normalized_format.Webp(),
		OriginalDimensions: dims,
		OriginalByteSize:   bs,
		MetadataStrippedAt: now.Add(time.Second),
		Now:                now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("MarkAvailable: %v", err)
	}
	if err := repo.MarkAvailable(ctx, avail); err != nil {
		t.Fatalf("repo.MarkAvailable: %v", err)
	}
	return avail
}

// setupAttachRouter は handler 直接 ServeHTTP 用の chi router（middleware なし、handler の挙動だけ見る）。
func setupAttachRouter(pool *pgxpool.Pool) (http.Handler, *photobookhttp.EditHandlers) {
	uc := usecase.NewAttachAvailableImages(pool)
	// 他の usecase 引数は本 test では使わないため nil でも問題ないが、NewEditHandlers は
	// 全引数を要求するため最小限注入する（attach 以外の handler は本 test で呼ばれない）。
	h := photobookhttp.NewEditHandlers(
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		uc,
		nil, nil, nil, // STOP P-2: updatePageCaption / splitPage / movePhoto = nil (未使用)
		nil, nil, // STOP P-3: mergePages / reorderPages = nil (未使用)
	)
	r := chi.NewRouter()
	r.Post("/api/photobooks/{id}/prepare/attach-images", h.AttachPrepareImages)
	return r, h
}

// setupAttachRouterWithNilUsecase は 503 検証用 (usecase 未注入)。
func setupAttachRouterWithNilUsecase() http.Handler {
	h := photobookhttp.NewEditHandlers(
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, // attachAvailableImages = nil
		nil, nil, nil, // STOP P-2: updatePageCaption / splitPage / movePhoto = nil
		nil, nil, // STOP P-3: mergePages / reorderPages = nil
	)
	r := chi.NewRouter()
	r.Post("/api/photobooks/{id}/prepare/attach-images", h.AttachPrepareImages)
	return r
}

func TestAttachPrepareImagesHandler(t *testing.T) {
	pool := dbPoolForHandler(t)

	t.Run("正常_200_count_only_response_raw_ID非露出", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		_ = seedAvailableImageForAttachHandler(t, pool, pb.ID())
		_ = seedAvailableImageForAttachHandler(t, pool, pb.ID())

		router, _ := setupAttachRouter(pool)
		body := mustJSON(t, map[string]int{"expected_version": pb.Version()})
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/prepare/attach-images",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
		}
		// headers (commonHeaders 経由で付与)
		if got := rr.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}
		if got := rr.Header().Get("X-Robots-Tag"); got != "noindex, nofollow" {
			t.Errorf("X-Robots-Tag = %q, want noindex, nofollow", got)
		}
		// response shape: count フィールド 3 つだけ、raw ID 系は出ない
		var got map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("response keys = %d (expected exactly 3: attached_count/page_count/skipped_count), got=%v", len(got), got)
		}
		if v, ok := got["attached_count"].(float64); !ok || int(v) != 2 {
			t.Errorf("attached_count = %v, want 2", got["attached_count"])
		}
		if _, ok := got["page_count"]; !ok {
			t.Errorf("page_count missing")
		}
		if _, ok := got["skipped_count"]; !ok {
			t.Errorf("skipped_count missing")
		}
		// raw ID / Secret / R2 系の禁止語が body に出ていない
		bodyStr := rr.Body.String()
		assertNoRawIDLeakage(t, bodyStr)
	})

	t.Run("異常_400_invalid_JSON", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)

		router, _ := setupAttachRouter(pool)
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/prepare/attach-images",
			strings.NewReader("not json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400, body=%s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "bad_request") {
			t.Errorf("body = %q, want contain 'bad_request'", rr.Body.String())
		}
	})

	t.Run("異常_400_expected_version_型違い_文字列", func(t *testing.T) {
		// expected_version: "abc" のような型違いは JSON decode で 400
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)

		router, _ := setupAttachRouter(pool)
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/prepare/attach-images",
			strings.NewReader(`{"expected_version":"abc"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (expected_version 型違い、JSON decode 失敗)", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "bad_request") {
			t.Errorf("body = %q, want contain 'bad_request'", rr.Body.String())
		}
	})

	t.Run("異常_400_expected_version_負数", func(t *testing.T) {
		// expected_version=-1 は入力として不正、handler validation で 400
		// （usecase に渡して 409 にしない、user 指示 §3 推奨）
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)

		router, _ := setupAttachRouter(pool)
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/prepare/attach-images",
			strings.NewReader(`{"expected_version":-1}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (negative expected_version)", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "bad_request") {
			t.Errorf("body = %q, want contain 'bad_request'", rr.Body.String())
		}
	})

	t.Run("正常_expected_version_欠落は0として扱う_version0と一致すれば200", func(t *testing.T) {
		// 仕様決定: expected_version field 不在は Go json zero-value 0 として扱う
		// （明示的な validation は行わず、usecase で photobook の実 version と比較。
		// 初回 attach (version=0) ならそのまま成功する）。
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		_ = seedAvailableImageForAttachHandler(t, pool, pb.ID())

		router, _ := setupAttachRouter(pool)
		// expected_version field を含めない body
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/prepare/attach-images",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// pb.Version() == 0（seed 直後）なので 0 として扱われた expected_version が一致 → 200
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (omitted expected_version = 0、photobook version 0 一致), body=%s",
				rr.Code, rr.Body.String())
		}
	})

	t.Run("異常_404_invalid_photobook_UUID", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)

		router, _ := setupAttachRouter(pool)
		body := mustJSON(t, map[string]int{"expected_version": 0})
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/not-a-uuid/prepare/attach-images",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "not_found") {
			t.Errorf("body = %q, want contain 'not_found'", rr.Body.String())
		}
	})

	t.Run("異常_404_photobook_存在しない", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)

		router, _ := setupAttachRouter(pool)
		nonExistentID := "11111111-1111-1111-1111-111111111111"
		body := mustJSON(t, map[string]int{"expected_version": 0})
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+nonExistentID+"/prepare/attach-images",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "not_found") {
			t.Errorf("body = %q, want contain 'not_found'", rr.Body.String())
		}
	})

	t.Run("異常_409_OCC_version_mismatch", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		_ = seedAvailableImageForAttachHandler(t, pool, pb.ID())

		router, _ := setupAttachRouter(pool)
		body := mustJSON(t, map[string]int{"expected_version": pb.Version() + 99})
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/prepare/attach-images",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "version_conflict") {
			t.Errorf("body = %q, want contain 'version_conflict'", rr.Body.String())
		}
	})

	t.Run("異常_409_status_not_draft", func(t *testing.T) {
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)
		// 直接 SQL で status=purged に変更（migration §photobooks_status_columns_consistency_check
		// で purged は ELSE TRUE で全 column 制約から外れる、最も簡素な非 draft 状態。
		// published / deleted は他 column の整合性が必要で fixture が複雑になるため避ける）。
		// test fixture、production パス経由しない。
		if _, err := pool.Exec(context.Background(),
			"UPDATE photobooks SET status='purged' WHERE id=$1",
			pgtype.UUID{Bytes: pb.ID().UUID(), Valid: true},
		); err != nil {
			t.Fatalf("status update: %v", err)
		}

		router, _ := setupAttachRouter(pool)
		body := mustJSON(t, map[string]int{"expected_version": pb.Version()})
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/"+pb.ID().String()+"/prepare/attach-images",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "version_conflict") {
			t.Errorf("body = %q, want contain 'version_conflict'", rr.Body.String())
		}
	})

	t.Run("異常_503_usecase未注入", func(t *testing.T) {
		router := setupAttachRouterWithNilUsecase()
		body := mustJSON(t, map[string]int{"expected_version": 0})
		req := httptest.NewRequest(http.MethodPost,
			"/api/photobooks/11111111-1111-1111-1111-111111111111/prepare/attach-images",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "internal_error") {
			t.Errorf("body = %q, want contain 'internal_error'", rr.Body.String())
		}
	})
}

// assertNoRawIDLeakage は response body に raw ID / Secret / R2 系の禁止語が出ていないことを assert する。
//
// pattern は plan v2 §6.5 + §15 + .agents/rules/security-guard.md に準拠:
//   - storage_key / upload_url / r2_endpoint / Bearer / set-cookie 等
//   - photo_id / page_id / image_id field名（response shape として attached_count 等以外を使わない）
func assertNoRawIDLeakage(t *testing.T, body string) {
	t.Helper()
	forbidden := []string{
		"\"image_id\"",
		"\"page_id\"",
		"\"photo_id\"",
		"\"storage_key\"",
		"\"upload_url\"",
		"\"r2_endpoint\"",
		"storage_key",
		"upload-intent",
		"Bearer ",
		"sk_live_",
		"sk_test_",
		"DATABASE_URL",
		"Set-Cookie",
		"set-cookie",
	}
	for _, w := range forbidden {
		if strings.Contains(body, w) {
			t.Errorf("response body contains forbidden token %q (raw ID/Secret leak), body=%s", w, body)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// TestAttachPrepareImagesRouteIsUnderDraftSessionMiddleware は user 必須項目 §6
// 「route が draft session 認可下にあることを router/handler test で確認」を満たす。
//
// 検証アプローチ:
//   - 実 chi router を組み立てて Cookie 不在 401 を返す test は、middleware の
//     Validator interface が internal package（auth/session/internal/usecase）の
//     ValidateSessionInput / Output を要求するため http_test パッケージから直接
//     mock 不能（internal import 制約）
//   - 代替として、router.go のソース上で /prepare/attach-images route が
//     RequireDraftSession middleware を Use している sub-router の同 block 内に
//     登録されていることを inspect する file-based verification を行う
//   - 既存 GetEditView / AddPage 等は同 sub-router で middleware 通過後の挙動を
//     handler test で個別検証しており、middleware 自体の挙動は
//     auth/session/middleware の test layer でカバー済
func TestAttachPrepareImagesRouteIsUnderDraftSessionMiddleware(t *testing.T) {
	// router.go の絶対 path（test 実行時 cwd は backend/internal/photobook/interface/http）
	src, err := os.ReadFile("../../../http/router.go")
	if err != nil {
		t.Fatalf("ReadFile router.go: %v", err)
	}
	body := string(src)

	// /prepare/attach-images route の登録箇所
	const attachRoute = `sub.Post("/prepare/attach-images"`
	attachIdx := strings.Index(body, attachRoute)
	if attachIdx < 0 {
		t.Fatalf("router.go does not register %q", attachRoute)
	}

	// RequireDraftSession middleware の Use 箇所
	const requireDraftMiddleware = `sub.Use(authmiddleware.RequireDraftSession(cfg.DraftSessionValidator, photobookIDFromURL))`
	requireIdx := strings.Index(body, requireDraftMiddleware)
	if requireIdx < 0 {
		t.Fatalf("router.go does not contain RequireDraftSession middleware Use call")
	}

	// 同一 sub-router block 内であることを brace 範囲で確認:
	//   r.Route("...", func(sub chi.Router) {
	//     sub.Use(authmiddleware.RequireDraftSession(...))   ← requireIdx
	//     sub.Get(...)
	//     ...
	//     sub.Post("/prepare/attach-images", ...)            ← attachIdx
	//     ...
	//   })
	//
	// 単純化: requireIdx < attachIdx であり、その間に閉じる block （\n\t})）が無いことを確認。
	if attachIdx < requireIdx {
		t.Fatalf("attach-images is registered BEFORE RequireDraftSession middleware (configured outside sub-router)")
	}
	between := body[requireIdx:attachIdx]
	if strings.Contains(between, "\n\t})") {
		t.Errorf("RequireDraftSession block is closed BEFORE /prepare/attach-images is registered (route is outside the auth sub-router)")
	}
}
