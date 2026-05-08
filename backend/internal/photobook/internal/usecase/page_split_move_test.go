// STOP P-2: Phase A 核 3 endpoint (UpdatePageCaption / SplitPage /
// MovePhotoBetweenPages) UseCase の実 DB 統合テスト。
//
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §7 (test matrix)
//
// 観点:
//   - UpdatePageCaption: 正常 / null clear / version conflict / page ownership / invalid caption
//   - SplitPage: middle photo / first photo / last photo edge / 30 page limit / ownership /
//     version conflict / display_order continuity / version+1
//   - MovePhotoBetweenPages: 別 page end / 別 page start / 同 page start / 同 page end /
//     source page が空残置 / target ownership / invalid position / version conflict / continuity
//
// 実行方法: edit_extras_test.go と同じ。dbPool / seedPhotobook / seedAvailableImage /
// truncateAll は同 package 内 helper を流用。
package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// ============================================================================
// UpdatePageCaption
// ============================================================================

func TestUpdatePageCaption(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

	t.Run("正常_caption設定", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		addPage := usecase.NewAddPage(pool)
		pageOut, _ := addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pb.ID(), ExpectedVersion: pb.Version(), Now: now,
		})
		repo := photobookrdb.NewPhotobookRepository(pool)
		pb1, _ := repo.FindByID(ctx, pb.ID())

		c := caption.MustNew("hello")
		uc := usecase.NewUpdatePageCaption(pool)
		if err := uc.Execute(ctx, usecase.UpdatePageCaptionInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			Caption: &c, ExpectedVersion: pb1.Version(), Now: now,
		}); err != nil {
			t.Fatalf("UpdatePageCaption: %v", err)
		}
		// version が +1 (page caption も A 方式 だが内部 bumpVersion で +1)
		pb2, _ := repo.FindByID(ctx, pb.ID())
		if pb2.Version() != pb1.Version()+1 {
			t.Errorf("version=%d want %d", pb2.Version(), pb1.Version()+1)
		}
		pages, _ := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if got := pages[0].Caption(); got == nil || got.String() != "hello" {
			t.Errorf("page caption=%v want hello", got)
		}
	})

	t.Run("正常_null_clear", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		addPage := usecase.NewAddPage(pool)
		pageOut, _ := addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pb.ID(), ExpectedVersion: pb.Version(), Now: now,
		})
		repo := photobookrdb.NewPhotobookRepository(pool)
		// まず caption を入れる
		pb1, _ := repo.FindByID(ctx, pb.ID())
		c := caption.MustNew("first")
		uc := usecase.NewUpdatePageCaption(pool)
		_ = uc.Execute(ctx, usecase.UpdatePageCaptionInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			Caption: &c, ExpectedVersion: pb1.Version(), Now: now,
		})
		// nil で clear
		pb2, _ := repo.FindByID(ctx, pb.ID())
		if err := uc.Execute(ctx, usecase.UpdatePageCaptionInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			Caption: nil, ExpectedVersion: pb2.Version(), Now: now,
		}); err != nil {
			t.Fatalf("UpdatePageCaption nil: %v", err)
		}
		pages, _ := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if pages[0].Caption() != nil {
			t.Errorf("caption=%v want nil", pages[0].Caption())
		}
	})

	t.Run("異常_version_conflict_oldVersion", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		addPage := usecase.NewAddPage(pool)
		pageOut, _ := addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pb.ID(), ExpectedVersion: pb.Version(), Now: now,
		})
		c := caption.MustNew("conflict")
		uc := usecase.NewUpdatePageCaption(pool)
		err := uc.Execute(ctx, usecase.UpdatePageCaptionInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			Caption: &c, ExpectedVersion: pb.Version(), // 古い (AddPage で +1 済)
			Now: now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Errorf("err=%v want OptimisticLockConflict", err)
		}
	})

	t.Run("異常_page_ownership_mismatch", func(t *testing.T) {
		// 別 photobook の page を更新しようとする
		truncateAll(t, pool)
		pbA := seedPhotobook(t, pool)
		pbB := seedPhotobook(t, pool)
		addPage := usecase.NewAddPage(pool)
		_, _ = addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pbA.ID(), ExpectedVersion: pbA.Version(), Now: now,
		})
		pageBOut, _ := addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pbB.ID(), ExpectedVersion: pbB.Version(), Now: now,
		})
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbA1, _ := repo.FindByID(ctx, pbA.ID())
		c := caption.MustNew("attack")
		uc := usecase.NewUpdatePageCaption(pool)
		err := uc.Execute(ctx, usecase.UpdatePageCaptionInput{
			PhotobookID: pbA.ID(), PageID: pageBOut.Page.ID(), // pbB の page を pbA scope で
			Caption: &c, ExpectedVersion: pbA1.Version(), Now: now,
		})
		if !errors.Is(err, photobookrdb.ErrPageNotFound) {
			t.Errorf("err=%v want ErrPageNotFound", err)
		}
	})

	t.Run("異常_invalid_caption_too_long", func(t *testing.T) {
		// caption.New は length 200 で reject。201 文字を渡すと caption.New エラー → handler 層で 400 にする想定だが、
		// UseCase では caption VO を作ってから渡す前提。ここでは domain VO 経由で reject される事を確認。
		_, err := caption.New(strings.Repeat("あ", 201))
		if err == nil {
			t.Errorf("caption 201 chars should fail validation but did not")
		}
	})
}

