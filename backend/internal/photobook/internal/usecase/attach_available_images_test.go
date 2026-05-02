// AttachAvailableImages usecase の実 DB integration test。
//
// 設計参照: docs/plan/m2-prepare-resilience-and-throughput-plan.md §3.4 / §5
//
// 実行方法（既存 photobook_edit_test.go と同じ）:
//   docker compose -f backend/docker-compose.yaml up -d postgres
//   export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//   go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//   go -C backend test ./internal/photobook/internal/usecase/...
//
// 観点（user 指示 sub-step 2-8 完了条件）:
//   1. available + unattached が attach される
//   2. processing は attach されない（SQL の status='available' 条件）
//   3. failed は attach されない（同上）
//   4. attach 済 image は idempotent skip される（NOT EXISTS 条件）
//   5. 0 件は idempotent 成功、version bump なし
//   6. 21 枚で page が 2 ページに分割される（P-1 仕様、20 枚で page 区切り）
//   7. 既存末尾 page に空きがある場合はそこから埋める
//   8. OCC 衝突時に rollback され、page/photo が残らない
//   9. partial attach（途中失敗）で全 rollback、半端 attach なし

package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

const truncateAllAttachTablesSQL = "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"

func truncateForAttach(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), truncateAllAttachTablesSQL); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
}

func seedDraftPhotobookForAttach(t *testing.T, pool *pgxpool.Pool) domain.Photobook {
	t.Helper()
	pb := photobooktests.NewPhotobookBuilder().Build(t)
	repo := photobookrdb.NewPhotobookRepository(pool)
	if err := repo.CreateDraft(context.Background(), pb); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	return pb
}

