// edit-view の images 拡張（plan v2 §3.2 P0-b）の handler test。
//
// 観点（user 指示 sub-step 2-10 完了条件）:
//   1. images に processing / available / failed が含まれる
//   2. attach 済 image と未配置 image の両方が images に含まれる
//   3. pages.*.photos 既存 response が壊れていない:
//      - attach 済 available image は従来通り pages.*.photos に出る
//      - 未配置 available image は pages.*.photos には出ず images には出る
//   4. processing_count / failed_count 既存値が images と整合
//   5. response body に storage_key / R2 URL / upload URL / Cookie / Secret が出ない
//   6. authenticated API response として image_id が images に含まれるのは許容
//      （UI/DOM/log/report には出さない方針、plan v2 §6.5）
//
// 実 DB integration test。`dbPoolForHandler` (handler_test.go) と FakeR2Client を使用。

package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	imagedomain "vrcpb/backend/internal/image/domain"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/mime_type"
	"vrcpb/backend/internal/image/domain/vo/normalized_format"
	"vrcpb/backend/internal/image/domain/vo/storage_key"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	imagebuilders "vrcpb/backend/internal/image/domain/tests"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	uploadtests "vrcpb/backend/internal/imageupload/tests"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// === fixture helpers ===

// seedAttachedAvailableImageWithVariantsForView は page 配置済 + variant 完備な image を seed する。
//
// edit-view が pages.*.photos に photo を出すには、image が available + display + thumbnail
// variant 完備 + photobook_photos に attach されている必要がある。本 helper はその完全
// fixture を組む（既存 public_handler_test の seedPublishedWithPhoto pattern を踏襲、
// publish ロジックは含めず draft 段階に留める）。
func seedAttachedAvailableImageWithVariantsForView(
	t *testing.T,
	pool *pgxpool.Pool,
	pb domain.Photobook,
) imagedomain.Image {
	t.Helper()
	ctx := context.Background()
	imgRepo := imagerdb.NewImageRepository(pool)
	now := time.Now().UTC()

	// 1. page 作成
	addPage := usecase.NewAddPage(pool)
	pageOut, err := addPage.Execute(ctx, usecase.AddPageInput{
		PhotobookID:     pb.ID(),
		ExpectedVersion: pb.Version(),
		Now:             now,
	})
	if err != nil {
		t.Fatalf("AddPage: %v", err)
	}

	// 2. uploading → processing → available の image
	img := imagebuilders.NewImageBuilder().WithOwnerPhotobookID(pb.ID()).Build(t)
	if err := imgRepo.CreateUploading(ctx, img); err != nil {
		t.Fatalf("CreateUploading: %v", err)
	}
	processed, _ := img.MarkProcessing(now)
	if err := imgRepo.MarkProcessing(ctx, processed); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}
	dims, _ := image_dimensions.New(800, 600)
	bs, _ := byte_size.New(50_000)
	avail, err := processed.MarkAvailable(imagedomain.MarkAvailableParams{
		NormalizedFormat:   normalized_format.Jpg(),
		OriginalDimensions: dims,
		OriginalByteSize:   bs,
		MetadataStrippedAt: now,
		Now:                now,
	})
	if err != nil {
		t.Fatalf("MarkAvailable: %v", err)
	}
	if err := imgRepo.MarkAvailable(ctx, avail); err != nil {
		t.Fatalf("repo MarkAvailable: %v", err)
	}

	// 3. display + thumbnail variant
	dispKey, _ := storage_key.GenerateForVariant(pb.ID(), img.ID(), variant_kind.Display())
	thumbKey, _ := storage_key.GenerateForVariant(pb.ID(), img.ID(), variant_kind.Thumbnail())
	dispDims, _ := image_dimensions.New(1600, 1200)
	thumbDims, _ := image_dimensions.New(480, 360)
	dispBs, _ := byte_size.New(150_000)
	thumbBs, _ := byte_size.New(20_000)
	dispVar, _ := imagedomain.NewImageVariant(imagedomain.NewImageVariantParams{
		ImageID: img.ID(), Kind: variant_kind.Display(), StorageKey: dispKey,
		Dimensions: dispDims, ByteSize: dispBs, MimeType: mime_type.Jpeg(), CreatedAt: now,
	})
	thumbVar, _ := imagedomain.NewImageVariant(imagedomain.NewImageVariantParams{
		ImageID: img.ID(), Kind: variant_kind.Thumbnail(), StorageKey: thumbKey,
		Dimensions: thumbDims, ByteSize: thumbBs, MimeType: mime_type.Jpeg(), CreatedAt: now,
	})
	if err := imgRepo.AttachVariant(ctx, dispVar); err != nil {
		t.Fatalf("AttachVariant display: %v", err)
	}
	if err := imgRepo.AttachVariant(ctx, thumbVar); err != nil {
		t.Fatalf("AttachVariant thumbnail: %v", err)
	}

	// 4. photobook_photos に attach
	addPhoto := usecase.NewAddPhoto(pool)
	if _, err := addPhoto.Execute(ctx, usecase.AddPhotoInput{
		PhotobookID:     pb.ID(),
		PageID:          pageOut.Page.ID(),
		ImageID:         img.ID(),
		ExpectedVersion: pb.Version() + 1, // AddPage で +1 された
		Now:             now,
	}); err != nil {
		t.Fatalf("AddPhoto: %v", err)
	}

	return avail
}

