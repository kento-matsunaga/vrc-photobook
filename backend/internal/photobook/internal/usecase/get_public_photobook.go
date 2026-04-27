// get_public_photobook: 公開 Viewer 用の read UseCase。
//
// 設計参照:
//   - docs/plan/m2-public-viewer-and-manage-plan.md §3 / §5 / §6
//   - 業務知識 v4 §2.3 / §3.2
//
// 振る舞い:
//  1. slug を VO で検証
//  2. permissive lookup（status / hidden / visibility は問わず）
//  3. status / hidden / visibility に応じて 200 / 410 / 404 相当のエラーを返す
//  4. pages → photos の順に取得し、image が available のものだけを採用
//  5. 各 photo の display / thumbnail variant を短命 presigned GET URL に変換
//
// セキュリティ:
//   - 失敗理由は外に漏らさない（draft / deleted / private を区別しない）
//   - storage_key 完全値は応答に出さない（presigned URL のみ）
//   - presigned URL / storage_key は logs に出さない
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
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// PublicViewExpiresIn は公開 Viewer 用 presigned URL の有効期限。
//
// plan §5.3 で 15 分。Frontend 側の自動 refresh は PR41+ で扱う。
const PublicViewExpiresIn = 15 * time.Minute

// 公開 Viewer 系エラー（HTTP layer で 404 / 410 / 500 へ変換）。
var (
	// ErrPublicNotFound: slug 不存在 / status=draft / status=deleted / status=purged /
	// visibility=private のいずれか。**外部に区別を漏らさない** ために 1 種類に集約。
	ErrPublicNotFound = errors.New("public photobook not found")

	// ErrPublicGone: status=published かつ hidden_by_operator=true。
	// 410 Gone を返すための sentinel。MVP UI は 404 と同等の表示で問題ないが、
	// 将来 PR35 (Report) 以降で運営対応の文脈を伝える余地を残す。
	ErrPublicGone = errors.New("public photobook gone")
)

// GetPublicPhotobookInput は UseCase の入力。
type GetPublicPhotobookInput struct {
	RawSlug string
}

// PublicPhotobookView は API レスポンスに対応する read view。
type PublicPhotobookView struct {
	Type               string
	Title              string
	Description        *string
	Layout             string
	OpeningStyle       string
	CreatorDisplayName string
	CreatorXID         *string
	CoverTitle         *string
	Cover              *PublicVariantSet // cover image があれば variant URL 群、無ければ nil
	PublishedAt        time.Time
	Pages              []PublicPageView
}

// PublicPageView は 1 ページ分の view。
type PublicPageView struct {
	Caption *string
	Photos  []PublicPhotoView
}

// PublicPhotoView は 1 写真分の view。
type PublicPhotoView struct {
	Caption  *string
	Variants PublicVariantSet
}

// PublicVariantSet は display / thumbnail の URL 群。
type PublicVariantSet struct {
	Display   PresignedURLView
	Thumbnail PresignedURLView
}

// PresignedURLView は短命 GET URL の応答用 view。
type PresignedURLView struct {
	URL       string
	Width     int
	Height    int
	ExpiresAt time.Time
}

// GetPublicPhotobookOutput は UseCase の出力。
type GetPublicPhotobookOutput struct {
	View PublicPhotobookView
}

// GetPublicPhotobook は公開 Viewer の read UseCase。
//
// 集約をまたぐ Query（photobook + image）。同一 TX は不要（read のみ、整合性は最終的）。
type GetPublicPhotobook struct {
	pool     *pgxpool.Pool
	r2Client r2.Client
}

// NewGetPublicPhotobook は UseCase を組み立てる。
func NewGetPublicPhotobook(pool *pgxpool.Pool, r2Client r2.Client) *GetPublicPhotobook {
	return &GetPublicPhotobook{pool: pool, r2Client: r2Client}
}

