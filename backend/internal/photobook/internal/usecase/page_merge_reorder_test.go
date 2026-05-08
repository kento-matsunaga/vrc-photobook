// STOP P-3: Phase A 補強 2 endpoint (MergePages / ReorderPages) UseCase の実 DB 統合テスト。
//
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §7 (test matrix) / §3.4.4 / §3.4.5
//
// 観点:
//   - MergePages: 正常 / source != target / source の caption / page_meta が捨てられる /
//     source 削除後の display_order 詰め直し / version+1 / source == target reject /
//     sole page reject / version conflict / 別 photobook の page reject
//   - ReorderPages: 正常 (3 page swap) / version+1 / 部分 assignments reject /
//     重複 page_id reject / 0..N-1 permutation 違反 reject / 別 photobook の page_id 混入 reject /
//     version conflict
//
// 実行方法: page_split_move_test.go と同じ。dbPool / seedPhotobook / seedAvailableImage /
// truncateAll は同 package 内 helper を流用。
package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// seedPageWithNPhotosForMR は page を 1 枚追加し N photo を attach する helper。
//
// page_split_move_test.go の seedPageWithNPhotos と同等だが、test 関数 scope を超えて再利用する
// ため file-level helper として外出し。
func seedPageWithNPhotosForMR(
	t *testing.T,
	pool *pgxpool.Pool,
	pb domain.Photobook,
	photoCount int,
	now time.Time,
) (page_id.PageID, []photo_id.PhotoID) {
	t.Helper()
	ctx := context.Background()
	repo := photobookrdb.NewPhotobookRepository(pool)
	addPage := usecase.NewAddPage(pool)
	pbCur, _ := repo.FindByID(ctx, pb.ID())
	pageOut, err := addPage.Execute(ctx, usecase.AddPageInput{
		PhotobookID: pb.ID(), ExpectedVersion: pbCur.Version(), Now: now,
	})
	if err != nil {
		t.Fatalf("AddPage: %v", err)
	}
	photoIDs := make([]photo_id.PhotoID, 0, photoCount)
	addPhoto := usecase.NewAddPhoto(pool)
	for i := 0; i < photoCount; i++ {
		img := seedAvailableImage(t, pool, pb.ID())
		pbN, _ := repo.FindByID(ctx, pb.ID())
		out, err := addPhoto.Execute(ctx, usecase.AddPhotoInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			ImageID: img.ID(), ExpectedVersion: pbN.Version(), Now: now,
		})
		if err != nil {
			t.Fatalf("AddPhoto[%d]: %v", i, err)
		}
		photoIDs = append(photoIDs, out.Photo.ID())
	}
	return pageOut.Page.ID(), photoIDs
}

// ============================================================================
// MergePages
// ============================================================================

