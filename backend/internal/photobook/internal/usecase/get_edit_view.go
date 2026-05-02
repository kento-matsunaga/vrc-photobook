// get_edit_view: 編集画面用の read UseCase（PR27）。
//
// 設計参照:
//   - docs/plan/m2-frontend-edit-ui-fullspec-plan.md §4 / §6
//
// 振る舞い:
//  1. photobook を id で find（draft session middleware 通過後の前提）
//  2. status=draft 以外 → ErrEditNotAllowed（編集系は draft のみ）
//  3. pages → photos の順に取得し、image が available のもののみ display/thumbnail URL を返す
//  4. processing / failed の件数も返す（UI で「処理中 N 件」「失敗 N 件」表示）
//  5. 編集に必要な expected_version / settings を含めて返す
package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	imagedomain "vrcpb/backend/internal/image/domain"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// 編集系エラー（HTTP layer で 404 / 409 / 500 へ変換）。
var (
	// ErrEditPhotobookNotFound: photobook 不存在 / id 不一致。
	ErrEditPhotobookNotFound = errors.New("edit photobook not found")
	// ErrEditNotAllowed: status != draft（published / deleted / purged）。
	ErrEditNotAllowed = errors.New("edit not allowed (photobook is not in draft state)")
)

// EditViewExpiresIn は編集画面の variant URL 有効期限。
//
// PR25 と同じ 15 分。Frontend 側で有効期限切れ時に edit-view を再 fetch する。
const EditViewExpiresIn = 15 * time.Minute

// GetEditViewInput は UseCase の入力。
type GetEditViewInput struct {
	PhotobookID photobook_id.PhotobookID
}

// EditPhotoView は 1 写真分の view（編集画面用）。
type EditPhotoView struct {
	PhotoID      string
	ImageID      string
	DisplayOrder int
	Caption      *string
	Display      EditPresignedURLView
	Thumbnail    EditPresignedURLView
}

// EditPageView は 1 ページ分の view。
type EditPageView struct {
	PageID       string
	DisplayOrder int
	Caption      *string
	Photos       []EditPhotoView
}

// EditPresignedURLView は短命 GET URL の応答用 view。
type EditPresignedURLView struct {
	URL       string
	Width     int
	Height    int
	ExpiresAt time.Time
}

// EditImageView は photobook 内の 1 image の状態（attach 済か未配置かを問わない）。
//
// /prepare の reload 復元と進捗 UI で使う。authenticated API response 内部識別子としての
// imageId は許容するが、UI / DOM / log / report への露出は禁止（plan v2 §4.5 / §6.5）。
type EditImageView struct {
	ImageID          string     // UUID 文字列（client 内部 reconcile / attach request 用）
	Status           string     // "uploading" / "processing" / "available" / "failed"
	SourceFormat     string     // "jpg" / "png" / "webp"
	OriginalByteSize int64      // upload 直後の declared / verified bytes（0 = 不明）
	FailureReason    *string    // failed 時のみ。敵対者対策で具体値、Frontend で user-friendly に再 mapping
	CreatedAt        time.Time  // uploaded_at 相当
}

// EditViewSettings は編集画面で表示 / 編集する settings。
type EditViewSettings struct {
	Type         string
	Title        string
	Description  *string
	Layout       string
	OpeningStyle string
	Visibility   string
	CoverTitle   *string
}

// EditPhotobookView は編集画面用 view。
type EditPhotobookView struct {
	PhotobookID     string
	Status          string // 常に draft（編集可能なのは draft のみ）
	Version         int
	Settings        EditViewSettings
	CoverImageID    *string
	Cover           *EditVariantSet // cover_image_id があり available なら variant URL
	Pages           []EditPageView
	ProcessingCount int
	FailedCount     int
	// Images は photobook 内の全 active image を status 付きで返す（uploading / processing /
	// available / failed）。attach 済か unplaced かを問わず列挙し、/prepare の reload 復元と
	// progress UI 用 ground truth として使う（plan v2 §3.2 P0-b）。
	Images         []EditImageView
	DraftExpiresAt *time.Time
}