// Execute は slug → public view を返す。
func (u *GetPublicPhotobook) Execute(
	ctx context.Context,
	in GetPublicPhotobookInput,
) (GetPublicPhotobookOutput, error) {
	// 1) slug 形式検証
	s, err := slug.Parse(in.RawSlug)
	if err != nil {
		// 不正な slug 形式は外部に「Not Found」だけを返す
		return GetPublicPhotobookOutput{}, ErrPublicNotFound
	}

	// 2) permissive lookup
	pbRepo := rdb.NewPhotobookRepository(u.pool)
	pb, err := pbRepo.FindAnyBySlug(ctx, s)
	if err != nil {
		if errors.Is(err, rdb.ErrNotFound) {
			return GetPublicPhotobookOutput{}, ErrPublicNotFound
		}
		return GetPublicPhotobookOutput{}, fmt.Errorf("find by slug: %w", err)
	}

	// 3) 公開条件の判定
	if err := assessPublicVisibility(pb); err != nil {
		return GetPublicPhotobookOutput{}, err
	}
	if pb.PublishedAt() == nil {
		// invariant: published 状態なら published_at は必須。データ不整合は 500 系扱い。
		return GetPublicPhotobookOutput{}, fmt.Errorf("published_at missing for published photobook")
	}

	// 4) pages + photos
	pages, err := pbRepo.ListPagesByPhotobookID(ctx, pb.ID())
	if err != nil {
		return GetPublicPhotobookOutput{}, fmt.Errorf("list pages: %w", err)
	}

	imgRepo := imagerdb.NewImageRepository(u.pool)

	publicPages := make([]PublicPageView, 0, len(pages))
	for _, page := range pages {
		photos, err := pbRepo.ListPhotosByPageID(ctx, page.ID())
		if err != nil {
			return GetPublicPhotobookOutput{}, fmt.Errorf("list photos: %w", err)
		}
		publicPhotos := make([]PublicPhotoView, 0, len(photos))
		for _, photo := range photos {
			img, err := imgRepo.FindByID(ctx, photo.ImageID())
			if err != nil {
				if errors.Is(err, imagerdb.ErrNotFound) {
					// 参照先 image が消えている場合は表示から除外（DB 不整合の可能性、
					// 観測点で警告すべきだが Viewer は壊さない）
					continue
				}
				return GetPublicPhotobookOutput{}, fmt.Errorf("find image: %w", err)
			}
			if !img.IsAvailable() {
				// processing / failed / deleted は表示しない
				continue
			}
			set, err := u.buildVariantSet(ctx, img)
			if err != nil {
				return GetPublicPhotobookOutput{}, fmt.Errorf("build variants: %w", err)
			}
			if set == nil {
				// display / thumbnail variant 不足（不整合）。Viewer は壊さず除外。
				continue
			}
			photoCaption := captionPtr(photo.Caption())
			publicPhotos = append(publicPhotos, PublicPhotoView{
				Caption:  photoCaption,
				Variants: *set,
			})
		}
		publicPages = append(publicPages, PublicPageView{
			Caption: captionPtr(page.Caption()),
			Photos:  publicPhotos,
		})
	}

	// 5) cover の variant
	var cover *PublicVariantSet
	if pb.CoverImageID() != nil {
		coverImg, err := imgRepo.FindByID(ctx, *pb.CoverImageID())
		if err != nil && !errors.Is(err, imagerdb.ErrNotFound) {
			return GetPublicPhotobookOutput{}, fmt.Errorf("find cover image: %w", err)
		}
		if err == nil && coverImg.IsAvailable() {
			set, err := u.buildVariantSet(ctx, coverImg)
			if err != nil {
				return GetPublicPhotobookOutput{}, fmt.Errorf("build cover variants: %w", err)
			}
			cover = set
		}
	}

	view := PublicPhotobookView{
		Type:               pb.Type().String(),
		Title:              pb.Title(),
		Description:        pb.Description(),
		Layout:             pb.Layout().String(),
		OpeningStyle:       pb.OpeningStyle().String(),
		CreatorDisplayName: pb.CreatorDisplayName(),
		CreatorXID:         pb.CreatorXID(),
		CoverTitle:         pb.CoverTitle(),
		Cover:              cover,
		PublishedAt:        *pb.PublishedAt(),
		Pages:              publicPages,
	}
	return GetPublicPhotobookOutput{View: view}, nil
}

// assessPublicVisibility は public 経路から見える / 見えないを判定する。
//
//   - status='published' AND hidden=false AND visibility != 'private' → nil
//   - status='published' AND hidden=true → ErrPublicGone
//   - その他 → ErrPublicNotFound（draft / deleted / purged / private を区別しない）
func assessPublicVisibility(pb domain.Photobook) error {
	if !pb.Status().IsPublished() {
		return ErrPublicNotFound
	}
	if pb.Visibility().Equal(visibility.Private()) {
		return ErrPublicNotFound
	}
	if pb.HiddenByOperator() {
		return ErrPublicGone
	}
	return nil
}

// buildVariantSet は available image から display + thumbnail の presigned URL を作る。
//
// 必須 variant が欠けている場合 (nil, nil) を返す。呼び出し側は表示から除外する。
func (u *GetPublicPhotobook) buildVariantSet(
	ctx context.Context,
	img imagedomain.Image,
) (*PublicVariantSet, error) {
	display, ok := findVariantByKindStr(img, "display")
	if !ok {
		return nil, nil
	}
	thumbnail, ok := findVariantByKindStr(img, "thumbnail")
	if !ok {
		return nil, nil
	}
	displayURL, err := u.r2Client.PresignGetObject(ctx, r2.PresignGetInput{
		StorageKey: display.StorageKey().String(),
		ExpiresIn:  PublicViewExpiresIn,
	})
	if err != nil {
		return nil, fmt.Errorf("presign display: %w", err)
	}
	thumbnailURL, err := u.r2Client.PresignGetObject(ctx, r2.PresignGetInput{
		StorageKey: thumbnail.StorageKey().String(),
		ExpiresIn:  PublicViewExpiresIn,
	})
	if err != nil {
		return nil, fmt.Errorf("presign thumbnail: %w", err)
	}
	return &PublicVariantSet{
		Display: PresignedURLView{
			URL:       displayURL.URL,
			Width:     display.Dimensions().Width(),
			Height:    display.Dimensions().Height(),
			ExpiresAt: displayURL.ExpiresAt,
		},
		Thumbnail: PresignedURLView{
			URL:       thumbnailURL.URL,
			Width:     thumbnail.Dimensions().Width(),
			Height:    thumbnail.Dimensions().Height(),
			ExpiresAt: thumbnailURL.ExpiresAt,
		},
	}, nil
}

// findVariantByKindStr は image の variants から kind 名一致の 1 件を返す。
//
// variant_kind VO に直接依存せず文字列で比較するのは、photobook usecase 側の依存方向を
// 抑えるため（image domain の Variants() / VariantByKind() を活用）。
func findVariantByKindStr(img imagedomain.Image, kindName string) (imagedomain.ImageVariant, bool) {
	for _, v := range img.Variants() {
		if v.Kind().String() == kindName {
			return v, true
		}
	}
	return imagedomain.ImageVariant{}, false
}

// captionPtr は *caption.Caption を *string に展開する（nil 安全）。
func captionPtr(c *caption.Caption) *string {
	if c == nil {
		return nil
	}
	s := c.String()
	return &s
}
