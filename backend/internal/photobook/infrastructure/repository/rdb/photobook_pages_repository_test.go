// STOP P-1: Page caption / page offset / photo move primitive の Repository test。
//
// 目的:
//   - UpdatePageCaption: 正常 / 別 photobook の page に対して no-op
//   - BulkOffsetPagesInPhotobook: 全 page +1000、別 photobook には影響なし
//   - UpdatePhotoPageAndOrder: 正常 (同 page / 別 page) / 別 photobook の photo は ErrPhotoNotFound /
//     別 photobook の target page は ErrPageNotFound
//   - FindPhotoWithPhotobookID: 正常 / 不存在で ErrPhotoNotFound
//
// 実行方法 (photobook_repository_test.go と同じ):
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//	go -C backend test ./internal/photobook/infrastructure/repository/rdb/...
//
// DATABASE_URL 未設定なら skip。
//
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §7.1 (test matrix)
package rdb_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	imagedomain "vrcpb/backend/internal/image/domain"
	imagebuilders "vrcpb/backend/internal/image/domain/tests"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/normalized_format"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/domain"
	domaintests "vrcpb/backend/internal/photobook/domain/tests"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// truncateAllForPageRepoTests は page / photo / image を含む全テーブルを CASCADE で空にする。
// 既存 dbPool() の TRUNCATE は sessions/photobooks CASCADE のみで page まで届くが、
// 各 sub-test 冒頭に明示するため別関数化。
func truncateAllForPageRepoTests(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
}

// seedDraftPhotobookForRepo は draft photobook を 1 件 INSERT して返す (rdb_test package 内 helper)。
func seedDraftPhotobookForRepo(t *testing.T, pool *pgxpool.Pool) domain.Photobook {
	t.Helper()
	pb := domaintests.NewPhotobookBuilder().Build(t)
	repo := rdb.NewPhotobookRepository(pool)
	if err := repo.CreateDraft(context.Background(), pb); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	return pb
}