// EditVariantSet は display + thumbnail の URL set。
type EditVariantSet struct {
	Display   EditPresignedURLView
	Thumbnail EditPresignedURLView
}

// GetEditViewOutput は UseCase の出力。
type GetEditViewOutput struct {
	View EditPhotobookView
}

// GetEditView は編集画面の read UseCase。
type GetEditView struct {
	pool     *pgxpool.Pool
	r2Client r2.Client
}

// NewGetEditView は UseCase を組み立てる。
func NewGetEditView(pool *pgxpool.Pool, r2Client r2.Client) *GetEditView {
	return &GetEditView{pool: pool, r2Client: r2Client}
}

// Execute は photobook_id → edit view を返す。
func (u *GetEditView) Execute(ctx context.Context, in GetEditViewInput) (GetEditViewOutput, error) {
	pbRepo := rdb.NewPhotobookRepository(u.pool)
	pb, err := pbRepo.FindByID(ctx, in.PhotobookID)
	if err != nil {
		if errors.Is(err, rdb.ErrNotFound) {
			return GetEditViewOutput{}, ErrEditPhotobookNotFound
		}
		return GetEditViewOutput{}, fmt.Errorf("find by id: %w", err)
	}
	if !pb.Status().IsDraft() {
		return GetEditViewOutput{}, ErrEditNotAllowed
	}

	pages, err := pbRepo.ListPagesByPhotobookID(ctx, pb.ID())
	if err != nil {
		return GetEditViewOutput{}, fmt.Errorf("list pages: %w", err)
	}

	imgRepo := imagerdb.NewImageRepository(u.pool)
	allImages, err := imgRepo.ListActiveByPhotobookID(ctx, pb.ID())
	if err != nil {
		return GetEditViewOutput{}, fmt.Errorf("list images: %w", err)
	}
	processingCount := 0
	failedCount := 0
	editImages := make([]EditImageView, 0, len(allImages))
	for _, img := range allImages {
		switch {
		case img.IsProcessing(), img.IsUploading():
			processingCount++
		case img.IsFailed():
			failedCount++
		}
		var byteSize int64
		if bs := img.OriginalByteSize(); bs != nil {
			byteSize = bs.Int64()
		}
		var reason *string
		if r := img.FailureReason(); r != nil {
			s := r.String()
			reason = &s
		}
		editImages = append(editImages, EditImageView{
			ImageID:          img.ID().String(),
			Status:           img.Status().String(),
			SourceFormat:     img.SourceFormat().String(),
			OriginalByteSize: byteSize,
			FailureReason:    reason,
			CreatedAt:        img.UploadedAt(),
		})
	}

	editPages := make([]EditPageView, 0, len(pages))
	for _, page := range pages {
		photos, err := pbRepo.ListPhotosByPageID(ctx, page.ID())
		if err != nil {
			return GetEditViewOutput{}, fmt.Errorf("list photos: %w", err)
		}
		editPhotos := make([]EditPhotoView, 0, len(photos))
		for _, photo := range photos {
			img, err := imgRepo.FindByID(ctx, photo.ImageID())
			if err != nil {
				if errors.Is(err, imagerdb.ErrNotFound) {
					continue
				}
				return GetEditViewOutput{}, fmt.Errorf("find image: %w", err)
			}
			if !img.IsAvailable() {
				continue
			}
			set, err := u.buildVariantSet(ctx, img)
			if err != nil {
				return GetEditViewOutput{}, fmt.Errorf("build variants: %w", err)
			}
			if set == nil {
				continue
			}
			editPhotos = append(editPhotos, EditPhotoView{
				PhotoID:      photo.ID().String(),
				ImageID:      img.ID().String(),
				DisplayOrder: photo.DisplayOrder().Int(),
				Caption:      captionToPtr(photo.Caption()),
				Display:      set.Display,
				Thumbnail:    set.Thumbnail,
			})
		}
		editPages = append(editPages, EditPageView{
			PageID:       page.ID().String(),
			DisplayOrder: page.DisplayOrder().Int(),
			Caption:      captionToPtr(page.Caption()),
			Photos:       editPhotos,
		})
	}

	var coverID *string
	var coverSet *EditVariantSet
	if pb.CoverImageID() != nil {
		idStr := pb.CoverImageID().String()
		coverID = &idStr
		coverImg, err := imgRepo.FindByID(ctx, *pb.CoverImageID())
		if err != nil && !errors.Is(err, imagerdb.ErrNotFound) {
			return GetEditViewOutput{}, fmt.Errorf("find cover image: %w", err)
		}
		if err == nil && coverImg.IsAvailable() {
			coverSet, err = u.buildVariantSet(ctx, coverImg)
			if err != nil {
				return GetEditViewOutput{}, fmt.Errorf("build cover variants: %w", err)
			}
		}
	}

	view := EditPhotobookView{
		PhotobookID: pb.ID().String(),
		Status:      pb.Status().String(),
		Version:     pb.Version(),
		Settings: EditViewSettings{
			Type:         pb.Type().String(),
			Title:        pb.Title(),
			Description:  pb.Description(),
			Layout:       pb.Layout().String(),
			OpeningStyle: pb.OpeningStyle().String(),
			Visibility:   pb.Visibility().String(),
			CoverTitle:   pb.CoverTitle(),
		},
		CoverImageID:    coverID,
		Cover:           coverSet,
		Pages:           editPages,
		ProcessingCount: processingCount,
		FailedCount:     failedCount,
		Images:          editImages,
		DraftExpiresAt:  pb.DraftExpiresAt(),
	}
	return GetEditViewOutput{View: view}, nil
}

