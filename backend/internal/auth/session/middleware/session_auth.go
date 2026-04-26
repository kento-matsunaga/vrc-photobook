// Package middleware は Session 認可機構の HTTP middleware を提供する。
//
// 配置の理由（Go internal ルール）:
//   - usecase パッケージは internal/auth/session/internal/usecase 配下にあり、
//     internal/auth/session のサブツリーからしか import できない
//   - 本 middleware は usecase に依存するため、同じサブツリー内（auth/session/middleware）に置く
//   - 外部（chi router 等）からは「session 機構が公開する HTTP middleware」として import する
//
// PR8: Session 認可の枠（RequireDraftSession / RequireManageSession / SessionFromContext）。
//
// 設計参照:
//   - docs/adr/0003-frontend-token-session-flow.md
//   - docs/plan/m2-session-auth-implementation-plan.md §7 / §11 / §12
//
// セキュリティ:
//   - Cookie 値・raw token はログに出さない（slog の任意フィールド出力禁止）
//   - 失敗時は 401 unauthorized を返し、原因詳細は body に出さない
//   - 認可成功時のみ context に Session を格納し、handler は SessionFromContext で取得する
//
// 注意:
//   - PR8 では本番 router からは未接続。HTTP endpoint の本接続は PR9 (Photobook aggregate) と
//     PR10 (Frontend route) の段階で行う。
//   - photobook_id の取得関数は呼び出し元から注入する（URL param / path / body の差を吸収するため）。
package middleware

import (
	"context"
	"errors"
	"net/http"

	"vrcpb/backend/internal/auth/session/cookie"
	"vrcpb/backend/internal/auth/session/domain"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/internal/usecase"
)

// sessionContextKey は context.Value のキー。外部からは不可視にして衝突を防ぐ。
type sessionContextKey struct{}

// SessionFromContext は middleware で確認済みの Session を context から取り出す。
// 呼び出し元 handler は middleware を通った後でのみ呼ぶ前提（未認証の経路では nil/false）。
func SessionFromContext(ctx context.Context) (domain.Session, bool) {
	v, ok := ctx.Value(sessionContextKey{}).(domain.Session)
	if !ok {
		return domain.Session{}, false
	}
	return v, true
}

// PhotobookIDExtractor はリクエストから photobook_id を取り出す関数型。
//
// chi router の URL param / リクエスト body / クエリ等、呼び出し元の都合で
// 抽出方法が変わるため、注入する。失敗時はエラーを返す（middleware は 401 にする）。
type PhotobookIDExtractor func(r *http.Request) (photobook_id.PhotobookID, error)

// Validator は ValidateSession UseCase の依存抽象（テスト用に interface 化）。
type Validator interface {
	Execute(ctx context.Context, in usecase.ValidateSessionInput) (usecase.ValidateSessionOutput, error)
}

// RequireDraftSession は draft session を必須とする middleware。
//
// 流れ:
//  1. extractor で photobook_id を取得
//  2. cookie.Name(draft, pid) で Cookie 名を組み立て、Request の Cookie から raw token を取り出す
//  3. session_token.Parse で 43 文字 base64url を検証
//  4. validator.Execute で hash → DB 照合（revoked_at IS NULL AND expires_at > now() を repository が担保）
//  5. 成功時のみ context に Session を入れて next を呼ぶ
//
// いずれかのステップで失敗した場合は 401 を返し、原因は body に出さない。
func RequireDraftSession(validator Validator, extractor PhotobookIDExtractor) func(http.Handler) http.Handler {
	return requireSession(validator, extractor, session_type.Draft())
}

// RequireManageSession は manage session を必須とする middleware。
func RequireManageSession(validator Validator, extractor PhotobookIDExtractor) func(http.Handler) http.Handler {
	return requireSession(validator, extractor, session_type.Manage())
}

// requireSession は draft / manage 共通の middleware 実装。
func requireSession(
	validator Validator,
	extractor PhotobookIDExtractor,
	st session_type.SessionType,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pid, err := extractor(r)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			name := cookie.Name(st, pid)
			c, err := r.Cookie(name)
			if err != nil {
				// http.ErrNoCookie / その他いずれも 401（情報漏洩抑止）
				writeUnauthorized(w)
				return
			}
			tok, err := session_token.Parse(c.Value)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			out, err := validator.Execute(r.Context(), usecase.ValidateSessionInput{
				RawToken:    tok,
				PhotobookID: pid,
				SessionType: st,
			})
			if err != nil {
				if errors.Is(err, usecase.ErrSessionInvalid) {
					writeUnauthorized(w)
					return
				}
				// 想定外エラー（DB 障害等）も 401 で返し、上位観測点でログ出力する。
				// 本 middleware ではログ出力しない（slog 中央 handler の責務）。
				writeUnauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), sessionContextKey{}, out.Session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeUnauthorized は 401 を返す。body は固定文言で、原因詳細を含めない。
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	// Cache 抑止（auth 失敗を CDN にキャッシュさせない）
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"status":"unauthorized"}`))
}