// ============================================================================
// SplitPage
// ============================================================================

func TestSplitPage(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

	// helper: page に N 枚 photo を attach
	seedPageWithNPhotos := func(t *testing.T, pb domain.Photobook, photoCount int) (page_id.PageID, []photo_id.PhotoID) {
		t.Helper()
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

	t.Run("正常_middle_photoでsplit", func(t *testing.T) {
		// 5 photo の page を photo[1] で split → source: photo[0..1] (2 件)、new: photo[2..4] (3 件)
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageID, phIDs := seedPageWithNPhotos(t, pb, 5)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewSplitPage(pool)
		out, err := uc.Execute(ctx, usecase.SplitPageInput{
			PhotobookID: pb.ID(), SourcePageID: pageID, SplitAtPhotoID: phIDs[1],
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if err != nil {
			t.Fatalf("SplitPage: %v", err)
		}

		// version +1
		pbAfter, _ := repo.FindByID(ctx, pb.ID())
		if pbAfter.Version() != pbBefore.Version()+1 {
			t.Errorf("version=%d want %d", pbAfter.Version(), pbBefore.Version()+1)
		}

		// pages: 2 件、source は display_order 0、new は 1
		pages, _ := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if len(pages) != 2 {
			t.Fatalf("page count=%d want 2", len(pages))
		}
		if pages[0].DisplayOrder().Int() != 0 || pages[1].DisplayOrder().Int() != 1 {
			t.Errorf("page orders=[%d,%d] want [0,1]", pages[0].DisplayOrder().Int(), pages[1].DisplayOrder().Int())
		}
		if !pages[0].ID().Equal(pageID) {
			t.Errorf("source page should still be at index 0")
		}
		if !pages[1].ID().Equal(out.NewPageID) {
			t.Errorf("new page should be at index 1")
		}

		// source page photos: phIDs[0], phIDs[1] が display_order 0,1
		sourcePhotos, _ := repo.ListPhotosByPageID(ctx, pageID)
		if len(sourcePhotos) != 2 {
			t.Fatalf("source photo count=%d want 2", len(sourcePhotos))
		}
		if sourcePhotos[0].DisplayOrder().Int() != 0 || sourcePhotos[1].DisplayOrder().Int() != 1 {
			t.Errorf("source photo orders=[%d,%d]", sourcePhotos[0].DisplayOrder().Int(), sourcePhotos[1].DisplayOrder().Int())
		}

		// new page photos: phIDs[2..4] が display_order 0,1,2
		newPhotos, _ := repo.ListPhotosByPageID(ctx, out.NewPageID)
		if len(newPhotos) != 3 {
			t.Fatalf("new photo count=%d want 3", len(newPhotos))
		}
		for i := 0; i < 3; i++ {
			if newPhotos[i].DisplayOrder().Int() != i {
				t.Errorf("new photo[%d] order=%d want %d", i, newPhotos[i].DisplayOrder().Int(), i)
			}
			if !newPhotos[i].ID().Equal(phIDs[2+i]) {
				t.Errorf("new photo[%d] id mismatch", i)
			}
		}
	})

	t.Run("正常_first_photoでsplit_sourceには1枚残り_newには残り", func(t *testing.T) {
		// 3 photo の page を photo[0] で split → source: photo[0] (1 件)、new: photo[1..2] (2 件)
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageID, phIDs := seedPageWithNPhotos(t, pb, 3)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewSplitPage(pool)
		out, err := uc.Execute(ctx, usecase.SplitPageInput{
			PhotobookID: pb.ID(), SourcePageID: pageID, SplitAtPhotoID: phIDs[0],
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if err != nil {
			t.Fatalf("SplitPage: %v", err)
		}
		sourcePhotos, _ := repo.ListPhotosByPageID(ctx, pageID)
		newPhotos, _ := repo.ListPhotosByPageID(ctx, out.NewPageID)
		if len(sourcePhotos) != 1 {
			t.Errorf("source photo count=%d want 1", len(sourcePhotos))
		}
		if len(newPhotos) != 2 {
			t.Errorf("new photo count=%d want 2", len(newPhotos))
		}
	})

	t.Run("異常_last_photoでsplitは新pageが空でreject", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageID, phIDs := seedPageWithNPhotos(t, pb, 3)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())
		uc := usecase.NewSplitPage(pool)
		_, err := uc.Execute(ctx, usecase.SplitPageInput{
			PhotobookID: pb.ID(), SourcePageID: pageID, SplitAtPhotoID: phIDs[2], // 末尾
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if !errors.Is(err, usecase.ErrSplitWouldCreateEmptyPage) {
			t.Errorf("err=%v want ErrSplitWouldCreateEmptyPage", err)
		}
	})

	t.Run("異常_30page_limit_到達でreject", func(t *testing.T) {
		// 既に 30 page ある photobook で split 不能
		// SetUp 重い → AddPage を 30 回呼んで limit に到達させる。1 page = 1 photo にして split 試行
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		// 最初の page には 2 photo を入れて split 候補にする
		page1ID, ph1IDs := seedPageWithNPhotos(t, pb, 2)
		// 残り 29 page を空 page として追加 (合計 30 page)
		repo := photobookrdb.NewPhotobookRepository(pool)
		addPage := usecase.NewAddPage(pool)
		for i := 0; i < 29; i++ {
			pbN, _ := repo.FindByID(ctx, pb.ID())
			if _, err := addPage.Execute(ctx, usecase.AddPageInput{
				PhotobookID: pb.ID(), ExpectedVersion: pbN.Version(), Now: now,
			}); err != nil {
				t.Fatalf("AddPage[%d]: %v", i, err)
			}
		}
		// 30 page 到達 → split しようとすると 31 page になるため reject
		pbBefore, _ := repo.FindByID(ctx, pb.ID())
		uc := usecase.NewSplitPage(pool)
		_, err := uc.Execute(ctx, usecase.SplitPageInput{
			PhotobookID: pb.ID(), SourcePageID: page1ID, SplitAtPhotoID: ph1IDs[0],
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if !errors.Is(err, domain.ErrPageLimitExceeded) {
			t.Errorf("err=%v want ErrPageLimitExceeded", err)
		}
	})

	t.Run("異常_version_conflict", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageID, phIDs := seedPageWithNPhotos(t, pb, 3)
		uc := usecase.NewSplitPage(pool)
		_, err := uc.Execute(ctx, usecase.SplitPageInput{
			PhotobookID: pb.ID(), SourcePageID: pageID, SplitAtPhotoID: phIDs[0],
			ExpectedVersion: pb.Version(), // 古い (AddPage / AddPhoto で +1 多数済)
			Now:             now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Errorf("err=%v want OptimisticLockConflict", err)
		}
	})

	t.Run("異常_split_at_photo_別pageの_photoはErrPhotoNotFound", func(t *testing.T) {
		// page A に photo X、page B に photo Y、SourcePageID=A、SplitAtPhotoID=Y → not found
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, _ := seedPageWithNPhotos(t, pb, 1)
		_, phB := seedPageWithNPhotos(t, pb, 1)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewSplitPage(pool)
		_, err := uc.Execute(ctx, usecase.SplitPageInput{
			PhotobookID: pb.ID(), SourcePageID: pageA, SplitAtPhotoID: phB[0],
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if !errors.Is(err, photobookrdb.ErrPhotoNotFound) {
			t.Errorf("err=%v want ErrPhotoNotFound", err)
		}
	})
}

// ============================================================================
// MovePhotoBetweenPages
// ============================================================================

func TestMovePhotoBetweenPages(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

	seedPageWithNPhotos := func(t *testing.T, pb domain.Photobook, photoCount int) (page_id.PageID, []photo_id.PhotoID) {
		t.Helper()
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

	t.Run("正常_別page_end_へ移動", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, phA := seedPageWithNPhotos(t, pb, 3)
		pageB, phB := seedPageWithNPhotos(t, pb, 2)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewMovePhotoBetweenPages(pool)
		// pageA[1] を pageB end に move
		if err := uc.Execute(ctx, usecase.MovePhotoBetweenPagesInput{
			PhotobookID: pb.ID(), PhotoID: phA[1], TargetPageID: pageB,
			Position: usecase.MovePositionEnd, ExpectedVersion: pbBefore.Version(), Now: now,
		}); err != nil {
			t.Fatalf("Move: %v", err)
		}
		// pageA: phA[0], phA[2] (詰め)、pageB: phB[0], phB[1], phA[1] (末尾)
		photosA, _ := repo.ListPhotosByPageID(ctx, pageA)
		if len(photosA) != 2 {
			t.Fatalf("pageA len=%d want 2", len(photosA))
		}
		if !photosA[0].ID().Equal(phA[0]) || !photosA[1].ID().Equal(phA[2]) {
			t.Errorf("pageA order incorrect after move")
		}
		photosB, _ := repo.ListPhotosByPageID(ctx, pageB)
		if len(photosB) != 3 {
			t.Fatalf("pageB len=%d want 3", len(photosB))
		}
		if !photosB[0].ID().Equal(phB[0]) || !photosB[1].ID().Equal(phB[1]) || !photosB[2].ID().Equal(phA[1]) {
			t.Errorf("pageB order incorrect after end-move (want phB[0], phB[1], phA[1])")
		}
		// version +1
		pbAfter, _ := repo.FindByID(ctx, pb.ID())
		if pbAfter.Version() != pbBefore.Version()+1 {
			t.Errorf("version=%d want %d", pbAfter.Version(), pbBefore.Version()+1)
		}
		// continuity check: 0..N-1
		for i, p := range photosA {
			if p.DisplayOrder().Int() != i {
				t.Errorf("pageA[%d] order=%d", i, p.DisplayOrder().Int())
			}
		}
		for i, p := range photosB {
			if p.DisplayOrder().Int() != i {
				t.Errorf("pageB[%d] order=%d", i, p.DisplayOrder().Int())
			}
		}
	})

	t.Run("正常_別page_start_へ移動", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		_, phA := seedPageWithNPhotos(t, pb, 3)
		pageB, phB := seedPageWithNPhotos(t, pb, 2)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewMovePhotoBetweenPages(pool)
		if err := uc.Execute(ctx, usecase.MovePhotoBetweenPagesInput{
			PhotobookID: pb.ID(), PhotoID: phA[2], TargetPageID: pageB,
			Position: usecase.MovePositionStart, ExpectedVersion: pbBefore.Version(), Now: now,
		}); err != nil {
			t.Fatalf("Move: %v", err)
		}
		photosB, _ := repo.ListPhotosByPageID(ctx, pageB)
		// pageB: phA[2] (start), phB[0], phB[1]
		if len(photosB) != 3 || !photosB[0].ID().Equal(phA[2]) || !photosB[1].ID().Equal(phB[0]) || !photosB[2].ID().Equal(phB[1]) {
			t.Errorf("pageB order incorrect after start-move")
		}
	})

	t.Run("正常_同page_start_reorder", func(t *testing.T) {
		// page に 3 photo、photo[2] を start に move → [photo[2], photo[0], photo[1]]
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageID, phs := seedPageWithNPhotos(t, pb, 3)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewMovePhotoBetweenPages(pool)
		if err := uc.Execute(ctx, usecase.MovePhotoBetweenPagesInput{
			PhotobookID: pb.ID(), PhotoID: phs[2], TargetPageID: pageID,
			Position: usecase.MovePositionStart, ExpectedVersion: pbBefore.Version(), Now: now,
		}); err != nil {
			t.Fatalf("Move (same-page start): %v", err)
		}
		photos, _ := repo.ListPhotosByPageID(ctx, pageID)
		if !photos[0].ID().Equal(phs[2]) || !photos[1].ID().Equal(phs[0]) || !photos[2].ID().Equal(phs[1]) {
			t.Errorf("same-page start: order incorrect")
		}
	})

	t.Run("正常_同page_end_reorder", func(t *testing.T) {
		// page に 3 photo、photo[0] を end に move → [photo[1], photo[2], photo[0]]
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageID, phs := seedPageWithNPhotos(t, pb, 3)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewMovePhotoBetweenPages(pool)
		if err := uc.Execute(ctx, usecase.MovePhotoBetweenPagesInput{
			PhotobookID: pb.ID(), PhotoID: phs[0], TargetPageID: pageID,
			Position: usecase.MovePositionEnd, ExpectedVersion: pbBefore.Version(), Now: now,
		}); err != nil {
			t.Fatalf("Move (same-page end): %v", err)
		}
		photos, _ := repo.ListPhotosByPageID(ctx, pageID)
		if !photos[0].ID().Equal(phs[1]) || !photos[1].ID().Equal(phs[2]) || !photos[2].ID().Equal(phs[0]) {
			t.Errorf("same-page end: order incorrect")
		}
	})

	t.Run("正常_source_pageが空になっても_pageは残る", func(t *testing.T) {
		// pageA に 1 photo、pageB に 1 photo。pageA の唯一の photo を pageB end に move → pageA は空
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageA, phA := seedPageWithNPhotos(t, pb, 1)
		pageB, _ := seedPageWithNPhotos(t, pb, 1)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewMovePhotoBetweenPages(pool)
		if err := uc.Execute(ctx, usecase.MovePhotoBetweenPagesInput{
			PhotobookID: pb.ID(), PhotoID: phA[0], TargetPageID: pageB,
			Position: usecase.MovePositionEnd, ExpectedVersion: pbBefore.Version(), Now: now,
		}); err != nil {
			t.Fatalf("Move: %v", err)
		}
		// pageA は空、pageB は 2 件
		photosA, _ := repo.ListPhotosByPageID(ctx, pageA)
		if len(photosA) != 0 {
			t.Errorf("pageA should be empty, got %d", len(photosA))
		}
		// pageA 自身は削除されていない
		pages, _ := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if len(pages) != 2 {
			t.Errorf("page count=%d want 2 (source page must remain)", len(pages))
		}
	})

	t.Run("異常_target_page_別photobook_は_ErrPageNotFound", func(t *testing.T) {
		// pbA に photo X、pbB に target page。pbA scope で move → not found
		truncateAll(t, pool)
		pbA := seedPhotobook(t, pool)
		pbB := seedPhotobook(t, pool)
		_, phA := seedPageWithNPhotos(t, pbA, 1)
		// pbB の page 用に AddPage を直接実行
		repo := photobookrdb.NewPhotobookRepository(pool)
		addPage := usecase.NewAddPage(pool)
		pbBOut, _ := addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pbB.ID(), ExpectedVersion: pbB.Version(), Now: now,
		})
		pbA1, _ := repo.FindByID(ctx, pbA.ID())

		uc := usecase.NewMovePhotoBetweenPages(pool)
		err := uc.Execute(ctx, usecase.MovePhotoBetweenPagesInput{
			PhotobookID: pbA.ID(), PhotoID: phA[0], TargetPageID: pbBOut.Page.ID(),
			Position: usecase.MovePositionEnd, ExpectedVersion: pbA1.Version(), Now: now,
		})
		if !errors.Is(err, photobookrdb.ErrPageNotFound) {
			t.Errorf("err=%v want ErrPageNotFound", err)
		}
	})

	t.Run("異常_invalid_position", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		pageID, phs := seedPageWithNPhotos(t, pb, 1)
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbBefore, _ := repo.FindByID(ctx, pb.ID())

		uc := usecase.NewMovePhotoBetweenPages(pool)
		err := uc.Execute(ctx, usecase.MovePhotoBetweenPagesInput{
			PhotobookID: pb.ID(), PhotoID: phs[0], TargetPageID: pageID,
			Position: usecase.MovePosition("middle"), // invalid
			ExpectedVersion: pbBefore.Version(), Now: now,
		})
		if !errors.Is(err, usecase.ErrInvalidMovePosition) {
			t.Errorf("err=%v want ErrInvalidMovePosition", err)
		}
	})

	t.Run("異常_version_conflict", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		_, phA := seedPageWithNPhotos(t, pb, 1)
		pageB, _ := seedPageWithNPhotos(t, pb, 1)
		uc := usecase.NewMovePhotoBetweenPages(pool)
		err := uc.Execute(ctx, usecase.MovePhotoBetweenPagesInput{
			PhotobookID: pb.ID(), PhotoID: phA[0], TargetPageID: pageB,
			Position: usecase.MovePositionEnd, ExpectedVersion: pb.Version(), Now: now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Errorf("err=%v want OptimisticLockConflict", err)
		}
	})
}
