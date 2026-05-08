// STOP P-2: MovePhotoBetweenPages UseCase。
//
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §3.4.3 / §4 / §5 / §11
// API spec:
//
//	PATCH /api/photobooks/{id}/photos/{photoId}/move
//	{ "target_page_id": string, "position": "start"|"end", "expected_version": number }
//	-> EditView (B 方式)
//
// 振る舞い:
//   - draft only / expected_version 必須
//   - MVP では target_display_order 数値ではなく start / end のみ
//   - source page から photo を抜き、display_order を 0..N-2 に詰める
//   - target page の先頭 (start) または末尾 (end) に挿入
//   - 同 page move (source == target) も start / end として処理 (内部 reorder と等価)
//   - source page が空になっても page 自体は削除しない (caller の責務外)
//
// アルゴリズム:
//
//	[1 TX、bumpVersion 1 度きり]
//	1. BumpVersion
//	2. FindPhotoWithPhotobookID で photo + photobook ownership check
//	3. (cross page の場合) target page の photobook ownership check
//	4. List source / target photos
//	5. cross page の場合:
//	     - source 全 photos を BulkOffsetPhotoOrdersOnPage で +1000 escape
//	     - target 全 photos を別 BulkOffsetPhotoOrdersOnPage で +1000 escape
//	     - UpdatePhotoPageAndOrder で photo を target に move (新 display_order = 0 or len(target))
//	     - source 残 photos を 0..M-2 に書き戻し (除く moved photo)
//	     - target 既存 photos を:
//	         start: 1..L に書き戻し (moved photo は 0)
//	         end:   0..L-1 に書き戻し (moved photo は L)
//	   same page の場合:
//	     - 全 photos を BulkOffsetPhotoOrdersOnPage で +1000 escape (1 度のみ)
//	     - 各 photo を新 display_order に書き戻し:
//	         start: moved=0, others shift +1 (before moved) or stay (after moved)
//	         end:   moved=M-1, others stay (before) or shift -1 (after)
//	     UpdatePhotoOrder のみ使用 (page_id 不変なので UpdatePhotoPageAndOrder は不要)
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

// MovePosition は move 先の位置 (MVP では start / end のみ、中間挿入は Phase B+)。
type MovePosition string

const (
	MovePositionStart MovePosition = "start"
	MovePositionEnd   MovePosition = "end"
)

// ErrInvalidMovePosition は position が start / end 以外の場合に返す。handler 層で 400。
var ErrInvalidMovePosition = errors.New("invalid move position (must be start or end)")

// MovePhotoBetweenPagesInput は MovePhotoBetweenPages UseCase の入力。
type MovePhotoBetweenPagesInput struct {
	PhotobookID     photobook_id.PhotobookID
	PhotoID         photo_id.PhotoID
	TargetPageID    page_id.PageID
	Position        MovePosition
	ExpectedVersion int
	Now             time.Time
}

// MovePhotoBetweenPages は draft Photobook 内で photo を別 page に移動する UseCase。
type MovePhotoBetweenPages struct {
	pool *pgxpool.Pool
}

// NewMovePhotoBetweenPages は UseCase を組み立てる。
func NewMovePhotoBetweenPages(pool *pgxpool.Pool) *MovePhotoBetweenPages {
	return &MovePhotoBetweenPages{pool: pool}
}

