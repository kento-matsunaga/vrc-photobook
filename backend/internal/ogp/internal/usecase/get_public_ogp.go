// get_public_ogp: Workers proxy / Frontend metadata からの OGP lookup UseCase。
//
// 設計参照:
//   - docs/plan/m2-ogp-generation-plan.md §5（配信経路）/ §11（Security）
//   - docs/design/cross-cutting/ogp-generation.md §7
//
// 入力: photobook_id（公開識別子）
// 出力: status / version / storage_key（generated 時のみ）
//
// public 経路から呼ばれるため:
//   - Cookie / token / 管理 URL は受け取らない
//   - photobook が published / visibility='public' / hidden=false で **無い**場合は
//     status='not_public' を返し（呼び出し側 = Workers proxy が default OGP に redirect）
//   - photobook そのものが無ければ ErrNotFound（呼び出し側 = HTTP layer が 404）
//   - storage_key は internal にだけ返し、HTTP response body には含めない（PR33c
//     の HTTP handler で image_url_path に変換して返す）
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	ogprdb "vrcpb/backend/internal/ogp/infrastructure/repository/rdb"
	photobookid "vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// ErrOgpNotFound は photobook_ogp_images row が存在しないとき。
//
// 呼び出し側（HTTP / Workers proxy）は default OGP fallback に切り替える。
var ErrOgpNotFound = errors.New("ogp not found")

// PublicOgpView は HTTP layer に返す view。
type PublicOgpView struct {
	OgpImageStatus string // 'pending' / 'generated' / 'failed' / 'fallback' / 'stale' / 'not_public'
	OgpVersion     int    // generated 時のみ意味あり、それ以外も値は返す
	// StorageKey は **internal 用**。Workers proxy が R2 binding 経由で GET する場合に
	// 必要だが、HTTP body に含める場合は呼び出し側が image_url_path に変換すること。
	StorageKey string // generated 時のみ非空
}

// GetPublicOgp は public OGP lookup UseCase。
type GetPublicOgp struct {
	pool *pgxpool.Pool
}

// NewGetPublicOgp は UseCase を組み立てる。
func NewGetPublicOgp(pool *pgxpool.Pool) *GetPublicOgp {
	return &GetPublicOgp{pool: pool}
}

// Execute は photobook_id から OGP delivery 情報を返す。
func (u *GetPublicOgp) Execute(ctx context.Context, pid photobookid.PhotobookID) (PublicOgpView, error) {
	repo := ogprdb.NewOgpRepository(u.pool)
	d, err := repo.GetDeliveryByPhotobookID(ctx, pid.UUID())
	if err != nil {
		if errors.Is(err, ogprdb.ErrNotFound) {
			return PublicOgpView{}, ErrOgpNotFound
		}
		return PublicOgpView{}, fmt.Errorf("get delivery: %w", err)
	}

	// public 配信可否（v4 §3.2 / cross-cutting/ogp-generation.md §11）。
	publicAllowed := d.PhotobookStatus == "published" &&
		d.PhotobookVisibility == "public" &&
		!d.HiddenByOperator
	if !publicAllowed {
		return PublicOgpView{
			OgpImageStatus: "not_public",
			OgpVersion:     d.OgpVersion,
		}, nil
	}

	// generated 以外は storage_key 返さない（fallback）
	if d.OgpStatus != "generated" || d.StorageKey == "" {
		return PublicOgpView{
			OgpImageStatus: d.OgpStatus,
			OgpVersion:     d.OgpVersion,
		}, nil
	}

	return PublicOgpView{
		OgpImageStatus: "generated",
		OgpVersion:     d.OgpVersion,
		StorageKey:     d.StorageKey,
	}, nil
}
