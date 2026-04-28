// Package contract は outbox handler が要求する依存抽象を**外部 package から見える形**で
// 公開する場所。`internal/outbox/internal/usecase/handlers` は internal package のため
// 別ツリー（ogp/wireup 等）から直接 import できないので、interface / sentinel を本
// package に切り出す。
//
// 依存方向:
//   - handlers（同 outbox 配下） → contract
//   - ogp/wireup（別ツリー） → contract（internal は backend/internal 直下なので見える）
//   - cmd → contract / handlers / ogp/wireup
package contract

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// OgpGenerator は outbox の photobook.published handler が呼び出す OGP 生成抽象。
//
// 実装は ogp/wireup の adapter（OGP UseCase をラップ）。
type OgpGenerator interface {
	GenerateForPhotobook(ctx context.Context, photobookID uuid.UUID, now time.Time) (OgpGenerateResult, error)
}

// OgpGenerateResult は handler が log するための最小 view。
type OgpGenerateResult struct {
	OgpImageID uuid.UUID
	Generated  bool
}

// ErrNotPublishedSkippable は対象 photobook が公開状態（published + visibility=public
// + hidden=false）で **無い**ため OGP 生成を skip したことを示す。
//
// outbox worker はこの error を **processed** 扱いにする（handler が nil 返却で
// processed に倒す方針、handler 内で errors.Is で判定）。public に切り替わって OGP
// 再生成が必要な経路は、cover 変更や ReissueManageUrl 等の Photobook 更新時に
// `MarkStale` で別 event を起こす設計（cross-cutting/ogp-generation.md §5.2）。
var ErrNotPublishedSkippable = errors.New("ogp generation skipped: photobook not public")