// seedProcessingImageForView は status='processing' の image を seed する。
func seedProcessingImageForView(
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
	processed, _ := img.MarkProcessing(time.Now().UTC())
	if err := repo.MarkProcessing(ctx, processed); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}
	return processed
}

// seedFailedImageForView は status='failed' の image を seed する（SQL UPDATE で直接、
// test fixture 専用）。
func seedFailedImageForView(
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
	processed, _ := img.MarkProcessing(time.Now().UTC())
	if err := repo.MarkProcessing(ctx, processed); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx,
		"UPDATE images SET status='failed', failed_at=$1, failure_reason='decode_failed' WHERE id=$2",
		pgtype.Timestamptz{Time: now, Valid: true},
		pgtype.UUID{Bytes: img.ID().UUID(), Valid: true}); err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}
	return processed
}

func setupEditViewRouter(pool *pgxpool.Pool, fakeR2 r2.Client) http.Handler {
	uc := usecase.NewGetEditView(pool, fakeR2)
	h := photobookhttp.NewEditHandlers(
		uc, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	r := chi.NewRouter()
	r.Get("/api/photobooks/{id}/edit-view", h.GetEditView)
	return r
}

// === tests ===

func TestEditViewImagesField(t *testing.T) {
	pool := dbPoolForHandler(t)
	fakeR2 := &uploadtests.FakeR2Client{
		PresignGetObjectFn: func(ctx context.Context, in r2.PresignGetInput) (r2.PresignGetOutput, error) {
			// 固定 fake URL（実 R2 endpoint には繋がない）
			return r2.PresignGetOutput{
				URL:       "https://fake.r2.test/get/" + in.StorageKey,
				ExpiresAt: time.Now().Add(15 * time.Minute),
			}, nil
		},
	}

	t.Run("正常_4種image_状態が_imagesに全部出る_pages.photosは_attach済のみ_countsと整合", func(t *testing.T) {
		// 注: image_id は authenticated API response に含めることは plan v2 §6.5 で許容済
		// （UI/DOM/log/report には出さない方針）。本 test では「images の各 entry に
		// image_id field が含まれる」ことを assert するが、raw 値を chat / docs / report
		// 本文には載せない。
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)

		// 1. attach 済 + variant 完備 (pages.*.photos に出る、images にも出る)
		_ = seedAttachedAvailableImageWithVariantsForView(t, pool, pb)
		// 2. 未配置 available（pages.*.photos には出ない、images にだけ出る）
		_ = seedAvailableImageForAttachHandler(t, pool, pb.ID())
		// 3. processing
		_ = seedProcessingImageForView(t, pool, pb.ID())
		// 4. failed
		_ = seedFailedImageForView(t, pool, pb.ID())

		router := setupEditViewRouter(pool, fakeR2)
		req := httptest.NewRequest(http.MethodGet,
			"/api/photobooks/"+pb.ID().String()+"/edit-view", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
		}
		bodyBytes, _ := io.ReadAll(rr.Body)
		bodyStr := string(bodyBytes)

		var payload struct {
			ProcessingCount int `json:"processing_count"`
			FailedCount     int `json:"failed_count"`
			Images          []struct {
				ImageID          string  `json:"image_id"`
				Status           string  `json:"status"`
				SourceFormat     string  `json:"source_format"`
				OriginalByteSize int64   `json:"original_byte_size"`
				FailureReason    *string `json:"failure_reason,omitempty"`
			} `json:"images"`
			Pages []struct {
				PageID string `json:"page_id"`
				Photos []struct {
					PhotoID string `json:"photo_id"`
					ImageID string `json:"image_id"`
				} `json:"photos"`
			} `json:"pages"`
		}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			t.Fatalf("Unmarshal: %v\nbody=%s", err, bodyStr)
		}

		// === 観点 1: images に 4 件全部含まれる ===
		if len(payload.Images) != 4 {
			t.Fatalf("images count = %d, want 4 (attach済 + 未配置 + processing + failed)", len(payload.Images))
		}

		// === 観点 1+4: status の internal 集計 ===
		statusCounts := map[string]int{}
		for _, img := range payload.Images {
			statusCounts[img.Status]++
		}
		if statusCounts["available"] != 2 {
			t.Errorf("available count = %d, want 2 (attach済 + 未配置)", statusCounts["available"])
		}
		if statusCounts["processing"] != 1 {
			t.Errorf("processing count in images = %d, want 1", statusCounts["processing"])
		}
		if statusCounts["failed"] != 1 {
			t.Errorf("failed count in images = %d, want 1", statusCounts["failed"])
		}

		// === 観点 4: processing_count / failed_count 既存値が images と整合 ===
		if payload.ProcessingCount != statusCounts["processing"] {
			t.Errorf("processing_count = %d but images の processing 数 = %d (整合してない)",
				payload.ProcessingCount, statusCounts["processing"])
		}
		if payload.FailedCount != statusCounts["failed"] {
			t.Errorf("failed_count = %d but images の failed 数 = %d (整合してない)",
				payload.FailedCount, statusCounts["failed"])
		}

		// === 観点 2 + 3: pages.*.photos は attach 済 available のみ（1 件） ===
		var totalPhotos int
		for _, p := range payload.Pages {
			totalPhotos += len(p.Photos)
		}
		if totalPhotos != 1 {
			t.Errorf("pages.photos 合計 = %d, want 1 (attach済 available のみ、未配置/processing/failed は出ない)", totalPhotos)
		}

		// === 観点 3: 未配置 available の image_id は images にあるが pages.photos には無い ===
		photoImageIDs := map[string]bool{}
		for _, p := range payload.Pages {
			for _, ph := range p.Photos {
				photoImageIDs[ph.ImageID] = true
			}
		}
		var availableImagesNotInPhotos int
		for _, img := range payload.Images {
			if img.Status == "available" && !photoImageIDs[img.ImageID] {
				availableImagesNotInPhotos++
			}
		}
		if availableImagesNotInPhotos != 1 {
			t.Errorf("未配置 available image 数 = %d, want 1 (images に出るが pages.photos には居ない)",
				availableImagesNotInPhotos)
		}

		// === 観点 5: storage_key / R2 URL / upload URL / Cookie / Secret 非露出 ===
		assertNoEditViewLeakage(t, bodyStr)

		// === 観点 6: image_id が images の entry に含まれることは許容（authenticated response 内部識別子）===
		for _, img := range payload.Images {
			if img.ImageID == "" {
				t.Errorf("image_id が空 (authenticated response 内部識別子は必須)")
			}
		}

		// === failed image の failure_reason は domain 値（user-friendly mapping は Frontend 側） ===
		for _, img := range payload.Images {
			if img.Status == "failed" {
				if img.FailureReason == nil || *img.FailureReason == "" {
					t.Errorf("failed image に failure_reason が無い")
				}
			}
		}
	})

	t.Run("正常_image無し_imagesは空配列_pages空_既存挙動維持", func(t *testing.T) {
		// regression: image を 1 件も seed しない場合、images=[]、pages=[]、counts=0
		truncateAllForAttachHandler(t, pool)
		pb := seedDraftPhotobookForAttachHandler(t, pool)

		router := setupEditViewRouter(pool, fakeR2)
		req := httptest.NewRequest(http.MethodGet,
			"/api/photobooks/"+pb.ID().String()+"/edit-view", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		bodyBytes, _ := io.ReadAll(rr.Body)
		var payload struct {
			ProcessingCount int             `json:"processing_count"`
			FailedCount     int             `json:"failed_count"`
			Images          []json.RawMessage `json:"images"`
			Pages           []json.RawMessage `json:"pages"`
		}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if len(payload.Images) != 0 {
			t.Errorf("images = %d, want 0", len(payload.Images))
		}
		if len(payload.Pages) != 0 {
			t.Errorf("pages = %d, want 0", len(payload.Pages))
		}
		if payload.ProcessingCount != 0 || payload.FailedCount != 0 {
			t.Errorf("counts = (%d, %d), want (0, 0)", payload.ProcessingCount, payload.FailedCount)
		}
		// images field 自体が response に存在することを assert
		if !strings.Contains(string(bodyBytes), `"images"`) {
			t.Errorf("response body に images field が存在しない")
		}
	})
}

// assertNoEditViewLeakage は edit-view response body に raw secret / storage_key /
// upload URL / Cookie / R2 endpoint が出ていないことを assert する。
//
// pages.*.photos.variants.display.url / thumbnail.url は presigned URL を含むが、
// fake URL（`https://fake.r2.test/...`）に固定しているため実 R2 endpoint や Secret は
// 出ない。本 test ではこれらの fake URL は許容、production 由来の禁止 token のみ check。
func assertNoEditViewLeakage(t *testing.T, body string) {
	t.Helper()
	forbidden := []string{
		// raw key 名（response field として出ないこと）
		"\"storage_key\"",
		"\"upload_url\"",
		"\"r2_endpoint\"",
		// raw secret / token / cookie
		"Bearer ",
		"sk_live_",
		"sk_test_",
		"DATABASE_URL",
		"Set-Cookie",
		"set-cookie",
		"draft_edit_token=",
		"manage_url_token=",
		// upload-intent の internal path
		"/upload-intent",
		"upload_verification_token",
	}
	for _, w := range forbidden {
		if strings.Contains(body, w) {
			t.Errorf("response body contains forbidden token %q", w)
		}
	}
}
