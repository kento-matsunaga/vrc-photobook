// Package cookie は Session 認可機構の Cookie policy を提供する。
//
// Cookie 属性の確定事項（docs/adr/0003-frontend-token-session-flow.md / docs/plan/m2-session-auth-implementation-plan.md §6）:
//   - HttpOnly = true
//   - Secure   = true（localhost も Secure context として扱われるため、ローカルでも true のまま）
//   - SameSite = Strict
//   - Path     = /
//   - Domain   = COOKIE_DOMAIN が空なら未設定、値があればその値（独自ドメイン取得後 .<domain>）
//   - Max-Age  = expires_at - now（int 秒）
//
// 名前:
//   - draft session: vrcpb_draft_{photobook_id}
//   - manage session: vrcpb_manage_{photobook_id}
//
// セキュリティ:
//   - Set-Cookie ヘッダ全体 / Cookie 値はログに出さない（shared/logging.go 禁止リスト）
//   - 本パッケージは「ヘッダ文字列を組み立てるだけ」。読み取りは middleware 側の責務
package cookie

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
)

// 名前 prefix。session_type ごとに使い分ける。
const (
	prefixDraft  = "vrcpb_draft_"
	prefixManage = "vrcpb_manage_"
)

// ErrInvalidExpiry は expiresAt が now 以下のときのエラー。
var ErrInvalidExpiry = errors.New("expiresAt must be after now")

// Policy は Cookie 発行ポリシーの設定。環境変数等から組み立てる。
type Policy struct {
	// Domain は COOKIE_DOMAIN 環境変数の値。空文字なら Domain 属性を出さない（host-only Cookie）。
	Domain string
}

// Name は session_type と photobook_id から Cookie 名を組み立てる。
//
// Cookie 名は仕様上「token (RFC 7230) の文字集合」にする必要があるが、
// session_type は draft / manage の固定値、photobook_id は UUID（hex + ハイフン）なので
// 制約に収まる。バリデーションは VO 側で済んでいる前提。
func Name(t session_type.SessionType, pid photobook_id.PhotobookID) string {
	prefix := prefixDraft
	if t.IsManage() {
		prefix = prefixManage
	}
	return prefix + pid.String()
}

// BuildIssue は Set-Cookie 発行用の http.Cookie を組み立てる。
//
// 呼び出し側は http.SetCookie(w, cookie) で書き込む。
// raw token は cookie.Value に乗る — **ログに出さない**。
func (p Policy) BuildIssue(
	t session_type.SessionType,
	pid photobook_id.PhotobookID,
	token session_token.SessionToken,
	now time.Time,
	expiresAt time.Time,
) (*http.Cookie, error) {
	if !expiresAt.After(now) {
		return nil, fmt.Errorf("%w: now=%s expires=%s", ErrInvalidExpiry, now, expiresAt)
	}
	maxAge := int(expiresAt.Sub(now).Seconds())
	if maxAge <= 0 {
		return nil, ErrInvalidExpiry
	}
	c := &http.Cookie{
		Name:     Name(t, pid),
		Value:    token.Encode(),
		Path:     "/",
		Domain:   p.Domain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	return c, nil
}

// BuildClear は明示破棄用 Cookie を組み立てる（Max-Age=0、空 Value）。
//
// 元の draft_edit_token / manage_url_token は失効させない（別端末からの再入場を妨げない、
// 設計書 §3.3）。
func (p Policy) BuildClear(
	t session_type.SessionType,
	pid photobook_id.PhotobookID,
) *http.Cookie {
	return &http.Cookie{
		Name:     Name(t, pid),
		Value:    "",
		Path:     "/",
		Domain:   p.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}

// AssertSecureAttributes は Cookie に必須属性が揃っているかを開発時 / テストで確認するためのヘルパ。
//
// 本番ハンドラから呼ぶことは想定しない（テストとレビューでの目視確認の補助）。
func AssertSecureAttributes(c *http.Cookie) error {
	if c == nil {
		return errors.New("cookie is nil")
	}
	if !c.HttpOnly {
		return errors.New("HttpOnly must be true")
	}
	if !c.Secure {
		return errors.New("Secure must be true")
	}
	if c.SameSite != http.SameSiteStrictMode {
		return errors.New("SameSite must be Strict")
	}
	if c.Path != "/" {
		return errors.New(`Path must be "/"`)
	}
	if !strings.HasPrefix(c.Name, prefixDraft) && !strings.HasPrefix(c.Name, prefixManage) {
		return fmt.Errorf("name must start with %q or %q", prefixDraft, prefixManage)
	}
	return nil
}
