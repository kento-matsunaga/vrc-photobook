// attach_available_images: /prepare の「編集へ進む」で呼ばれる bulk attach UseCase。
//
// 設計参照: docs/plan/m2-prepare-resilience-and-throughput-plan.md §3.4 / §5
//
// 役割（user 指示の必須条件、plan v2 §6 / §7）:
//   - 同一 TX 内で photobook FOR UPDATE → status=draft + version 検証 → available 未 attach
//     image_id 一覧取得 → page 不足分作成 → photo bulk INSERT → version+1 を 1 度だけ実行
//   - 途中失敗で半端に attach されない（atomicity 保証、AddPhoto を per-image で呼ばず
//     CreatePageInTx / CreatePhotoInTx で INSERT のみ + 末尾で BumpVersion 1 回）
//   - status='available' 以外（uploading / processing / failed / deleted / purged）は
//     ListAvailableUnattachedImageIDs SQL で除外済（呼び出し側で skip ロジック不要）
//   - 既に attach 済の image は NOT EXISTS で除外済 → idempotent
//   - 0 件は idempotent 成功（version は bump しない、何も変えていない）
//   - request body に image_id 配列を要求しない（server ground truth から取得）
//   - response は count のみ（raw image_id / page_id / photo_id を返さない）
//
// page 分割は P-1（plan §5.4）: 末尾 page から 20 枚まで埋め、超過分は新 page を作成。

package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// MaxPhotosPerPageForAttach は 1 page あたりの photo 上限。20 枚で page 分割する（plan §5.4 P-1）。
const MaxPhotosPerPageForAttach = 20

// AttachAvailableImagesInput は UseCase の入力。
type AttachAvailableImagesInput struct {
	PhotobookID     photobook_id.PhotobookID
	ExpectedVersion int
	Now             time.Time
}

// AttachAvailableImagesOutput は count-only の集計結果。
//
// raw image_id / page_id / photo_id は含めない。Frontend は本 result の count のみを用い、
// 詳細表示は GET /edit-view の images / pages で取得する（reload 復元 + ground truth）。
type AttachAvailableImagesOutput struct {
	// AttachedCount は本 call で新規 attach された photo 数（= INSERT された photobook_photos 行数）。
	AttachedCount int
	// PageCount は本 call 完了後の photobook の総 page 数（既存 page 数 + 本 call で作成した page 数）。
	PageCount int
	// SkippedCount は「server に available があるが何らかの理由で attach 対象外」の件数。
	// 現状の実装では SQL で除外済のため常に 0（attach 済 image は NOT EXISTS で除外）。
	// 将来的に「20 page 超で attach 不能」等が発生したら埋める領域。
	SkippedCount int
}

// AttachAvailableImages は draft photobook の available 未 attach image を一括 attach する UseCase。
type AttachAvailableImages struct {
	pool *pgxpool.Pool
}

// NewAttachAvailableImages は UseCase を組み立てる。
func NewAttachAvailableImages(pool *pgxpool.Pool) *AttachAvailableImages {
	return &AttachAvailableImages{pool: pool}
}