func seedAvailableImageForAttach(
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

func seedProcessingImageForAttach(
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

// seedPageWithDisplayOrder は raw SQL で photobook_pages を直接 INSERT する（test fixture 専用）。
//
// 用途: AttachAvailableImages の partial-rollback test で、display_order の飛び地状態を
// 作り、loop が新 page 作成時に UNIQUE INDEX `(photobook_id, display_order)` 衝突を
// deterministic に起こすため。production code パスは経由しない。
func seedPageWithDisplayOrder(
	t *testing.T,
	pool *pgxpool.Pool,
	photobookID photobook_id.PhotobookID,
	displayOrder int,
) {
	t.Helper()
	now := time.Now().UTC()
	pgID, err := page_id.New()
	if err != nil {
		t.Fatalf("page_id.New: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO photobook_pages (id, photobook_id, display_order, caption, created_at, updated_at)
		 VALUES ($1, $2, $3, NULL, $4, $4)`,
		pgtype.UUID{Bytes: pgID.UUID(), Valid: true},
		pgtype.UUID{Bytes: photobookID.UUID(), Valid: true},
		int32(displayOrder),
		pgtype.Timestamptz{Time: now, Valid: true},
	); err != nil {
		t.Fatalf("seed page direct INSERT: %v", err)
	}
}

// seedFailedImageForAttach は image を failed 状態にして seed する（attach 対象外検証用）。
//
// 実装注: image domain は MarkFailed メソッドを持つが、repository に対応 method があるか
// 確認していないため、本 test では SQL UPDATE で直接 status='failed' に書き換える簡易方式
// （test fixture 用途、production code パスは触らない）。
func seedFailedImageForAttach(
	t *testing.T,
	pool *pgxpool.Pool,
	ownerID photobook_id.PhotobookID,
) {
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
	// status='failed' に直接 UPDATE（test fixture 用、production パスは経由しない）
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx,
		"UPDATE images SET status='failed', failed_at=$1, failure_reason='decode_failed' WHERE id=$2",
		pgtype.Timestamptz{Time: now, Valid: true},
		pgtype.UUID{Bytes: img.ID().UUID(), Valid: true}); err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}
}

func TestAttachAvailableImages(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 2, 13, 0, 0, 0, time.UTC)

	t.Run("正常_available_unattached_3件がattachされversionが1bumpされる", func(t *testing.T) {
		truncateForAttach(t, pool)
		pb := seedDraftPhotobookForAttach(t, pool)
		for i := 0; i < 3; i++ {
			_ = seedAvailableImageForAttach(t, pool, pb.ID())
		}

		uc := usecase.NewAttachAvailableImages(pool)
		out, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.AttachedCount != 3 {
			t.Errorf("AttachedCount = %d, want 3", out.AttachedCount)
		}
		if out.PageCount != 1 {
			t.Errorf("PageCount = %d, want 1", out.PageCount)
		}
		if out.SkippedCount != 0 {
			t.Errorf("SkippedCount = %d, want 0", out.SkippedCount)
		}
		repo := photobookrdb.NewPhotobookRepository(pool)
		updated, err := repo.FindByID(ctx, pb.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if updated.Version() != pb.Version()+1 {
			t.Errorf("version = %d, want %d (1 度だけ bump)", updated.Version(), pb.Version()+1)
		}
		remaining, err := repo.ListAvailableUnattachedImageIDs(ctx, pb.ID())
		if err != nil {
			t.Fatalf("ListAvailableUnattachedImageIDs: %v", err)
		}
		if len(remaining) != 0 {
			t.Errorf("unattached remaining = %d, want 0", len(remaining))
		}
	})

	t.Run("正常_processing_image_はattachされない", func(t *testing.T) {
		truncateForAttach(t, pool)
		pb := seedDraftPhotobookForAttach(t, pool)
		_ = seedProcessingImageForAttach(t, pool, pb.ID())

		uc := usecase.NewAttachAvailableImages(pool)
		out, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.AttachedCount != 0 {
			t.Errorf("AttachedCount = %d, want 0 (processing は対象外)", out.AttachedCount)
		}
		repo := photobookrdb.NewPhotobookRepository(pool)
		updated, _ := repo.FindByID(ctx, pb.ID())
		if updated.Version() != pb.Version() {
			t.Errorf("0 件 attach で version bump されている (%d → %d)", pb.Version(), updated.Version())
		}
	})

	t.Run("正常_failed_image_はattachされない", func(t *testing.T) {
		truncateForAttach(t, pool)
		pb := seedDraftPhotobookForAttach(t, pool)
		seedFailedImageForAttach(t, pool, pb.ID())

		uc := usecase.NewAttachAvailableImages(pool)
		out, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.AttachedCount != 0 {
			t.Errorf("AttachedCount = %d, want 0 (failed は対象外)", out.AttachedCount)
		}
	})

	t.Run("正常_attach済_imageは2回目で0件idempotent", func(t *testing.T) {
		truncateForAttach(t, pool)
		pb := seedDraftPhotobookForAttach(t, pool)
		_ = seedAvailableImageForAttach(t, pool, pb.ID())

		uc := usecase.NewAttachAvailableImages(pool)
		out1, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("Execute 1: %v", err)
		}
		if out1.AttachedCount != 1 {
			t.Fatalf("first AttachedCount = %d, want 1", out1.AttachedCount)
		}
		// 2 回目: 同じ photobook、新規 available なし → 0 件 idempotent
		out2, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version() + 1, // 1 回目で 1 bump
			Now:             now,
		})
		if err != nil {
			t.Fatalf("Execute 2: %v", err)
		}
		if out2.AttachedCount != 0 {
			t.Errorf("second AttachedCount = %d, want 0 (idempotent skip)", out2.AttachedCount)
		}
		if out2.PageCount != 1 {
			t.Errorf("PageCount = %d, want 1", out2.PageCount)
		}
		// 2 回目で version bump なし（0 件 idempotent）
		repo := photobookrdb.NewPhotobookRepository(pool)
		updated, _ := repo.FindByID(ctx, pb.ID())
		if updated.Version() != pb.Version()+1 {
			t.Errorf("second call で version 追加 bump (%d → %d、+1 までで止まるべき)",
				pb.Version(), updated.Version())
		}
	})

	t.Run("正常_0件は成功でversion_bumpされない", func(t *testing.T) {
		truncateForAttach(t, pool)
		pb := seedDraftPhotobookForAttach(t, pool)

		uc := usecase.NewAttachAvailableImages(pool)
		out, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.AttachedCount != 0 || out.PageCount != 0 || out.SkippedCount != 0 {
			t.Errorf("counts = %+v, want all 0", out)
		}
		repo := photobookrdb.NewPhotobookRepository(pool)
		updated, _ := repo.FindByID(ctx, pb.ID())
		if updated.Version() != pb.Version() {
			t.Errorf("0 件 attach で version bumped (%d → %d)", pb.Version(), updated.Version())
		}
	})

	t.Run("正常_21枚で2ページに分割される", func(t *testing.T) {
		truncateForAttach(t, pool)
		pb := seedDraftPhotobookForAttach(t, pool)
		for i := 0; i < 21; i++ {
			_ = seedAvailableImageForAttach(t, pool, pb.ID())
		}

		uc := usecase.NewAttachAvailableImages(pool)
		out, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.AttachedCount != 21 {
			t.Errorf("AttachedCount = %d, want 21", out.AttachedCount)
		}
		if out.PageCount != 2 {
			t.Errorf("PageCount = %d, want 2 (20 枚 + 1 枚)", out.PageCount)
		}
	})

	t.Run("正常_既存末尾pageに空きがあればそこから埋める", func(t *testing.T) {
		truncateForAttach(t, pool)
		pb := seedDraftPhotobookForAttach(t, pool)
		// 1 回目: 5 枚 attach（1 page 目に 5 photo）
		for i := 0; i < 5; i++ {
			_ = seedAvailableImageForAttach(t, pool, pb.ID())
		}
		uc := usecase.NewAttachAvailableImages(pool)
		out1, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("Execute 1: %v", err)
		}
		if out1.PageCount != 1 {
			t.Fatalf("PageCount after first = %d, want 1", out1.PageCount)
		}
		// 2 回目: 10 枚追加 available（合計 5+10=15、1 page 容量 20 内）
		for i := 0; i < 10; i++ {
			_ = seedAvailableImageForAttach(t, pool, pb.ID())
		}
		out2, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version() + 1, // 1 回目で 1 bump
			Now:             now,
		})
		if err != nil {
			t.Fatalf("Execute 2: %v", err)
		}
		if out2.AttachedCount != 10 {
			t.Errorf("second AttachedCount = %d, want 10", out2.AttachedCount)
		}
		if out2.PageCount != 1 {
			t.Errorf("PageCount = %d, want 1 (末尾 page に詰める、新 page 不要)", out2.PageCount)
		}
	})

	t.Run("異常_OCC衝突_versionミスマッチでrollbackされ何もattachされない", func(t *testing.T) {
		truncateForAttach(t, pool)
		pb := seedDraftPhotobookForAttach(t, pool)
		_ = seedAvailableImageForAttach(t, pool, pb.ID())
		_ = seedAvailableImageForAttach(t, pool, pb.ID())

		uc := usecase.NewAttachAvailableImages(pool)
		_, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version() + 99,
			Now:             now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Fatalf("err = %v, want ErrOptimisticLockConflict", err)
		}
		// rollback 確認: page も photo も残らない、available unattached は依然 2 件
		repo := photobookrdb.NewPhotobookRepository(pool)
		pages, _ := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if len(pages) != 0 {
			t.Errorf("OCC rollback で page 残っている (count=%d)", len(pages))
		}
		remaining, _ := repo.ListAvailableUnattachedImageIDs(ctx, pb.ID())
		if len(remaining) != 2 {
			t.Errorf("OCC rollback で attach 残存 (unattached=%d, want 2)", len(remaining))
		}
		updated, _ := repo.FindByID(ctx, pb.ID())
		if updated.Version() != pb.Version() {
			t.Errorf("OCC rollback で version 変動 (%d → %d、不変のはず)",
				pb.Version(), updated.Version())
		}
	})

	t.Run("異常_loop途中SQL違反でpartial_rollback_全attach巻き戻る", func(t *testing.T) {
		// シナリオ: display_order=1 の page を直接 SQL で seed しておき（飛び地）、
		// AttachAvailableImages の loop が新 page を display_order=1 (= len(pages)=1)
		// で作ろうとして UNIQUE INDEX `(photobook_id, display_order)` 違反を起こす。
		// 21 image attach の途中で SQL error → TX rollback → 全 attach 巻き戻り。
		//
		// 期待挙動:
		//   - Execute は non-nil error を返す
		//   - photobook_photos 0 件（loop で attach した 20 photo すべて rollback）
		//   - photobook_pages は既存の 1 件のみ（loop で作ろうとした 2nd page も rollback）
		//   - photobook version 不変（BumpVersion 未到達）
		//   - available unattached 21 件維持（attach されていない）
		truncateForAttach(t, pool)
		pb := seedDraftPhotobookForAttach(t, pool)
		// 飛び地 page (display_order=1) を seed
		seedPageWithDisplayOrder(t, pool, pb.ID(), 1)
		for i := 0; i < 21; i++ {
			_ = seedAvailableImageForAttach(t, pool, pb.ID())
		}

		uc := usecase.NewAttachAvailableImages(pool)
		_, err := uc.Execute(ctx, usecase.AttachAvailableImagesInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err == nil {
			t.Fatalf("Execute: expected error (UNIQUE INDEX 違反による rollback)、got nil")
		}
		// rollback 検証
		repo := photobookrdb.NewPhotobookRepository(pool)
		// pages: pre-seed の 1 件のみ（loop 内で作ろうとした 2nd page は rollback）
		pages, err := repo.ListPagesByPhotobookID(ctx, pb.ID())
		if err != nil {
			t.Fatalf("ListPagesByPhotobookID: %v", err)
		}
		if len(pages) != 1 {
			t.Errorf("partial rollback で page 残存 (count=%d, want 1: pre-seed のみ)", len(pages))
		}
		// photos: 0 件（loop 内で attach した photo すべて rollback）
		var photoCount int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM photobook_photos pp
			 JOIN photobook_pages pg ON pp.page_id = pg.id
			 WHERE pg.photobook_id = $1`,
			pgtype.UUID{Bytes: pb.ID().UUID(), Valid: true},
		).Scan(&photoCount); err != nil {
			t.Fatalf("count photos: %v", err)
		}
		if photoCount != 0 {
			t.Errorf("partial rollback で photo 残存 (count=%d, want 0)", photoCount)
		}
		// version 不変
		updated, _ := repo.FindByID(ctx, pb.ID())
		if updated.Version() != pb.Version() {
			t.Errorf("partial rollback で version 変動 (%d → %d、不変のはず)",
				pb.Version(), updated.Version())
		}
		// unattached 21 件維持
		remaining, _ := repo.ListAvailableUnattachedImageIDs(ctx, pb.ID())
		if len(remaining) != 21 {
			t.Errorf("partial rollback で attach 残存 (unattached=%d, want 21)", len(remaining))
		}
	})
}
