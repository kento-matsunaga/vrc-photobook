// STOP P-2: SplitPage UseCase。
//
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §3.4.2 / §4 / §5 / §11
// API spec:
//
//	POST /api/photobooks/{id}/pages/{pageId}/split
//	{ "photo_id": string, "expected_version": number } -> EditView (B 方式)
//
// 振る舞い:
//   - source page の指定 photo "の次" から末尾までを新 page に分離
//   - 新 page は source page の display_order の **直後** に挿入
//   - 後続 page の display_order は 0..N の連続を保つ (1 つずつシフト)
//   - source page / 新 page の photo display_order は 0..N-1 に再採番
//   - 30 page 上限到達時は domain.ErrPageLimitExceeded
//   - 切断点 photo が source page 末尾 → ErrSplitWouldCreateEmptyPage
//   - 切断点 photo が source page 配下にない → ErrPhotoNotFound
//
// アルゴリズム (1 TX、bumpVersion は 1 度きり):
//  1. BumpVersion (OCC + draft check)
//  2. 30 page 上限事前 check
//  3. List source photos / find split index / edge case 判定
//  4. List all pages / source page の display_order 取得 / ownership check
//  5. BulkOffsetPagesInPhotobook で全 page +1000 escape
//  6. CreatePageInTx で新 page を sourceOrder+1 に INSERT
//  7. 既存 page を新 order に書き戻し:
//     - oldOrder <= sourceOrder: そのまま
//     - oldOrder >  sourceOrder: oldOrder + 1
//  8. BulkOffsetPhotoOrdersOnPage で source page 全 photo を +1000 escape
//  9. 切断点以降の photo を UpdatePhotoPageAndOrder で新 page に move、display_order 0..M-k-2
//  10. 切断点以前の photo は UpdatePhotoOrder で 0..k に書き戻し
package usecase

