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
//   - 配信可否は CanDeliverPublicOgp(...) で判定する。published かつ
//     hidden_by_operator=false かつ visibility ∈ {public, unlisted} のみ generated
//     OGP を返す。private は not_public で fallback、hidden / deleted も同様。
//   - photobook そのものが無ければ ErrNotFound（呼び出し側 = HTTP layer が 404）
//   - storage_key は internal にだけ返し、HTTP response body には含めない（PR33c
//     の HTTP handler で image_url_path に変換して返す）
//
// 2026-05-11 unlisted も OGP 配信許可（A 案）:
//   業務知識 v4 §3.2: unlisted は「URL を知っている人のみが閲覧可能」公開範囲。SNS /
//   Discord / Slack / LINE に URL を貼った時に OGP が出る方が自然 UX。検索拒否
//   （`noindex` 全ページ付与）・一覧非表示は HTML 側で維持しており、OGP 配信許可で
//   検索 index には影響しない。private / hidden_by_operator は OGP も不可のまま。
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

// CanDeliverPublicOgp は OGP を public 経路で配信して良いかを判定する pure 関数。
//
// 仕様（業務知識 v4 §3.2、A 案 2026-05-11）:
//   - status: 'published' のみ true
//   - hidden_by_operator: true なら不可
//   - visibility: 'public' または 'unlisted' なら可、'private' は不可
//
// unlisted も配信許可とする理由は、unlisted の業務定義「URL を知っている人のみ
// 閲覧可能」と矛盾なく SNS 共有時の OGP 表示 UX を成立させるため。検索拒否は HTML
// `noindex` で別途維持しており本判定とは独立。
//
// 純粋関数なので unit test で全 case を網羅できる（DB 不要）。
func CanDeliverPublicOgp(d ogprdb.OgpDelivery) bool {
	if d.PhotobookStatus != "published" {
		return false
	}
	if d.HiddenByOperator {
		return false
	}
	return d.PhotobookVisibility == "public" || d.PhotobookVisibility == "unlisted"
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

	// 配信可否（v4 §3.2 / §6.17 / cross-cutting/ogp-generation.md §7.2.1）。
	if !CanDeliverPublicOgp(d) {
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
