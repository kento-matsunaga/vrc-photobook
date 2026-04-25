// Frontend / Backend 結合検証用 sandbox エンドポイント。
//
// セキュリティ方針:
//   - Cookie 値そのものをレスポンスに返さない（存在/不存在のみ）
//   - Cookie 値・raw token をログに出さない
package sandbox

import (
	"net/http"
	"strings"
)

// SessionCheck は draft / manage Cookie の存在のみを返す。
// Cookie 値そのものは返さない。
//
// Frontend からの credentials: "include" fetch で Cookie が引き渡せるかの確認用。
func SessionCheck(w http.ResponseWriter, r *http.Request) {
	draftPresent := false
	managePresent := false

	for _, c := range r.Cookies() {
		if strings.HasPrefix(c.Name, "vrcpb_draft_") {
			draftPresent = true
		}
		if strings.HasPrefix(c.Name, "vrcpb_manage_") {
			managePresent = true
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{
		"draft_cookie_present":  draftPresent,
		"manage_cookie_present": managePresent,
	})
}

// OriginCheck は Origin ヘッダが許可リストに含まれるかを判定する。
//
// 状態変更 API の最小 CSRF 対策として「自オリジンからのみ受理」を実現する基盤。
//
// 仕様:
//   - Origin ヘッダ空 → 403 (origin_required)
//     ※ 同一オリジンからの fetch では Origin が省略されることもあるが、
//       ブラウザは状態変更系（POST/PUT/PATCH/DELETE）では Origin を付ける。
//       本 PoC では空も拒否する厳しめ設定。
//   - Origin がホワイトリスト外 → 403 (origin_not_allowed)
//   - 一致 → 200 {"origin_allowed": true}
func OriginCheck(allowedOrigins []string) http.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o != "" {
			allowed[o] = struct{}{}
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			writeError(w, http.StatusForbidden, "origin_required")
			return
		}
		if _, ok := allowed[origin]; !ok {
			writeError(w, http.StatusForbidden, "origin_not_allowed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"origin_allowed": true})
	}
}
