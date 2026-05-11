// public_handler: OGP lookup の HTTP layer（PR33c）。
//
// 設計参照:
//   - docs/plan/m2-ogp-generation-plan.md §5 / §9
//   - docs/design/cross-cutting/ogp-generation.md §7
//
// endpoint: GET /api/public/photobooks/{photobookId}/ogp
//
// レスポンス:
//   - 200: { "status": "...", "version": <n>, "image_url_path": "..." }
//          - status='generated' なら image_url_path = "/ogp/<photobook_id>?v=<n>"
//          - それ以外（pending / failed / fallback / stale / not_public）は
//            image_url_path = "/og/default.png"（Frontend default OGP）
//   - 404: photobook_ogp_images row が無い（status_columns_consistency_check の整合上
//          row は publish 時に作られる想定だが、PR33b 段階では手動 CLI 実行のみのため
//          row が無い photobook も多い。Workers proxy はこのとき default に redirect）
//   - 500: DB エラー等
//
// セキュリティ:
//   - 管理 URL / token / hash / Cookie / R2 credentials を返さない
//   - storage_key 完全値も返さない（image_url_path のみ）
//   - draft / private / hidden / deleted の photobook は status='not_public' を返し
//     image_url_path はデフォルト OGP に倒す（情報漏洩抑止）
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"vrcpb/backend/internal/ogp/internal/usecase"
	photobookid "vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// 既定 OGP の Frontend 静的配信パス。Workers + middleware 経由で取得される。
const defaultOgpImagePath = "/og/default.png"

// GetPublicOgpExecutor は GetOgp handler が依存する最小 interface。
//
// production では `*usecase.GetPublicOgp` を直接渡す。test では fake 実装で
// visibility 別の Execute outcome（unlisted → generated, private → not_public 等）
// を制御し、handler のレスポンス整形を独立検証する。
type GetPublicOgpExecutor interface {
	Execute(ctx context.Context, pid photobookid.PhotobookID) (usecase.PublicOgpView, error)
}

// PublicHandlers は OGP lookup HTTP handler。
type PublicHandlers struct {
	getPublic GetPublicOgpExecutor
}

// NewPublicHandlers は組み立て関数。`*usecase.GetPublicOgp` が
// `GetPublicOgpExecutor` を満たすため production 呼び出し側の変更不要。
func NewPublicHandlers(getPublic GetPublicOgpExecutor) *PublicHandlers {
	return &PublicHandlers{getPublic: getPublic}
}

type ogpResponse struct {
	Status       string `json:"status"`
	Version      int    `json:"version"`
	ImageURLPath string `json:"image_url_path"`
}

// GetOgp は GET /api/public/photobooks/{photobookId}/ogp ハンドラ。
func (h *PublicHandlers) GetOgp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	rawID := chi.URLParam(r, "photobookId")
	u, err := uuid.Parse(rawID)
	if err != nil {
		writeOgpStatus(w, http.StatusNotFound, ogpResponse{
			Status:       "not_found",
			ImageURLPath: defaultOgpImagePath,
		})
		return
	}
	pid, err := photobookid.FromUUID(u)
	if err != nil {
		writeOgpStatus(w, http.StatusNotFound, ogpResponse{
			Status:       "not_found",
			ImageURLPath: defaultOgpImagePath,
		})
		return
	}

	out, err := h.getPublic.Execute(r.Context(), pid)
	if err != nil {
		if errors.Is(err, usecase.ErrOgpNotFound) {
			writeOgpStatus(w, http.StatusOK, ogpResponse{
				Status:       "not_found",
				ImageURLPath: defaultOgpImagePath,
			})
			return
		}
		writeOgpStatus(w, http.StatusInternalServerError, ogpResponse{
			Status:       "error",
			ImageURLPath: defaultOgpImagePath,
		})
		return
	}

	if out.OgpImageStatus == "generated" && out.StorageKey != "" {
		// /ogp/<photobook_id>?v=<n> を Workers proxy が解決する。
		writeOgpStatus(w, http.StatusOK, ogpResponse{
			Status:       "generated",
			Version:      out.OgpVersion,
			ImageURLPath: ogpImagePath(rawID, out.OgpVersion),
		})
		return
	}

	writeOgpStatus(w, http.StatusOK, ogpResponse{
		Status:       out.OgpImageStatus,
		Version:      out.OgpVersion,
		ImageURLPath: defaultOgpImagePath,
	})
}

func writeOgpStatus(w http.ResponseWriter, code int, body ogpResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func ogpImagePath(rawID string, version int) string {
	if version <= 0 {
		version = 1
	}
	return "/ogp/" + rawID + "?v=" + itoa(version)
}

// itoa は外部依存なしの int → 10 進文字列。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 12)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