// Execute は同一 TX 内で全 attach 操作を実行する。途中失敗時は rollback で何も commit
// されない（partial attach 不発生、user 指示 §5）。
func (u *AttachAvailableImages) Execute(
	ctx context.Context,
	in AttachAvailableImagesInput,
) (AttachAvailableImagesOutput, error) {
	var out AttachAvailableImagesOutput
	err := database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)

		// 1) photobook 取得 + status=draft + version 検証
		pb, err := repo.FindByID(ctx, in.PhotobookID)
		if err != nil {
			if errors.Is(err, photobookrdb.ErrNotFound) {
				return ErrEditPhotobookNotFound
			}
			return fmt.Errorf("find photobook: %w", err)
		}
		if !pb.Status().IsDraft() {
			return ErrEditNotAllowed
		}
		if pb.Version() != in.ExpectedVersion {
			return photobookrdb.ErrOptimisticLockConflict
		}

		// 2) available 未 attach image_id 一覧（SQL 側で status / unattached を保証）
		imageIDs, err := repo.ListAvailableUnattachedImageIDs(ctx, in.PhotobookID)
		if err != nil {
			return fmt.Errorf("list unattached: %w", err)
		}

		// 3) 既存 pages 取得（page 分割計算 + total count 集計用）
		pages, err := repo.ListPagesByPhotobookID(ctx, in.PhotobookID)
		if err != nil {
			return fmt.Errorf("list pages: %w", err)
		}

		// 0 件 idempotent: 何も変更せず成功で返す（version も bump しない）
		if len(imageIDs) == 0 {
			out.AttachedCount = 0
			out.PageCount = len(pages)
			out.SkippedCount = 0
			return nil
		}

		// 4) 末尾 page の状態取得（既存 page 末尾に 20 枚未満なら詰める、満杯なら新 page）
		var lastPageID *page_id.PageID
		var lastPageCount int
		if len(pages) > 0 {
			last := pages[len(pages)-1]
			id := last.ID()
			lastPageID = &id
			cnt, err := repo.CountPhotosByPageID(ctx, last.ID())
			if err != nil {
				return fmt.Errorf("count photos last page: %w", err)
			}
			lastPageCount = cnt
		}

		// 5) attach loop（同一 TX 内、途中失敗で全部 rollback）
		nextPageOrder := len(pages)
		attachedCount := 0
		pagesCreated := 0
		for _, imgID := range imageIDs {
			// 末尾 page が無い or 満杯 → 新 page 作成（version bump せず INSERT のみ）
			if lastPageID == nil || lastPageCount >= MaxPhotosPerPageForAttach {
				newPID, err := page_id.New()
				if err != nil {
					return fmt.Errorf("new page id: %w", err)
				}
				pgOrder, err := display_order.New(nextPageOrder)
				if err != nil {
					return fmt.Errorf("new page order: %w", err)
				}
				newPage, err := domain.NewPage(domain.NewPageParams{
					ID:           newPID,
					PhotobookID:  in.PhotobookID,
					DisplayOrder: pgOrder,
					Caption:      nil,
					Now:          in.Now,
				})
				if err != nil {
					return fmt.Errorf("new page: %w", err)
				}
				if err := repo.CreatePageInTx(ctx, newPage); err != nil {
					return fmt.Errorf("create page: %w", err)
				}
				lastPageID = &newPID
				lastPageCount = 0
				nextPageOrder++
				pagesCreated++
			}

			// photo 作成（version bump せず INSERT のみ、image owner / status は SQL で保証済）
			newPhotoID, err := photo_id.New()
			if err != nil {
				return fmt.Errorf("new photo id: %w", err)
			}
			photoOrder, err := display_order.New(lastPageCount)
			if err != nil {
				return fmt.Errorf("new photo order: %w", err)
			}
			photo, err := domain.NewPhoto(domain.NewPhotoParams{
				ID:           newPhotoID,
				PageID:       *lastPageID,
				ImageID:      imgID,
				DisplayOrder: photoOrder,
				Caption:      nil,
				Now:          in.Now,
			})
			if err != nil {
				return fmt.Errorf("new photo: %w", err)
			}
			if err := repo.CreatePhotoInTx(ctx, photo); err != nil {
				return fmt.Errorf("create photo: %w", err)
			}
			lastPageCount++
			attachedCount++
		}

		// 6) version 1 度だけ bump（loop 内で都度 bump せず、最後に 1 回）
		if err := repo.BumpVersion(ctx, in.PhotobookID, in.ExpectedVersion, in.Now); err != nil {
			// 並列 update で OCC 衝突 → ErrOptimisticLockConflict、上位で 409 にマップ
			return err
		}

		out.AttachedCount = attachedCount
		out.PageCount = len(pages) + pagesCreated
		out.SkippedCount = 0
		return nil
	})
	if err != nil {
		return AttachAvailableImagesOutput{}, err
	}
	return out, nil
}
