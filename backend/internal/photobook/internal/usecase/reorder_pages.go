// STOP P-3: ReorderPages UseCase。
//
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §3.4.5 / §4 / §5.11
// API spec:
//
//	PATCH /api/photobooks/{id}/pages/reorder
//	{
//	  "assignments": [{ "page_id": <p>, "display_order": <n> }, ...],
//	  "expected_version": number
//	} -> EditView (B 方式)
//
// 振る舞い:
//   - assignments は当該 photobook の **全 page** を含む必要あり (部分 reorder 不可)
//   - display_order は 0..N-1 の重複 / 欠番なし permutation
//   - photobook.version は同一 TX で +1 (1 度きり)
//
// アルゴリズム (1 TX、bumpVersion 1 度きり):
//  1. assignments の構造的検証 (空 / 重複 / 0..N-1 permutation)
//  2. BumpVersion (OCC + draft check)
//  3. ListPagesByPhotobookID で全 page 取得 / assignments と page_id set の一致を検証
//  4. BulkOffsetPagesInPhotobook で全 page +1000 escape
//  5. assignments の各 entry を UpdatePageOrder で書き戻し
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

// ErrInvalidReorderAssignments は assignments が photobook の全 page と一致しない、または
// display_order が 0..N-1 の permutation でない場合に返す。handler 層で 400 に mapping する
// (reason `invalid_reorder_assignments`)。
var ErrInvalidReorderAssignments = errors.New("invalid reorder assignments")

// ReorderPagesAssignment は 1 page 分の reorder 指示。
type ReorderPagesAssignment struct {
	PageID       page_id.PageID
	DisplayOrder display_order.DisplayOrder
}

// ReorderPagesInput は ReorderPages UseCase の入力。
type ReorderPagesInput struct {
	PhotobookID     photobook_id.PhotobookID
	Assignments     []ReorderPagesAssignment
	ExpectedVersion int
	Now             time.Time
}

// ReorderPages は draft Photobook の page display_order を一括再採番する UseCase。
type ReorderPages struct {
	pool *pgxpool.Pool
}

// NewReorderPages は UseCase を組み立てる。
func NewReorderPages(pool *pgxpool.Pool) *ReorderPages {
	return &ReorderPages{pool: pool}
}

// Execute は ReorderPages の本体。詳細は file header の「アルゴリズム」を参照。
func (u *ReorderPages) Execute(ctx context.Context, in ReorderPagesInput) error {
	// 1. assignments 内部の構造的検証 (TX 開始前、副作用なし)
	if err := validateReorderAssignments(in.Assignments); err != nil {
		return err
	}

	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)

		// 2. version+1 + draft check (1 度きり)
		if err := repo.BumpVersion(ctx, in.PhotobookID, in.ExpectedVersion, in.Now); err != nil {
			return err
		}

		// 3. 全 page list + assignments と page_id set 一致確認
		pages, err := repo.ListPagesByPhotobookID(ctx, in.PhotobookID)
		if err != nil {
			return err
		}
		if len(pages) != len(in.Assignments) {
			return ErrInvalidReorderAssignments
		}
		// page_id set 一致 (重複は validateReorderAssignments で確認済)
		pageIDSet := make(map[[16]byte]struct{}, len(pages))
		for _, p := range pages {
			pageIDSet[p.ID().UUID()] = struct{}{}
		}
		for _, a := range in.Assignments {
			if _, ok := pageIDSet[a.PageID.UUID()]; !ok {
				return ErrInvalidReorderAssignments
			}
		}

		// 4. 全 page +1000 escape
		if err := repo.BulkOffsetPagesInPhotobook(ctx, in.PhotobookID, in.Now); err != nil {
			return err
		}

		// 5. assignments の各 entry を新 display_order に書き戻し
		for _, a := range in.Assignments {
			if err := repo.UpdatePageOrder(ctx, a.PageID, a.DisplayOrder, in.Now); err != nil {
				return err
			}
		}

		return nil
	})
}

// validateReorderAssignments は assignments の内部構造を検証する。
//
//   - 空でないこと
//   - page_id 重複なし
//   - display_order が 0..N-1 の permutation (重複・欠番なし)
//
// photobook の page との一致は外側 (TX 内) で確認する。
func validateReorderAssignments(assignments []ReorderPagesAssignment) error {
	n := len(assignments)
	if n == 0 {
		return ErrInvalidReorderAssignments
	}
	pageIDSeen := make(map[[16]byte]struct{}, n)
	orderSeen := make(map[int]struct{}, n)
	for _, a := range assignments {
		if _, dup := pageIDSeen[a.PageID.UUID()]; dup {
			return ErrInvalidReorderAssignments
		}
		pageIDSeen[a.PageID.UUID()] = struct{}{}

		ord := a.DisplayOrder.Int()
		if ord < 0 || ord >= n {
			return ErrInvalidReorderAssignments
		}
		if _, dup := orderSeen[ord]; dup {
			return ErrInvalidReorderAssignments
		}
		orderSeen[ord] = struct{}{}
	}
	return nil
}
