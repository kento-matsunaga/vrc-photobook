// Package wireup は cmd/ops / 他 cmd から Moderation UseCase を取得する facade。
//
// `internal/moderation/internal/usecase` は moderation サブツリーからのみ import 可能なため、
// cmd/ops が必要とする入出力型 / エラー sentinel は本パッケージで re-export する。
package wireup

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/moderation/internal/usecase"
)

// 入出力型 re-export（cmd/ops から見える形にする）。
type (
	HideInput        = usecase.HideInput
	HideOutput       = usecase.HideOutput
	UnhideInput      = usecase.UnhideInput
	UnhideOutput    = usecase.UnhideOutput
	GetForOpsInput   = usecase.GetForOpsInput
	GetForOpsOutput  = usecase.GetForOpsOutput
	ListHiddenInput  = usecase.ListHiddenInput
	ListHiddenOutput = usecase.ListHiddenOutput
)

// エラー sentinel re-export（cmd/ops が errors.Is で識別できるように）。
var (
	ErrPhotobookNotFound    = usecase.ErrPhotobookNotFound
	ErrInvalidStatusForHide = usecase.ErrInvalidStatusForHide
	ErrAlreadyHidden        = usecase.ErrAlreadyHidden
	ErrAlreadyUnhidden      = usecase.ErrAlreadyUnhidden
)

// Handlers は cmd/ops が使う UseCase 群の facade。
//
// 各 UseCase の参照型は外部に公開しないため、Execute 経由で呼び出す薄い wrapper を
// 持つ。
type Handlers struct {
	hide       *usecase.HidePhotobookByOperator
	unhide     *usecase.UnhidePhotobookByOperator
	show       *usecase.GetPhotobookForOps
	listHidden *usecase.ListHiddenPhotobooks
}

// BuildHandlers は pool から Moderation UseCase 群を組み立てる。
// pool が nil なら nil を返す（呼び出し側で endpoint / cmd 自体を出さない判断）。
func BuildHandlers(pool *pgxpool.Pool) *Handlers {
	if pool == nil {
		return nil
	}
	return &Handlers{
		hide:       usecase.NewHidePhotobookByOperator(pool),
		unhide:     usecase.NewUnhidePhotobookByOperator(pool),
		show:       usecase.NewGetPhotobookForOps(pool),
		listHidden: usecase.NewListHiddenPhotobooks(pool),
	}
}

// Hide は HidePhotobookByOperator を実行する。
func (h *Handlers) Hide(ctx context.Context, in HideInput) (HideOutput, error) {
	return h.hide.Execute(ctx, in)
}

// Unhide は UnhidePhotobookByOperator を実行する。
func (h *Handlers) Unhide(ctx context.Context, in UnhideInput) (UnhideOutput, error) {
	return h.unhide.Execute(ctx, in)
}

// Show は GetPhotobookForOps を実行する。
func (h *Handlers) Show(ctx context.Context, in GetForOpsInput) (GetForOpsOutput, error) {
	return h.show.Execute(ctx, in)
}

// ListHidden は ListHiddenPhotobooks を実行する。
func (h *Handlers) ListHidden(ctx context.Context, in ListHiddenInput) (ListHiddenOutput, error) {
	return h.listHidden.Execute(ctx, in)
}