import (
	"context"
	"errors"
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

// ErrSplitWouldCreateEmptyPage は split で空 page を作ろうとした (切断点が source page 末尾、
// または source page が空) 時に返す。handler 層で 409 reason `split_would_create_empty_page`
// に mapping する。
var ErrSplitWouldCreateEmptyPage = errors.New("split would create empty page")

// SplitPageInput は SplitPage UseCase の入力。
type SplitPageInput struct {
	PhotobookID     photobook_id.PhotobookID
	SourcePageID    page_id.PageID
	SplitAtPhotoID  photo_id.PhotoID // この photo の "次" から新 page に分離
	ExpectedVersion int
	Now             time.Time
}

// SplitPageOutput は SplitPage の出力 (handler が改めて GetEditView を呼ぶため、新 pageID
// のみ返す)。
type SplitPageOutput struct {
	NewPageID page_id.PageID
}

// SplitPage は draft Photobook の指定 page を 2 つに分ける UseCase。
type SplitPage struct {
	pool *pgxpool.Pool
}

// NewSplitPage は UseCase を組み立てる。
func NewSplitPage(pool *pgxpool.Pool) *SplitPage {
	return &SplitPage{pool: pool}
}

// Execute は SplitPage の本体。詳細は file header の「アルゴリズム」を参照。
func (u *SplitPage) Execute(ctx context.Context, in SplitPageInput) (SplitPageOutput, error) {
	var out SplitPageOutput
	err := database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)

		// 1. version+1 + draft check (1 度きり)
		if err := repo.BumpVersion(ctx, in.PhotobookID, in.ExpectedVersion, in.Now); err != nil {
			return err
		}

		// 2. 30 page 上限 check (split 後 N+1 になるため、現在 N >= 30 で reject)
		count, err := repo.CountPagesByPhotobookID(ctx, in.PhotobookID)
		if err != nil {
			return err
		}
		if count >= domain.MaxPagesPerPhotobook {
			return domain.ErrPageLimitExceeded
		}

		// 3. source page の photo を取得 (display_order ASC)
		photos, err := repo.ListPhotosByPageID(ctx, in.SourcePageID)
		if err != nil {
			return err
		}
		if len(photos) == 0 {
			// 空 source page を split しようとした (通常 UI は出さないが defensive)
			return ErrSplitWouldCreateEmptyPage
		}

		// 4. 切断点 photo を検出
		splitIdx := -1
		for i, ph := range photos {
			if ph.ID().Equal(in.SplitAtPhotoID) {
				splitIdx = i
				break
			}
		}
		if splitIdx == -1 {
			return photobookrdb.ErrPhotoNotFound
		}

		// 5. 切断点が source page 末尾なら新 page が空になるため reject
		if splitIdx == len(photos)-1 {
			return ErrSplitWouldCreateEmptyPage
		}

		// 6. 全 page list + source page ownership / display_order 取得
		pages, err := repo.ListPagesByPhotobookID(ctx, in.PhotobookID)
		if err != nil {
			return err
		}
		sourceOrder := -1
		for _, p := range pages {
			if p.ID().Equal(in.SourcePageID) {
				sourceOrder = p.DisplayOrder().Int()
				break
			}
		}
		if sourceOrder == -1 {
			// source page が photobook 配下にない (photo は配下だが page が違う photobook?
			// 実際には photo の page_id が source なので photo lookup で既に検出済の想定)
			return photobookrdb.ErrPageNotFound
		}

		// 7. BulkOffsetPagesInPhotobook (全 page +1000 escape)
		if err := repo.BulkOffsetPagesInPhotobook(ctx, in.PhotobookID, in.Now); err != nil {
			return err
		}

		// 8. 新 page を sourceOrder + 1 に INSERT (CreatePageInTx は version bump しない)
		newPID, err := page_id.New()
		if err != nil {
			return err
		}
		newPageOrder, err := display_order.New(sourceOrder + 1)
		if err != nil {
			return err
		}
		newPage, err := domain.NewPage(domain.NewPageParams{
			ID:           newPID,
			PhotobookID:  in.PhotobookID,
			DisplayOrder: newPageOrder,
			Caption:      nil,
			Now:          in.Now,
		})
		if err != nil {
			return err
		}
		if err := repo.CreatePageInTx(ctx, newPage); err != nil {
			return err
		}

		// 9. 既存 page を新 order に書き戻し
		//    - oldOrder <= sourceOrder: そのまま
		//    - oldOrder >  sourceOrder: oldOrder + 1
		for _, p := range pages {
			origOrder := p.DisplayOrder().Int()
			var nextInt int
			if origOrder <= sourceOrder {
				nextInt = origOrder
			} else {
				nextInt = origOrder + 1
			}
			next, err := display_order.New(nextInt)
			if err != nil {
				return err
			}
			if err := repo.UpdatePageOrder(ctx, p.ID(), next, in.Now); err != nil {
				return err
			}
		}

		// 10. source page の photos を +1000 escape
		if err := repo.BulkOffsetPhotoOrdersOnPage(ctx, in.SourcePageID); err != nil {
			return err
		}

		// 11. 切断点以前 (0..splitIdx) は source page に残し、display_order 0..splitIdx で書き戻し
		for i := 0; i <= splitIdx; i++ {
			ord, err := display_order.New(i)
			if err != nil {
				return err
			}
			if err := repo.UpdatePhotoOrder(ctx, photos[i].ID(), ord); err != nil {
				return err
			}
		}

		// 12. 切断点以降 (splitIdx+1..M-1) は 新 page に move + display_order 0..M-splitIdx-2
		for i := splitIdx + 1; i < len(photos); i++ {
			ord, err := display_order.New(i - splitIdx - 1)
			if err != nil {
				return err
			}
			if err := repo.UpdatePhotoPageAndOrder(ctx, in.PhotobookID, photos[i].ID(), newPID, ord); err != nil {
				return err
			}
		}

		out = SplitPageOutput{NewPageID: newPID}
		return nil
	})
	if err != nil {
		return SplitPageOutput{}, err
	}
	return out, nil
}