func TestMergePages(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

	t.Run("正常_2photoのsourceを3photoのtargetにmerge", func(t *testing.T) {
		// source: 2 photo / target: 3 photo (target は source の前 / display_order=0、source=1)
		// → merge 後 target に 5 photo (target 既存 0..2、source 由来 3..4)、source page 削除
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		// page A: 3 photos (target、display_order=0)
		pageA, phA := seedPageWithNPhotosForMR(t, pool, pb, 3, now)
		// page B: 2 photos (source、display_order=1)
		pageB, phB := seedPageWithNPhotosForMR(t, pool, pb, 2, now)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewMergePages(pool)
		err := uc.Execute(ctx, usecase.MergePagesInput{
			PhotobookID: pb.ID(), SourcePageID: pageB, TargetPageID: pageA,
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if err != nil {
			t.Fatalf("MergePages: %v", err)
		}

		// version +1
		pbAfter, _ := repo.FindByID(ctx, pb.ID())
		if pbAfter.Version() != pbBefore.Version()+1 {
			t.Errorf("version=%d want %d", pbAfter.Version(), pbBefore.Version()+1)
		}

		// page 数: 1 (page B 削除済)
		pages, _ := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if len(pages) != 1 {
			t.Fatalf("page count=%d want 1", len(pages))
		}
		if !pages[0].ID().Equal(pageA) {
			t.Errorf("remaining page should be target (pageA)")
		}
		if pages[0].DisplayOrder().Int() != 0 {
			t.Errorf("page A order=%d want 0", pages[0].DisplayOrder().Int())
		}

		// page A photo: 5 件 (0..4)、source 由来 photo は末尾 (3,4)
		photos, _ := repo.ListPhotosByPageID(ctx, pageA)
		if len(photos) != 5 {
			t.Fatalf("page A photo count=%d want 5", len(photos))
		}
		for i := 0; i < 5; i++ {
			if photos[i].DisplayOrder().Int() != i {
				t.Errorf("photo[%d] order=%d want %d", i, photos[i].DisplayOrder().Int(), i)
			}
		}
		// 既存 target photos (phA[0..2]) が 0..2、source 由来 (phB[0..1]) が 3..4
		for i := 0; i < 3; i++ {
			if !photos[i].ID().Equal(phA[i]) {
				t.Errorf("photo[%d] id mismatch with target original", i)
			}
		}
		for i := 0; i < 2; i++ {
			if !photos[3+i].ID().Equal(phB[i]) {
				t.Errorf("photo[%d] id mismatch with source", 3+i)
			}
		}
	})

	t.Run("正常_source_caption_は捨てられtargetを保持", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageB, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		repo := photobookrdb.NewPhotobookRepository(pool)

		// pageA に caption "TARGET"、pageB に caption "SOURCE" を設定
		ucCap := usecase.NewUpdatePageCaption(pool)
		pbN, _ := repo.FindByID(ctx, pb.ID())
		capA := caption.MustNew("TARGET")
		_ = ucCap.Execute(ctx, usecase.UpdatePageCaptionInput{
			PhotobookID: pb.ID(), PageID: pageA, Caption: &capA,
			ExpectedVersion: pbN.Version(), Now: now,
		})
		pbN, _ = repo.FindByID(ctx, pb.ID())
		capB := caption.MustNew("SOURCE")
		_ = ucCap.Execute(ctx, usecase.UpdatePageCaptionInput{
			PhotobookID: pb.ID(), PageID: pageB, Caption: &capB,
			ExpectedVersion: pbN.Version(), Now: now,
		})

		// merge B into A
		pbBefore, _ := repo.FindByID(ctx, pb.ID())
		uc := usecase.NewMergePages(pool)
		if err := uc.Execute(ctx, usecase.MergePagesInput{
			PhotobookID: pb.ID(), SourcePageID: pageB, TargetPageID: pageA,
			ExpectedVersion: pbBefore.Version(), Now: now,
		}); err != nil {
			t.Fatalf("MergePages: %v", err)
		}
		pages, _ := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if len(pages) != 1 {
			t.Fatalf("page count=%d want 1", len(pages))
		}
		if got := pages[0].Caption(); got == nil || got.String() != "TARGET" {
			t.Errorf("caption=%v want TARGET (source captionは捨てられるべき)", got)
		}
	})

	t.Run("正常_source削除後の以降page繰り上げ_3page", func(t *testing.T) {
		// page A (order=0) / B (order=1) / C (order=2)、merge B into A → 結果: A (0), C (1)
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageB, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageC, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewMergePages(pool)
		if err := uc.Execute(ctx, usecase.MergePagesInput{
			PhotobookID: pb.ID(), SourcePageID: pageB, TargetPageID: pageA,
			ExpectedVersion: pbBefore.Version(), Now: now,
		}); err != nil {
			t.Fatalf("MergePages: %v", err)
		}
		pages, _ := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if len(pages) != 2 {
			t.Fatalf("page count=%d want 2", len(pages))
		}
		// pages[0] = A (order=0), pages[1] = C (order=1)
		if !pages[0].ID().Equal(pageA) {
			t.Errorf("pages[0] should be pageA")
		}
		if !pages[1].ID().Equal(pageC) {
			t.Errorf("pages[1] should be pageC")
		}
		if pages[0].DisplayOrder().Int() != 0 || pages[1].DisplayOrder().Int() != 1 {
			t.Errorf("orders=[%d,%d] want [0,1]",
				pages[0].DisplayOrder().Int(), pages[1].DisplayOrder().Int())
		}
	})

	t.Run("異常_source_eq_target", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		_, _ = seedPageWithNPhotosForMR(t, pool, pb, 1, now) // pageB (otherwise sole page)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewMergePages(pool)
		err := uc.Execute(ctx, usecase.MergePagesInput{
			PhotobookID: pb.ID(), SourcePageID: pageA, TargetPageID: pageA,
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if !errors.Is(err, usecase.ErrMergeIntoSelf) {
			t.Errorf("err=%v want ErrMergeIntoSelf", err)
		}
		// version は bump されないはず
		pbAfter, _ := repo.FindByID(ctx, pb.ID())
		if pbAfter.Version() != pbBefore.Version() {
			t.Errorf("version bumped on self-merge: %d -> %d",
				pbBefore.Version(), pbAfter.Version())
		}
	})

	t.Run("異常_sole_pageでmerge_reject", func(t *testing.T) {
		// page 1 件のみ (sole page) を merge source にできない (5.13)
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		// 同一 page を target にできない (self-merge guard) ので、
		// 別 fake page_id を target に渡す。先に self-merge check が走るため別 ID 必須。
		fakeTarget := mustNewPageID(t)
		uc := usecase.NewMergePages(pool)
		err := uc.Execute(ctx, usecase.MergePagesInput{
			PhotobookID: pb.ID(), SourcePageID: pageA, TargetPageID: fakeTarget,
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if !errors.Is(err, usecase.ErrCannotRemoveLastPage) {
			t.Errorf("err=%v want ErrCannotRemoveLastPage", err)
		}
	})

	t.Run("異常_version_conflict", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageB, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		uc := usecase.NewMergePages(pool)
		err := uc.Execute(ctx, usecase.MergePagesInput{
			PhotobookID: pb.ID(), SourcePageID: pageB, TargetPageID: pageA,
			ExpectedVersion: pb.Version(), // 古い (AddPage / AddPhoto で +1 多数)
			Now:             now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Errorf("err=%v want OptimisticLockConflict", err)
		}
	})

	t.Run("異常_別photobookのpageを_target指定_PageNotFound", func(t *testing.T) {
		truncateAll(t, pool)
		pbA := seedPhotobook(t, pool)
		pbB := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pbA, 1, now)
		_, _ = seedPageWithNPhotosForMR(t, pool, pbA, 1, now) // pbA 配下に 2 page で sole 回避
		pageBOnB, _ := seedPageWithNPhotosForMR(t, pool, pbB, 1, now)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbABefore, _ := repo.FindByID(ctx, pbA.ID())

		uc := usecase.NewMergePages(pool)
		err := uc.Execute(ctx, usecase.MergePagesInput{
			PhotobookID:     pbA.ID(),
			SourcePageID:    pageA,
			TargetPageID:    pageBOnB, // pbB の page を pbA scope で
			ExpectedVersion: pbABefore.Version(),
			Now:             now,
		})
		if !errors.Is(err, photobookrdb.ErrPageNotFound) {
			t.Errorf("err=%v want ErrPageNotFound", err)
		}
	})
}

// ============================================================================
// ReorderPages
// ============================================================================

func TestReorderPages(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

	t.Run("正常_3page_swap", func(t *testing.T) {
		// A(0), B(1), C(2) → C(0), A(1), B(2) に reorder
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageB, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageC, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewReorderPages(pool)
		err := uc.Execute(ctx, usecase.ReorderPagesInput{
			PhotobookID: pb.ID(),
			Assignments: []usecase.ReorderPagesAssignment{
				{PageID: pageC, DisplayOrder: mustOrder(t, 0)},
				{PageID: pageA, DisplayOrder: mustOrder(t, 1)},
				{PageID: pageB, DisplayOrder: mustOrder(t, 2)},
			},
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if err != nil {
			t.Fatalf("ReorderPages: %v", err)
		}

		// version +1
		pbAfter, _ := repo.FindByID(ctx, pb.ID())
		if pbAfter.Version() != pbBefore.Version()+1 {
			t.Errorf("version=%d want %d", pbAfter.Version(), pbBefore.Version()+1)
		}

		pages, _ := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if len(pages) != 3 {
			t.Fatalf("page count=%d want 3", len(pages))
		}
		// pages は display_order ASC で返る
		want := []page_id.PageID{pageC, pageA, pageB}
		for i, expected := range want {
			if !pages[i].ID().Equal(expected) {
				t.Errorf("pages[%d] id mismatch", i)
			}
			if pages[i].DisplayOrder().Int() != i {
				t.Errorf("pages[%d] order=%d want %d", i, pages[i].DisplayOrder().Int(), i)
			}
		}
	})

	t.Run("異常_部分assignments_count_mismatch", func(t *testing.T) {
		// 3 page あるのに 2 件しか送らない
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageB, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		_, _ = seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewReorderPages(pool)
		err := uc.Execute(ctx, usecase.ReorderPagesInput{
			PhotobookID: pb.ID(),
			Assignments: []usecase.ReorderPagesAssignment{
				{PageID: pageA, DisplayOrder: mustOrder(t, 0)},
				{PageID: pageB, DisplayOrder: mustOrder(t, 1)},
			},
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if !errors.Is(err, usecase.ErrInvalidReorderAssignments) {
			t.Errorf("err=%v want ErrInvalidReorderAssignments", err)
		}
	})

	t.Run("異常_重複page_id", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageB, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewReorderPages(pool)
		err := uc.Execute(ctx, usecase.ReorderPagesInput{
			PhotobookID: pb.ID(),
			Assignments: []usecase.ReorderPagesAssignment{
				{PageID: pageA, DisplayOrder: mustOrder(t, 0)},
				{PageID: pageA, DisplayOrder: mustOrder(t, 1)}, // 重複
			},
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if !errors.Is(err, usecase.ErrInvalidReorderAssignments) {
			t.Errorf("err=%v want ErrInvalidReorderAssignments", err)
		}
		_ = pageB
	})

	t.Run("異常_display_order_permutation違反_欠番", func(t *testing.T) {
		// 0..N-1 の permutation でない (0,1,3 など)
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageB, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageC, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewReorderPages(pool)
		err := uc.Execute(ctx, usecase.ReorderPagesInput{
			PhotobookID: pb.ID(),
			Assignments: []usecase.ReorderPagesAssignment{
				{PageID: pageA, DisplayOrder: mustOrder(t, 0)},
				{PageID: pageB, DisplayOrder: mustOrder(t, 1)},
				{PageID: pageC, DisplayOrder: mustOrder(t, 3)}, // 欠番 (2 が無い)
			},
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if !errors.Is(err, usecase.ErrInvalidReorderAssignments) {
			t.Errorf("err=%v want ErrInvalidReorderAssignments", err)
		}
	})

	t.Run("異常_別photobookのpage混入", func(t *testing.T) {
		truncateAll(t, pool)
		pbA := seedPhotobook(t, pool)
		pbB := seedPhotobook(t, pool)
		pageAOnA, _ := seedPageWithNPhotosForMR(t, pool, pbA, 1, now)
		pageBOnA, _ := seedPageWithNPhotosForMR(t, pool, pbA, 1, now)
		pageAOnB, _ := seedPageWithNPhotosForMR(t, pool, pbB, 1, now) // 別 photobook
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbABefore, _ := repo.FindByID(ctx, pbA.ID())

		uc := usecase.NewReorderPages(pool)
		err := uc.Execute(ctx, usecase.ReorderPagesInput{
			PhotobookID: pbA.ID(),
			Assignments: []usecase.ReorderPagesAssignment{
				{PageID: pageAOnA, DisplayOrder: mustOrder(t, 0)},
				{PageID: pageBOnA, DisplayOrder: mustOrder(t, 1)},
				{PageID: pageAOnB, DisplayOrder: mustOrder(t, 2)}, // pbB の page
			},
			ExpectedVersion: pbABefore.Version(), Now: now,
		})
		// page count mismatch (pbA は 2 page、assignments は 3 件) なので Invalid 扱い
		if !errors.Is(err, usecase.ErrInvalidReorderAssignments) {
			t.Errorf("err=%v want ErrInvalidReorderAssignments", err)
		}
	})

	t.Run("異常_version_conflict", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)
		pageB, _ := seedPageWithNPhotosForMR(t, pool, pb, 1, now)

		uc := usecase.NewReorderPages(pool)
		err := uc.Execute(ctx, usecase.ReorderPagesInput{
			PhotobookID: pb.ID(),
			Assignments: []usecase.ReorderPagesAssignment{
				{PageID: pageA, DisplayOrder: mustOrder(t, 1)},
				{PageID: pageB, DisplayOrder: mustOrder(t, 0)},
			},
			ExpectedVersion: pb.Version(), // 古い (AddPage / AddPhoto で +1 多数)
			Now:             now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Errorf("err=%v want OptimisticLockConflict", err)
		}
	})
}

// mustNewPageID は test 用の random page_id を生成する (sole page test の fake target で利用)。
func mustNewPageID(t *testing.T) page_id.PageID {
	t.Helper()
	id, err := page_id.New()
	if err != nil {
		t.Fatalf("page_id.New: %v", err)
	}
	return id
}

// mustOrder は test 用の display_order を生成する。
func mustOrder(t *testing.T, n int) display_order.DisplayOrder {
	t.Helper()
	o, err := display_order.New(n)
	if err != nil {
		t.Fatalf("display_order.New(%d): %v", n, err)
	}
	return o
}