// Execute は move を実行する。
func (u *MovePhotoBetweenPages) Execute(ctx context.Context, in MovePhotoBetweenPagesInput) error {
	// position validation (handler 層でも check するが defensive)
	if in.Position != MovePositionStart && in.Position != MovePositionEnd {
		return ErrInvalidMovePosition
	}

	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)

		// 1. bumpVersion (OCC + draft check、1 度きり)
		if err := repo.BumpVersion(ctx, in.PhotobookID, in.ExpectedVersion, in.Now); err != nil {
			return err
		}

		// 2. photo + photobook ownership 確認 (FindPhotoWithPhotobookID)
		photo, photoPhotobookID, err := repo.FindPhotoWithPhotobookID(ctx, in.PhotoID)
		if err != nil {
			return err
		}
		if !photoPhotobookID.Equal(in.PhotobookID) {
			return photobookrdb.ErrPhotoNotFound
		}
		sourcePageID := photo.PageID()

		// 3. source / target page の photos を取得
		sourcePhotos, err := repo.ListPhotosByPageID(ctx, sourcePageID)
		if err != nil {
			return err
		}
		// sourcePhotos に photo が含まれているはず (FindPhotoWithPhotobookID で確認済) → 念のため index 検出
		sourceIdx := -1
		for i, p := range sourcePhotos {
			if p.ID().Equal(in.PhotoID) {
				sourceIdx = i
				break
			}
		}
		if sourceIdx == -1 {
			// race などで source list と photo の page_id が分離した場合の defensive
			return photobookrdb.ErrPhotoNotFound
		}

		samePage := sourcePageID.Equal(in.TargetPageID)

		if samePage {
			// === 同 page reorder ===
			if err := u.reorderSamePage(ctx, repo, sourcePageID, sourcePhotos, sourceIdx, in.Position); err != nil {
				return err
			}
			return nil
		}

		// === cross page move ===
		// 4. target page の photobook ownership 確認 (UpdatePhotoPageAndOrder が内部で
		//    検証するが、target photos list 取得時に一度だけ確認するほうが query 数が少ない)
		targetPhotos, err := repo.ListPhotosByPageID(ctx, in.TargetPageID)
		if err != nil {
			return err
		}
		// target page が photobook 配下か検証は UpdatePhotoPageAndOrder でも行うが、
		// 事前の List に依存して 0 件 (page 不存在 or 別 photobook の page) ケースを
		// 区別する: 0 件は別 photobook かも、存在チェックを別 query で行う必要がある。
		// → UpdatePhotoPageAndOrder の内部 ownership check が ErrPageNotFound を返すので
		//    そちらに委ねる。

		// 5. source / target を別々に escape (+1000)
		if err := repo.BulkOffsetPhotoOrdersOnPage(ctx, sourcePageID); err != nil {
			return err
		}
		if err := repo.BulkOffsetPhotoOrdersOnPage(ctx, in.TargetPageID); err != nil {
			return err
		}

		// 6. moved photo を target に move (display_order は escape 後 0 / len(target) のどちらも空)
		var newTargetOrderInt int
		if in.Position == MovePositionStart {
			newTargetOrderInt = 0
		} else {
			newTargetOrderInt = len(targetPhotos) // end
		}
		newTargetOrder, err := display_order.New(newTargetOrderInt)
		if err != nil {
			return err
		}
		// UpdatePhotoPageAndOrder は内部で photo + target page の photobook ownership 検証を実施
		if err := repo.UpdatePhotoPageAndOrder(ctx, in.PhotobookID, in.PhotoID, in.TargetPageID, newTargetOrder); err != nil {
			return err
		}

		// 7. source 残 photos を詰めて 0..M-2 に書き戻し (除く moved photo)
		nextSourceOrder := 0
		for i, p := range sourcePhotos {
			if i == sourceIdx {
				continue // moved photo は target に移動済
			}
			ord, err := display_order.New(nextSourceOrder)
			if err != nil {
				return err
			}
			if err := repo.UpdatePhotoOrder(ctx, p.ID(), ord); err != nil {
				return err
			}
			nextSourceOrder++
		}

		// 8. target 既存 photos を書き戻し
		//    start: 既存 0..L-1 を 1..L にシフト (moved=0)
		//    end:   既存 0..L-1 を 0..L-1 のまま (moved=L)
		for i, p := range targetPhotos {
			var nextInt int
			if in.Position == MovePositionStart {
				nextInt = i + 1
			} else {
				nextInt = i // end: 既存 photo は order 不変、moved が末尾 (L) を取る
			}
			ord, err := display_order.New(nextInt)
			if err != nil {
				return err
			}
			if err := repo.UpdatePhotoOrder(ctx, p.ID(), ord); err != nil {
				return err
			}
		}

		return nil
	})
}

// reorderSamePage は same-page move (source == target) の inner logic。1 回 escape して
// 全 photo を新しい display_order に書き戻す。page_id は不変なので UpdatePhotoOrder のみ。
func (u *MovePhotoBetweenPages) reorderSamePage(
	ctx context.Context,
	repo *photobookrdb.PhotobookRepository,
	pageID page_id.PageID,
	sourcePhotos []domain.Photo,
	sourceIdx int,
	position MovePosition,
) error {
	// escape 全 photos +1000
	if err := repo.BulkOffsetPhotoOrdersOnPage(ctx, pageID); err != nil {
		return err
	}
	m := len(sourcePhotos)
	for i, p := range sourcePhotos {
		var nextInt int
		switch position {
		case MovePositionStart:
			if i == sourceIdx {
				nextInt = 0
			} else if i < sourceIdx {
				nextInt = i + 1
			} else {
				// i > sourceIdx
				nextInt = i
			}
		case MovePositionEnd:
			if i == sourceIdx {
				nextInt = m - 1
			} else if i < sourceIdx {
				nextInt = i
			} else {
				// i > sourceIdx
				nextInt = i - 1
			}
		}
		ord, err := display_order.New(nextInt)
		if err != nil {
			return err
		}
		if err := repo.UpdatePhotoOrder(ctx, p.ID(), ord); err != nil {
			return err
		}
	}
	return nil
}

