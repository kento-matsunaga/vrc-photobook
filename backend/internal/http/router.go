// Package http は HTTP ルーターを組み立てる。
//
// PR2: `/health` + `/readyz`（DB 未実装時 503 固定）。
// PR3: `/readyz` を pool 状態に応じた分岐に置き換え。
// PR8: Session 認可 middleware と UseCase 枠を用意（**未接続のまま**）。
// PR9c: Photobook の token 交換 endpoint 2 本（draft / manage）を追加。
//       本物の token 検証経由のみ。dummy token / 認証バイパスは作らない。
// PR10 以降で Frontend Route Handler 接続 + Cookie 化 + Safari 確認。
//
// 注意:
//   - chi の middleware（CORS / RequestID / Recoverer / Timeout）は本 PR では入れない
//   - protected route（/api/photobooks/{id}）は PR10 以降で session middleware と一緒に接続
package http

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/health"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
)

// NewRouter は API サーバの chi ルーターを返す。
//
// pool は nil でも可（その場合 /readyz は 503 db_not_configured）。
// photobookHandlers が nil の場合は token 交換 endpoint を **登録しない**（DB 未設定時）。
func NewRouter(pool *pgxpool.Pool, photobookHandlers *photobookhttp.Handlers) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/health", health.Health)
	r.Get("/readyz", health.Ready(pool))

	// PR9c: token 交換 endpoint。Photobook UseCase 経由で本物の token を検証する。
	// DB 未設定時（pool nil）は handlers も nil 渡しになるため endpoint を生やさない。
	if photobookHandlers != nil {
		r.Post("/api/auth/draft-session-exchange", photobookHandlers.DraftSessionExchange)
		r.Post("/api/auth/manage-session-exchange", photobookHandlers.ManageSessionExchange)
	}
	return r
}