// buildVariantSet は available image から display + thumbnail の presigned URL を作る。
func (u *GetEditView) buildVariantSet(ctx context.Context, img imagedomain.Image) (*EditVariantSet, error) {
	display, ok := findVariant(img, "display")
	if !ok {
		return nil, nil
	}
	thumb, ok := findVariant(img, "thumbnail")
	if !ok {
		return nil, nil
	}
	displayURL, err := u.r2Client.PresignGetObject(ctx, r2.PresignGetInput{
		StorageKey: display.StorageKey().String(),
		ExpiresIn:  EditViewExpiresIn,
	})
	if err != nil {
		return nil, fmt.Errorf("presign display: %w", err)
	}
	thumbURL, err := u.r2Client.PresignGetObject(ctx, r2.PresignGetInput{
		StorageKey: thumb.StorageKey().String(),
		ExpiresIn:  EditViewExpiresIn,
	})
	if err != nil {
		return nil, fmt.Errorf("presign thumbnail: %w", err)
	}
	return &EditVariantSet{
		Display: EditPresignedURLView{
			URL: displayURL.URL, Width: display.Dimensions().Width(), Height: display.Dimensions().Height(),
			ExpiresAt: displayURL.ExpiresAt,
		},
		Thumbnail: EditPresignedURLView{
			URL: thumbURL.URL, Width: thumb.Dimensions().Width(), Height: thumb.Dimensions().Height(),
			ExpiresAt: thumbURL.ExpiresAt,
		},
	}, nil
}

func findVariant(img imagedomain.Image, kindName string) (imagedomain.ImageVariant, bool) {
	for _, v := range img.Variants() {
		if v.Kind().String() == kindName {
			return v, true
		}
	}
	return imagedomain.ImageVariant{}, false
}

func captionToPtr(c *caption.Caption) *string {
	if c == nil {
		return nil
	}
	s := c.String()
	return &s
}
