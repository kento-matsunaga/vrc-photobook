// get_manage_photobook: 管理ページ用の read UseCase。
//
// 設計参照:
//   - docs/plan/m2-public-viewer-and-manage-plan.md §4
//   - 業務知識 v4 §6 manage URL
//
// 振る舞い:
//  1. id を VO で検証
//  2. photobook を find（status を問わず）
//  3. 公開 URL（slug が発行済みなら組み立てる）/ 状態 / 画像数 / 管理メタを返す
//
// 認可は HTTP middleware（RequireManageSession）で担保。本 UseCase は middleware 通過後
// に呼ばれる前提で id 一致確認のみ行う。
//
// セキュリティ:
//   - manage_url_token / draft_edit_token / hash 値は応答に含めない
//   - manage URL の再送経路は ADR-0006（email provider 再選定中）の決着後に検討。
//     MVP は publish 完了画面での 1 回表示が標準で、再発行 URL を view に含めない
package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// 管理ページ系エラー（HTTP layer で 404 / 500 へ変換）。
var (
	// ErrManageNotFound: photobook が存在しない / id 不一致。
	// 認可エラー（Cookie 不在 / photobook_id 不一致）は middleware が 401 を返すため
	// 本 UseCase に到達した時点で id 一致は前提。仮に id が parse 不能などで再 lookup
	// が失敗したら ErrManageNotFound に集約する。
	ErrManageNotFound = errors.New("manage photobook not found")
)

// GetManagePhotobookInput は UseCase の入力。
type GetManagePhotobookInput struct {
	PhotobookID photobook_id.PhotobookID
}

// ManagePhotobookView は管理ページ用 read view。
type ManagePhotobookView struct {
	PhotobookID           string
	Type                  string
	Title                 string
	Status                string // draft / published / deleted
	Visibility            string
	HiddenByOperator      bool
	PublicURLSlug         *string // null if not yet published
	PublicURLPath         *string // "/p/{slug}" 形式、未発行は null
	PublishedAt           *time.Time
	DeletedAt             *time.Time
	DraftExpiresAt        *time.Time
	ManageURLTokenVersion int
	AvailableImageCount   int
}

// GetManagePhotobookOutput は UseCase の出力。
type GetManagePhotobookOutput struct {
	View ManagePhotobookView
}

// GetManagePhotobook は管理ページの read UseCase。
type GetManagePhotobook struct {
	pool *pgxpool.Pool
}

// NewGetManagePhotobook は UseCase を組み立てる。
func NewGetManagePhotobook(pool *pgxpool.Pool) *GetManagePhotobook {
	return &GetManagePhotobook{pool: pool}
}

// Execute は photobook_id → manage view を返す。
func (u *GetManagePhotobook) Execute(
	ctx context.Context,
	in GetManagePhotobookInput,
) (GetManagePhotobookOutput, error) {
	pbRepo := rdb.NewPhotobookRepository(u.pool)
	pb, err := pbRepo.FindByID(ctx, in.PhotobookID)
	if err != nil {
		if errors.Is(err, rdb.ErrNotFound) {
			return GetManagePhotobookOutput{}, ErrManageNotFound
		}
		return GetManagePhotobookOutput{}, fmt.Errorf("find by id: %w", err)
	}

	// purged は管理ページからも 404
	if pb.Status().IsPurged() {
		return GetManagePhotobookOutput{}, ErrManageNotFound
	}

	imgRepo := imagerdb.NewImageRepository(u.pool)
	images, err := imgRepo.ListActiveByPhotobookID(ctx, pb.ID())
	if err != nil {
		return GetManagePhotobookOutput{}, fmt.Errorf("list images: %w", err)
	}
	availableCount := 0
	for _, img := range images {
		if img.IsAvailable() {
			availableCount++
		}
	}

	var publicSlug *string
	var publicPath *string
	if s := pb.PublicUrlSlug(); s != nil {
		v := s.String()
		publicSlug = &v
		path := "/p/" + v
		publicPath = &path
	}

	view := ManagePhotobookView{
		PhotobookID:           pb.ID().String(),
		Type:                  pb.Type().String(),
		Title:                 pb.Title(),
		Status:                pb.Status().String(),
		Visibility:            pb.Visibility().String(),
		HiddenByOperator:      pb.HiddenByOperator(),
		PublicURLSlug:         publicSlug,
		PublicURLPath:         publicPath,
		PublishedAt:           pb.PublishedAt(),
		DeletedAt:             pb.DeletedAt(),
		DraftExpiresAt:        pb.DraftExpiresAt(),
		ManageURLTokenVersion: pb.ManageUrlTokenVersion().Int(),
		AvailableImageCount:   availableCount,
	}
	return GetManagePhotobookOutput{View: view}, nil
}
