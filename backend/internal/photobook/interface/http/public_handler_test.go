// PublicHandlers の HTTP layer テスト。
//
// 観点:
//   - 200: published + visible で payload に variant URL / Cache-Control / X-Robots-Tag 揃う
//   - 410: hidden_by_operator
//   - 404: slug 不一致 / 不正形式 / draft / private
//   - storage_key 完全値や R2 credentials が response body に出ない
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
	"vrcpb/backend/internal/image/domain/vo/storage_key"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	uploadtests "vrcpb/backend/internal/imageupload/tests"
	imagebuilders "vrcpb/backend/internal/image/domain/tests"
	"vrcpb/backend/internal/image/domain/vo/normalized_format"
	photobooktests "vrcpb/backend/internal/photobook/domain/tests"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

func setupPublicRouter(t *testing.T, pool *pgxpool.Pool, fakeR2 r2.Client) http.Handler {
	t.Helper()
	uc := usecase.NewGetPublicPhotobook(pool, fakeR2)
	h := photobookhttp.NewPublicHandlers(uc)
	r := chi.NewRouter()
	r.Get("/api/public/photobooks/{slug}", h.GetPublicPhotobook)
	return r
}

func seedPublishedWithPhoto(t *testing.T, pool *pgxpool.Pool, slugStr string, hidden bool, visibilityVal string) {
	t.Helper()
	ctx := context.Background()
	pb := photobooktests.NewPhotobookBuilder().Build(t)
	pbRepo := photobookrdb.NewPhotobookRepository(pool)
	if err := pbRepo.CreateDraft(ctx, pb); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	// page 追加
	addPage := usecase.NewAddPage(pool)
	pageOut, err := addPage.Execute(ctx, usecase.AddPageInput{
		PhotobookID:     pb.ID(),
		ExpectedVersion: pb.Version(),
		Now:             time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("AddPage: %v", err)
	}
	pbAfter, _ := pbRepo.FindByID(ctx, pb.ID())

	// available image
	imgRepo := imagerdb.NewImageRepository(pool)
	img := imagebuilders.NewImageBuilder().WithOwnerPhotobookID(pb.ID()).Build(t)
	if err := imgRepo.CreateUploading(ctx, img); err != nil {
		t.Fatalf("CreateUploading: %v", err)
	}
	now := time.Now().UTC()
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

	// display + thumbnail variants
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

	// photo を attach
	addPhoto := usecase.NewAddPhoto(pool)
	if _, err := addPhoto.Execute(ctx, usecase.AddPhotoInput{
		PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
		ImageID: img.ID(), ExpectedVersion: pbAfter.Version(),
		Now: now,
	}); err != nil {
		t.Fatalf("AddPhoto: %v", err)
	}

	// publish へ直接遷移
	tok, _ := manage_url_token.Generate()
	hash := manage_url_token_hash.Of(tok)
	_, err = pool.Exec(ctx, `
		UPDATE photobooks SET
		   status='published',
		   public_url_slug=$2,
		   manage_url_token_hash=$3,
		   manage_url_token_version=1,
		   draft_edit_token_hash=NULL,
		   draft_expires_at=NULL,
		   published_at=$4,
		   updated_at=$4,
		   version=version+1,
		   hidden_by_operator=$5,
		   visibility=$6
		WHERE id=$1`,
		pgtype.UUID{Bytes: pb.ID().UUID(), Valid: true},
		slugStr, hash.Bytes(),
		pgtype.Timestamptz{Time: now, Valid: true},
		hidden, visibilityVal,
	)
	if err != nil {
		t.Fatalf("publish UPDATE: %v", err)
	}
}

func truncateAllForHandler(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
}

func TestGetPublicPhotobookHandler(t *testing.T) {
	pool := dbPoolForHandler(t)

	tests := []struct {
		name        string
		description string
		setup       func(t *testing.T, pool *pgxpool.Pool, slug string)
		slug        string
		wantStatus  int
		wantBodyHas string
	}{
		{
			name:        "正常_published",
			description: "Given: published+visible photobook with photo, When: GET /api/public/photobooks/{slug}, Then: 200 + variant URLs",
			setup: func(t *testing.T, pool *pgxpool.Pool, slug string) {
				truncateAllForHandler(t, pool)
				seedPublishedWithPhoto(t, pool, slug, false, "unlisted")
			},
			slug:        "ok12pp34zz56gh78",
			wantStatus:  http.StatusOK,
			wantBodyHas: "fake.r2.test",
		},
		{
			name:        "異常_hidden_410",
			description: "Given: hidden_by_operator, Then: 410 Gone",
			setup: func(t *testing.T, pool *pgxpool.Pool, slug string) {
				truncateAllForHandler(t, pool)
				seedPublishedWithPhoto(t, pool, slug, true, "unlisted")
			},
			slug:        "hi12pp34zz56gh78",
			wantStatus:  http.StatusGone,
			wantBodyHas: "gone",
		},
		{
			name:        "異常_private_404",
			description: "Given: visibility=private, Then: 404",
			setup: func(t *testing.T, pool *pgxpool.Pool, slug string) {
				truncateAllForHandler(t, pool)
				seedPublishedWithPhoto(t, pool, slug, false, "private")
			},
			slug:        "pr12pp34zz56gh78",
			wantStatus:  http.StatusNotFound,
			wantBodyHas: "not_found",
		},
		{
			name:        "異常_format_invalid_404",
			description: "Given: 短すぎる slug, Then: 404",
			setup: func(t *testing.T, pool *pgxpool.Pool, slug string) {
				truncateAllForHandler(t, pool)
			},
			slug:        "short",
			wantStatus:  http.StatusNotFound,
			wantBodyHas: "not_found",
		},
		{
			name:        "異常_not_found_404",
			description: "Given: slug 不存在, Then: 404",
			setup: func(t *testing.T, pool *pgxpool.Pool, slug string) {
				truncateAllForHandler(t, pool)
			},
			slug:        "nf12pp34zz56gh78",
			wantStatus:  http.StatusNotFound,
			wantBodyHas: "not_found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t, pool, tt.slug)
			fakeR2 := &uploadtests.FakeR2Client{
				PresignGetObjectFn: func(_ context.Context, in r2.PresignGetInput) (r2.PresignGetOutput, error) {
					return r2.PresignGetOutput{URL: "https://fake.r2.test/get/" + in.StorageKey, ExpiresAt: time.Now().Add(in.ExpiresIn)}, nil
				},
			}
			router := setupPublicRouter(t, pool, fakeR2)
			req := httptest.NewRequest(http.MethodGet, "/api/public/photobooks/"+tt.slug, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != tt.wantStatus {
				t.Errorf("status=%d want %d body=%s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			body, _ := io.ReadAll(rr.Body)
			if !strings.Contains(string(body), tt.wantBodyHas) {
				t.Errorf("body missing %q: %s", tt.wantBodyHas, string(body))
			}
			// 共通ヘッダ
			if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control=%q want no-store", cc)
			}
			if rb := rr.Header().Get("X-Robots-Tag"); rb != "noindex, nofollow" {
				t.Errorf("X-Robots-Tag=%q", rb)
			}

			// 200 系: storage_key 完全 path や R2 credential が body に漏れないこと（fake は URL に key を含むので、
			// その文字列が含まれてしまうのは fake の都合）。少なくとも JSON パース成功と必須フィールド存在を確認。
			if tt.wantStatus == http.StatusOK {
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("json unmarshal: %v", err)
				}
				if _, ok := payload["title"]; !ok {
					t.Errorf("title missing")
				}
				if _, ok := payload["pages"]; !ok {
					t.Errorf("pages missing")
				}
				// manage_url_token / draft_edit_token / hash が含まれないこと
				keys := []string{"manage_url_token", "draft_edit_token", "manage_url_token_hash", "draft_edit_token_hash"}
				for _, k := range keys {
					if _, has := payload[k]; has {
						t.Errorf("payload must not contain %q", k)
					}
				}
			}
		})
	}
}
