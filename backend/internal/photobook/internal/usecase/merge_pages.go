// STOP P-3: MergePages UseCase。
//
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §3.4.4 / §4 / §5.5 / §5.13
// API spec:
//
//	POST /api/photobooks/{id}/pages/{pageId}/merge-into/{targetPageId}
//	{ "expected_version": number } -> EditView (B 方式)
//
// 振る舞い:
//   - source page (= pageId) の全 photo を target page の末尾に追加
//   - source page 自身を削除 (CASCADE は走らないが、photo は事前に move 済のため安全)
//   - source 削除後、source 以降の page の display_order を 1 つ繰り上げ (gap 防止)
//   - source の caption / page_meta は捨てられる (target を保持、UI で警告 modal を出す前提)
//   - photobook.version は同一 TX で +1 (1 度きり)
//
// edge case:
//   - source == target → ErrMergeIntoSelf (5.5)
//   - photobook に page が 1 件しかない (sole page を merge source) → ErrCannotRemoveLastPage (5.13)
//   - photo 上限は MVP では設けない (§5.6 通り、merge で page 内 photo が増えても reject しない)
//
// アルゴリズム (1 TX、bumpVersion 1 度きり):
//  1. BumpVersion (OCC + draft check)
//  2. self-merge check (source == target)
//  3. List all pages / source / target ownership / display_order 取得 / sole page check
//  4. List source photos / target photos
//  5. source の photo を escape し target 末尾に append:
//     - BulkOffsetPhotoOrdersOnPage(source) で source 全 photo を +1000 escape
//     - target は escape 不要 (target 末尾の display_order = len(target) は空き)
//     - source 各 photo を UpdatePhotoPageAndOrder で target に move、新 order = len(target)+i
//  6. DeletePage(source) で source page を削除 (photo は既に target に move 済)
//  7. source 以降の page を 1 つ繰り上げ:
//     - BulkOffsetPagesInPhotobook で全 page +1000 escape
//     - 各 page を新 order に書き戻し:
//     oldOrder < sourceOrder: そのまま
//     oldOrder > sourceOrder: oldOrder - 1
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// ErrMergeIntoSelf は source page と target page が同一の場合に返す。
// handler 層で 409 + reason `merge_into_self` に mapping する。
var ErrMergeIntoSelf = errors.New("merge source and target are the same page")

// ErrCannotRemoveLastPage は photobook に page が 1 件しかない状態で merge を試みた場合に返す。
// 集約不変条件 (photobook には 1 page 以上必要) を守るため defensive で reject する。
// handler 層で 409 + reason `cannot_remove_last_page` に mapping する。
var ErrCannotRemoveLastPage = errors.New("cannot remove the last remaining page")

// MergePagesInput は MergePages UseCase の入力。
type MergePagesInput struct {
	PhotobookID     photobook_id.PhotobookID
	SourcePageID    page_id.PageID
	TargetPageID    page_id.PageID
	ExpectedVersion int
	Now             time.Time
}

// MergePages は draft Photobook の source page を target page にマージする UseCase。
type MergePages struct {
	pool *pgxpool.Pool
}

// NewMergePages は UseCase を組み立てる。
func NewMergePages(pool *pgxpool.Pool) *MergePages {
	return &MergePages{pool: pool}
}

// Execute は MergePages の本体。詳細は file header の「アルゴリズム」を参照。
func (u *MergePages) Execute(ctx context.Context, in MergePagesInput) error {
	// self-merge を TX 開始前に reject (副作用なし、bumpVersion も発生させない)
	if in.SourcePageID.Equal(in.TargetPageID) {
		return ErrMergeIntoSelf
	}

	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)

		// 1. version+1 + draft check (1 度きり)
		if err := repo.BumpVersion(ctx, in.PhotobookID, in.ExpectedVersion, in.Now); err != nil {
			return err
		}

		// 2. 全 page list + source / target ownership / display_order 取得
		pages, err := repo.ListPagesByPhotobookID(ctx, in.PhotobookID)
		if err != nil {
			return err
		}
		if len(pages) <= 1 {
			// sole page (or 0 page) を merge source にできない (5.13)
			return ErrCannotRemoveLastPage
		}
		sourceOrder := -1
		targetFound := false
		for _, p := range pages {
			if p.ID().Equal(in.SourcePageID) {
				sourceOrder = p.DisplayOrder().Int()
			}
			if p.ID().Equal(in.TargetPageID) {
				targetFound = true
			}
		}
		if sourceOrder == -1 {
			return photobookrdb.ErrPageNotFound
		}
		if !targetFound {
			return photobookrdb.ErrPageNotFound
		}

		// 3. source / target photos list
		sourcePhotos, err := repo.ListPhotosByPageID(ctx, in.SourcePageID)
		if err != nil {
			return err
		}
		targetPhotos, err := repo.ListPhotosByPageID(ctx, in.TargetPageID)
		if err != nil {
			return err
		}

		// 4. source の photo を target 末尾に append
		//    target は escape 不要 (新 order = len(target) + i は target 既存 photo の order と
		//    衝突しない)。source は escape して photo を移動させる必要あり。
		if len(sourcePhotos) > 0 {
			if err := repo.BulkOffsetPhotoOrdersOnPage(ctx, in.SourcePageID); err != nil {
				return err
			}
			baseOrder := len(targetPhotos)
			for i, ph := range sourcePhotos {
				ord, err := display_order.New(baseOrder + i)
				if err != nil {
					return err
				}
				if err := repo.UpdatePhotoPageAndOrder(ctx, in.PhotobookID, ph.ID(), in.TargetPageID, ord); err != nil {
					return err
				}
			}
		}

		// 5. source page を削除 (photo は既に target へ move 済、CASCADE で残 photo / meta が
		//    自動削除される。photo は move 済なので 0 件のはず)
		if err := repo.DeletePage(ctx, in.PhotobookID, in.SourcePageID); err != nil {
			return err
		}

		// 6. source 削除後、source 以降の page (oldOrder > sourceOrder) を 1 つ繰り上げ
		//    BulkOffsetPagesInPhotobook で残 page 全体を +1000 escape し、
		//    各 page を新 order に書き戻す。
		if err := repo.BulkOffsetPagesInPhotobook(ctx, in.PhotobookID, in.Now); err != nil {
			return err
		}
		for _, p := range pages {
			if p.ID().Equal(in.SourcePageID) {
				continue // 削除済
			}
			origOrder := p.DisplayOrder().Int()
			var nextInt int
			if origOrder < sourceOrder {
				nextInt = origOrder
			} else {
				// origOrder > sourceOrder (origOrder == sourceOrder は source 自身、上で除外済)
				nextInt = origOrder - 1
			}
			next, err := display_order.New(nextInt)
			if err != nil {
				return err
			}
			if err := repo.UpdatePageOrder(ctx, p.ID(), next, in.Now); err != nil {
				return err
			}
		}

		return nil
	})
}
