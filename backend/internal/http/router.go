// Package http は HTTP ルーターを組み立てる。
//
// PR2: `/health` + `/readyz`（DB 未実装時 503 固定）。
// PR3: `/readyz` を pool 状態に応じた分岐に置き換え。
// PR8: Session 認可 middleware（internal/auth/session/middleware）と UseCase は
//      用意したが、**本 router からは未接続**。dummy token 経由で動く公開エンドポイント
//      を作らないため、認証 endpoint の本接続は PR9（Photobook aggregate と一緒に
//      本物の draft_edit_token / manage_url_token 検証を組む）まで待つ。
// PR9 以降で CORS / RequestID / Recoverer / Timeout / token 交換 endpoint /
// 各集約ルートを追加する。
package http

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/health"
)

// NewRouter は API サーバの chi ルーターを返す。
// pool は nil でも可（その場合 /readyz は 503 db_not_configured）。
func NewRouter(pool *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/health", health.Health)
	r.Get("/readyz", health.Ready(pool))
	return r
}