// seedAvailableImageForRepo は対象 photobook 所有の available image を 1 件作る
// (usecase test の seedAvailableImage と同等、rdb_test package 内に複製)。
func seedAvailableImageForRepo(t *testing.T, pool *pgxpool.Pool, ownerID photobook_id.PhotobookID) imagedomain.Image {
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

// addPageDirect は AddPage UseCase を介さず、Repository.AddPage で page を直接追加する
// (rdb_test 範囲を保つため)。displayOrder は呼び出し側が指定。
func addPageDirect(
	t *testing.T,
	pool *pgxpool.Pool,
	pb domain.Photobook,
	displayOrder int,
	expectedVersion int,
	now time.Time,
) (domain.Page, int) {
	t.Helper()
	repo := rdb.NewPhotobookRepository(pool)
	pid, err := page_id.New()
	if err != nil {
		t.Fatalf("page_id.New: %v", err)
	}
	order := mustOrder(t, displayOrder)
	page, err := domain.NewPage(domain.NewPageParams{
		ID:           pid,
		PhotobookID:  pb.ID(),
		DisplayOrder: order,
		Caption:      nil,
		Now:          now,
	})
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := repo.AddPage(context.Background(), pb.ID(), page, expectedVersion, now); err != nil {
		t.Fatalf("AddPage: %v", err)
	}
	return page, expectedVersion + 1
}

// addPhotoDirect は Repository.AddPhoto で photo を 1 件追加する。
func addPhotoDirect(
	t *testing.T,
	pool *pgxpool.Pool,
	pb domain.Photobook,
	pageID page_id.PageID,
	imgID imagedomain.Image,
	displayOrder int,
	expectedVersion int,
	now time.Time,
) (domain.Photo, int) {
	t.Helper()
	repo := rdb.NewPhotobookRepository(pool)
	phid, err := photo_id.New()
	if err != nil {
		t.Fatalf("photo_id.New: %v", err)
	}
	order := mustOrder(t, displayOrder)
	ph, err := domain.NewPhoto(domain.NewPhotoParams{
		ID:           phid,
		PageID:       pageID,
		ImageID:      imgID.ID(),
		DisplayOrder: order,
		Caption:      nil,
		Now:          now,
	})
	if err != nil {
		t.Fatalf("NewPhoto: %v", err)
	}
	if err := repo.AddPhoto(context.Background(), pb.ID(), pageID, ph, expectedVersion, now); err != nil {
		t.Fatalf("AddPhoto: %v", err)
	}
	return ph, expectedVersion + 1
}

func mustOrder(t *testing.T, n int) display_order.DisplayOrder {
	t.Helper()
	o, err := display_order.New(n)
	if err != nil {
		t.Fatalf("display_order.New(%d): %v", n, err)
	}
	return o
}

// ============================================================================
// UpdatePageCaption
// ============================================================================

func TestPhotobookRepository_UpdatePageCaption(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	repo := rdb.NewPhotobookRepository(pool)

	t.Run("正常_caption_設定", func(t *testing.T) {
		// Given: draft photobook + page (caption nil)
		// When: UpdatePageCaption("hello")
		// Then: 正常完了 + ListPagesByPhotobookID で caption="hello" 取得
		truncateAllForPageRepoTests(t, pool)
		pb := seedDraftPhotobookForRepo(t, pool)
		page, _ := addPageDirect(t, pool, pb, 0, pb.Version(), now)

		c := caption.MustNew("hello")
		if err := repo.UpdatePageCaption(ctx, pb.ID(), page.ID(), &c, now); err != nil {
			t.Fatalf("UpdatePageCaption: %v", err)
		}

		pages, err := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if err != nil {
			t.Fatalf("ListPages: %v", err)
		}
		if len(pages) != 1 {
			t.Fatalf("pages len=%d want 1", len(pages))
		}
		got := pages[0].Caption()
		if got == nil || got.String() != "hello" {
			t.Errorf("caption=%v want 'hello'", got)
		}
	})

	t.Run("正常_caption_クリア_nil_保存", func(t *testing.T) {
		// Given: page に caption 既存
		// When: UpdatePageCaption(nil)
		// Then: caption が NULL に戻る
		truncateAllForPageRepoTests(t, pool)
		pb := seedDraftPhotobookForRepo(t, pool)
		page, _ := addPageDirect(t, pool, pb, 0, pb.Version(), now)
		c := caption.MustNew("first")
		if err := repo.UpdatePageCaption(ctx, pb.ID(), page.ID(), &c, now); err != nil {
			t.Fatalf("first UpdatePageCaption: %v", err)
		}
		if err := repo.UpdatePageCaption(ctx, pb.ID(), page.ID(), nil, now); err != nil {
			t.Fatalf("nil UpdatePageCaption: %v", err)
		}
		pages, _ := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if pages[0].Caption() != nil {
			t.Errorf("caption=%v want nil", pages[0].Caption())
		}
	})

	t.Run("異常_別photobook_の_pageは更新されない", func(t *testing.T) {
		// Given: photobook A と B、それぞれ page を持つ
		// When: pbA.ID() で pbB の page を UpdatePageCaption しようとする
		// Then: ErrPageNotFound + 実 DB で B の page caption は変化なし
		truncateAllForPageRepoTests(t, pool)
		pbA := seedDraftPhotobookForRepo(t, pool)
		pbB := seedDraftPhotobookForRepo(t, pool)
		_, _ = addPageDirect(t, pool, pbA, 0, pbA.Version(), now)
		pageB, _ := addPageDirect(t, pool, pbB, 0, pbB.Version(), now)

		c := caption.MustNew("attack")
		err := repo.UpdatePageCaption(ctx, pbA.ID(), pageB.ID(), &c, now)
		if !errors.Is(err, rdb.ErrPageNotFound) {
			t.Fatalf("err=%v want ErrPageNotFound", err)
		}
		// pbB の page caption は依然 nil
		pagesB, _ := repo.ListPagesByPhotobookID(ctx, pbB.ID())
		if pagesB[0].Caption() != nil {
			t.Errorf("pbB page caption was modified: %v", pagesB[0].Caption())
		}
	})

	t.Run("異常_未知の_pageId_は_ErrPageNotFound", func(t *testing.T) {
		truncateAllForPageRepoTests(t, pool)
		pb := seedDraftPhotobookForRepo(t, pool)
		unknownPid, _ := page_id.New()
		c := caption.MustNew("x")
		err := repo.UpdatePageCaption(ctx, pb.ID(), unknownPid, &c, now)
		if !errors.Is(err, rdb.ErrPageNotFound) {
			t.Fatalf("err=%v want ErrPageNotFound", err)
		}
	})
}

// ============================================================================
// BulkOffsetPagesInPhotobook
// ============================================================================

func TestPhotobookRepository_BulkOffsetPagesInPhotobook(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	repo := rdb.NewPhotobookRepository(pool)

	t.Run("正常_全_page_の_display_order_が_+1000_される", func(t *testing.T) {
		// Given: photobook A に 3 page (display_order 0, 1, 2)、photobook B に 1 page (display_order 0)
		// When: A に対して BulkOffsetPagesInPhotobook
		// Then: A の page は 1000, 1001, 1002 に shift、B は 0 のまま
		truncateAllForPageRepoTests(t, pool)
		pbA := seedDraftPhotobookForRepo(t, pool)
		pbB := seedDraftPhotobookForRepo(t, pool)
		v := pbA.Version()
		_, v = addPageDirect(t, pool, pbA, 0, v, now)
		_, v = addPageDirect(t, pool, pbA, 1, v, now)
		_, _ = addPageDirect(t, pool, pbA, 2, v, now)
		_, _ = addPageDirect(t, pool, pbB, 0, pbB.Version(), now)

		if err := repo.BulkOffsetPagesInPhotobook(ctx, pbA.ID(), now); err != nil {
			t.Fatalf("BulkOffsetPages: %v", err)
		}
		pagesA, _ := repo.ListPagesByPhotobookID(ctx, pbA.ID())
		gotOrders := make([]int, 0, len(pagesA))
		for _, p := range pagesA {
			gotOrders = append(gotOrders, p.DisplayOrder().Int())
		}
		want := []int{1000, 1001, 1002}
		if len(gotOrders) != 3 {
			t.Fatalf("pagesA len=%d want 3", len(gotOrders))
		}
		for i, o := range gotOrders {
			if o != want[i] {
				t.Errorf("pagesA[%d] order=%d want %d", i, o, want[i])
			}
		}
		// B は 0 のまま
		pagesB, _ := repo.ListPagesByPhotobookID(ctx, pbB.ID())
		if pagesB[0].DisplayOrder().Int() != 0 {
			t.Errorf("pbB page order=%d want 0 (untouched)", pagesB[0].DisplayOrder().Int())
		}
	})

	t.Run("正常_0_page_な_photobook_は_no_op", func(t *testing.T) {
		truncateAllForPageRepoTests(t, pool)
		pb := seedDraftPhotobookForRepo(t, pool)
		if err := repo.BulkOffsetPagesInPhotobook(ctx, pb.ID(), now); err != nil {
			t.Errorf("BulkOffsetPages: %v (want nil)", err)
		}
	})
}

// ============================================================================
// UpdatePhotoPageAndOrder + FindPhotoWithPhotobookID
// ============================================================================

func TestPhotobookRepository_UpdatePhotoPageAndOrder(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	repo := rdb.NewPhotobookRepository(pool)

	t.Run("正常_別page_へ移動_target_display_order_新規", func(t *testing.T) {
		// Given: 1 photobook に 2 page (P1, P2)、P1 に photo X (order 0)、P2 は空
		// When: photo X を P2 の order 0 に move
		// Then: photo X.page_id = P2、P1 は空、P2 に photo X 1 件
		truncateAllForPageRepoTests(t, pool)
		pb := seedDraftPhotobookForRepo(t, pool)
		v := pb.Version()
		page1, v := addPageDirect(t, pool, pb, 0, v, now)
		page2, v := addPageDirect(t, pool, pb, 1, v, now)
		img := seedAvailableImageForRepo(t, pool, pb.ID())
		photo, _ := addPhotoDirect(t, pool, pb, page1.ID(), img, 0, v, now)

		if err := repo.UpdatePhotoPageAndOrder(ctx, pb.ID(), photo.ID(), page2.ID(), mustOrder(t, 0)); err != nil {
			t.Fatalf("UpdatePhotoPageAndOrder: %v", err)
		}

		// 検証: photo の現状
		gotPhoto, gotPbID, err := repo.FindPhotoWithPhotobookID(ctx, photo.ID())
		if err != nil {
			t.Fatalf("FindPhotoWithPhotobookID: %v", err)
		}
		if !gotPbID.Equal(pb.ID()) {
			t.Errorf("photobook_id mismatch")
		}
		if !gotPhoto.PageID().Equal(page2.ID()) {
			t.Errorf("photo page_id=%s want page2", gotPhoto.PageID())
		}
		if gotPhoto.DisplayOrder().Int() != 0 {
			t.Errorf("photo display_order=%d want 0", gotPhoto.DisplayOrder().Int())
		}
		// page1 は空、page2 は 1 件
		photosP1, _ := repo.ListPhotosByPageID(ctx, page1.ID())
		if len(photosP1) != 0 {
			t.Errorf("page1 photos=%d want 0", len(photosP1))
		}
		photosP2, _ := repo.ListPhotosByPageID(ctx, page2.ID())
		if len(photosP2) != 1 {
			t.Errorf("page2 photos=%d want 1", len(photosP2))
		}
	})

	t.Run("正常_同_page_内_display_order_変更", func(t *testing.T) {
		// Given: 1 photobook、1 page、photo X (order 0)、photo Y (order 1)
		// When: photo X を escape 経由で order 1 に、photo Y を order 0 に
		// Then: 入れ替わる (UseCase の bulk reorder と同等の動作確認 — primitive レベルでも可能)
		truncateAllForPageRepoTests(t, pool)
		pb := seedDraftPhotobookForRepo(t, pool)
		v := pb.Version()
		page, v := addPageDirect(t, pool, pb, 0, v, now)
		imgX := seedAvailableImageForRepo(t, pool, pb.ID())
		photoX, v := addPhotoDirect(t, pool, pb, page.ID(), imgX, 0, v, now)
		imgY := seedAvailableImageForRepo(t, pool, pb.ID())
		photoY, vAfterY := addPhotoDirect(t, pool, pb, page.ID(), imgY, 1, v, now)

		// escape + reorder: BulkReorderPhotosOnPage は内部で bumpVersion を呼ぶため、
		// 直前の addPhotoDirect 完了時点の version (= vAfterY) を expectedVersion に渡す
		if err := repo.BulkReorderPhotosOnPage(ctx, pb.ID(), page.ID(),
			[]rdb.PhotoOrderAssignment{
				{PhotoID: photoX.ID(), NewOrder: mustOrder(t, 1)},
				{PhotoID: photoY.ID(), NewOrder: mustOrder(t, 0)},
			},
			vAfterY,
			now,
		); err != nil {
			t.Fatalf("BulkReorderPhotosOnPage: %v", err)
		}
		photos, _ := repo.ListPhotosByPageID(ctx, page.ID())
		if len(photos) != 2 {
			t.Fatalf("photos len=%d want 2", len(photos))
		}
		// 入れ替わっていることを id で確認
		if !photos[0].ID().Equal(photoY.ID()) {
			t.Errorf("photos[0] id=%s want photoY", photos[0].ID())
		}
		if !photos[1].ID().Equal(photoX.ID()) {
			t.Errorf("photos[1] id=%s want photoX", photos[1].ID())
		}
	})

	t.Run("異常_別_photobook_の_photo_は_ErrPhotoNotFound", func(t *testing.T) {
		// Given: pbA に photo X (page 1)、pbB に page Y
		// When: pbA.ID() で photoB を ... or pbB の photo を pbA scope で move しようとする
		// Then: ErrPhotoNotFound (photo は pbA 配下にない)
		truncateAllForPageRepoTests(t, pool)
		pbA := seedDraftPhotobookForRepo(t, pool)
		pbB := seedDraftPhotobookForRepo(t, pool)
		vA := pbA.Version()
		_, _ = addPageDirect(t, pool, pbA, 0, vA, now)
		vB := pbB.Version()
		pageB, vB := addPageDirect(t, pool, pbB, 0, vB, now)
		imgB := seedAvailableImageForRepo(t, pool, pbB.ID())
		photoB, _ := addPhotoDirect(t, pool, pbB, pageB.ID(), imgB, 0, vB, now)

		// pbA scope で photoB を move しようとする
		err := repo.UpdatePhotoPageAndOrder(ctx, pbA.ID(), photoB.ID(), pageB.ID(), mustOrder(t, 0))
		if !errors.Is(err, rdb.ErrPhotoNotFound) {
			t.Fatalf("err=%v want ErrPhotoNotFound", err)
		}
	})

	t.Run("異常_別_photobook_の_target_page_は_ErrPageNotFound", func(t *testing.T) {
		// Given: pbA に photo X (page A1)、pbB に page B1
		// When: photo X を pbB の page B1 に move しようとする (ownership 不一致)
		// Then: ErrPageNotFound
		truncateAllForPageRepoTests(t, pool)
		pbA := seedDraftPhotobookForRepo(t, pool)
		pbB := seedDraftPhotobookForRepo(t, pool)
		vA := pbA.Version()
		pageA1, vA := addPageDirect(t, pool, pbA, 0, vA, now)
		imgA := seedAvailableImageForRepo(t, pool, pbA.ID())
		photoA, _ := addPhotoDirect(t, pool, pbA, pageA1.ID(), imgA, 0, vA, now)
		pageB1, _ := addPageDirect(t, pool, pbB, 0, pbB.Version(), now)

		err := repo.UpdatePhotoPageAndOrder(ctx, pbA.ID(), photoA.ID(), pageB1.ID(), mustOrder(t, 0))
		if !errors.Is(err, rdb.ErrPageNotFound) {
			t.Fatalf("err=%v want ErrPageNotFound", err)
		}
	})

	t.Run("異常_未知の_photo_id_は_ErrPhotoNotFound", func(t *testing.T) {
		truncateAllForPageRepoTests(t, pool)
		pb := seedDraftPhotobookForRepo(t, pool)
		page, _ := addPageDirect(t, pool, pb, 0, pb.Version(), now)
		unknownPhid, _ := photo_id.New()
		err := repo.UpdatePhotoPageAndOrder(ctx, pb.ID(), unknownPhid, page.ID(), mustOrder(t, 0))
		if !errors.Is(err, rdb.ErrPhotoNotFound) {
			t.Fatalf("err=%v want ErrPhotoNotFound", err)
		}
	})

	t.Run("異常_未知の_target_page_id_は_ErrPageNotFound", func(t *testing.T) {
		truncateAllForPageRepoTests(t, pool)
		pb := seedDraftPhotobookForRepo(t, pool)
		v := pb.Version()
		page, v := addPageDirect(t, pool, pb, 0, v, now)
		img := seedAvailableImageForRepo(t, pool, pb.ID())
		photo, _ := addPhotoDirect(t, pool, pb, page.ID(), img, 0, v, now)
		unknownPageID, _ := page_id.New()
		err := repo.UpdatePhotoPageAndOrder(ctx, pb.ID(), photo.ID(), unknownPageID, mustOrder(t, 0))
		if !errors.Is(err, rdb.ErrPageNotFound) {
			t.Fatalf("err=%v want ErrPageNotFound", err)
		}
	})
}

func TestPhotobookRepository_FindPhotoWithPhotobookID(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	repo := rdb.NewPhotobookRepository(pool)

	t.Run("正常_photo_と_photobook_id_を返す", func(t *testing.T) {
		truncateAllForPageRepoTests(t, pool)
		pb := seedDraftPhotobookForRepo(t, pool)
		v := pb.Version()
		page, v := addPageDirect(t, pool, pb, 0, v, now)
		img := seedAvailableImageForRepo(t, pool, pb.ID())
		photo, _ := addPhotoDirect(t, pool, pb, page.ID(), img, 0, v, now)

		gotPhoto, gotPbID, err := repo.FindPhotoWithPhotobookID(ctx, photo.ID())
		if err != nil {
			t.Fatalf("FindPhotoWithPhotobookID: %v", err)
		}
		if !gotPhoto.ID().Equal(photo.ID()) {
			t.Errorf("photo id mismatch")
		}
		if !gotPhoto.PageID().Equal(page.ID()) {
			t.Errorf("page id mismatch")
		}
		if !gotPbID.Equal(pb.ID()) {
			t.Errorf("photobook id mismatch")
		}
	})

	t.Run("異常_未知_photo_id_は_ErrPhotoNotFound", func(t *testing.T) {
		truncateAllForPageRepoTests(t, pool)
		_ = seedDraftPhotobookForRepo(t, pool)
		unknownPhid, _ := photo_id.New()
		_, _, err := repo.FindPhotoWithPhotobookID(ctx, unknownPhid)
		if !errors.Is(err, rdb.ErrPhotoNotFound) {
			t.Fatalf("err=%v want ErrPhotoNotFound", err)
		}
	})
}
