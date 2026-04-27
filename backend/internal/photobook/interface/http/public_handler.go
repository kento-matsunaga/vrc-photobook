// public_handler: 公開 Viewer の HTTP handler。
//
// 設計参照:
//   - docs/plan/m2-public-viewer-and-manage-plan.md §3 / §6 / §12
//
// 仕様:
//   - GET /api/public/photobooks/{slug}
//   - 200 / 410 / 404 / 500 を返す
//   - 失敗の理由詳細は body に出さない
//   - Cache-Control: private, no-store（個人作成 photobook、indexing は Frontend 側で制御）
//   - X-Robots-Tag: noindex, nofollow（API も index させない）
//   - storage_key 完全値 / R2 credentials / 未公開 photobook の存在情報 は出さない
package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"vrcpb/backend/internal/photobook/internal/usecase"
)

const (
	bodyNotFound = `{"status":"not_found"}`
	bodyGone     = `{"status":"gone"}`
)

// PublicHandlers は公開 Viewer の HTTP handler 群。
type PublicHandlers struct {
	getPublic *usecase.GetPublicPhotobook
}

// NewPublicHandlers は PublicHandlers を組み立てる。
func NewPublicHandlers(getPublic *usecase.GetPublicPhotobook) *PublicHandlers {
	return &PublicHandlers{getPublic: getPublic}
}

// publicVariantPayload は presigned URL の JSON 出力。
type publicVariantPayload struct {
	URL       string    `json:"url"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	ExpiresAt time.Time `json:"expires_at"`
}

type publicVariantSetPayload struct {
	Display   publicVariantPayload `json:"display"`
	Thumbnail publicVariantPayload `json:"thumbnail"`
}

type publicPhotoPayload struct {
	Caption  *string                 `json:"caption,omitempty"`
	Variants publicVariantSetPayload `json:"variants"`
}

type publicPagePayload struct {
	Caption *string              `json:"caption,omitempty"`
	Photos  []publicPhotoPayload `json:"photos"`
}

type publicPhotobookPayload struct {
	PhotobookID        string                   `json:"photobook_id"`
	Slug               string                   `json:"slug"`
	Type               string                   `json:"type"`
	Title              string                   `json:"title"`
	Description        *string                  `json:"description,omitempty"`
	Layout             string                   `json:"layout"`
	OpeningStyle       string                   `json:"opening_style"`
	CreatorDisplayName string                   `json:"creator_display_name"`
	CreatorXID         *string                  `json:"creator_x_id,omitempty"`
	CoverTitle         *string                  `json:"cover_title,omitempty"`
	Cover              *publicVariantSetPayload `json:"cover,omitempty"`
	PublishedAt        time.Time                `json:"published_at"`
	Pages              []publicPagePayload      `json:"pages"`
}

// GetPublicPhotobook は GET /api/public/photobooks/{slug} ハンドラ。
func (h *PublicHandlers) GetPublicPhotobook(w http.ResponseWriter, r *http.Request) {
	addNoStore(w)
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	rawSlug := chi.URLParam(r, "slug")
	if rawSlug == "" {
		writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		return
	}

	out, err := h.getPublic.Execute(r.Context(), usecase.GetPublicPhotobookInput{
		RawSlug: rawSlug,
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrPublicNotFound):
			writeJSONStatus(w, http.StatusNotFound, bodyNotFound)
		case errors.Is(err, usecase.ErrPublicGone):
			writeJSONStatus(w, http.StatusGone, bodyGone)
		default:
			writeJSONStatus(w, http.StatusInternalServerError, bodyServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, toPublicPayload(out.View))
}

// toPublicPayload は usecase view を JSON payload に変換する。
func toPublicPayload(v usecase.PublicPhotobookView) publicPhotobookPayload {
	pages := make([]publicPagePayload, 0, len(v.Pages))
	for _, p := range v.Pages {
		photos := make([]publicPhotoPayload, 0, len(p.Photos))
		for _, ph := range p.Photos {
			photos = append(photos, publicPhotoPayload{
				Caption:  ph.Caption,
				Variants: toVariantSetPayload(ph.Variants),
			})
		}
		pages = append(pages, publicPagePayload{
			Caption: p.Caption,
			Photos:  photos,
		})
	}
	var cover *publicVariantSetPayload
	if v.Cover != nil {
		c := toVariantSetPayload(*v.Cover)
		cover = &c
	}
	return publicPhotobookPayload{
		PhotobookID:        v.PhotobookID,
		Slug:               v.Slug,
		Type:               v.Type,
		Title:              v.Title,
		Description:        v.Description,
		Layout:             v.Layout,
		OpeningStyle:       v.OpeningStyle,
		CreatorDisplayName: v.CreatorDisplayName,
		CreatorXID:         v.CreatorXID,
		CoverTitle:         v.CoverTitle,
		Cover:              cover,
		PublishedAt:        v.PublishedAt.UTC(),
		Pages:              pages,
	}
}

func toVariantSetPayload(v usecase.PublicVariantSet) publicVariantSetPayload {
	return publicVariantSetPayload{
		Display:   toVariantPayload(v.Display),
		Thumbnail: toVariantPayload(v.Thumbnail),
	}
}

func toVariantPayload(v usecase.PresignedURLView) publicVariantPayload {
	return publicVariantPayload{
		URL:       v.URL,
		Width:     v.Width,
		Height:    v.Height,
		ExpiresAt: v.ExpiresAt.UTC(),
	}
}
