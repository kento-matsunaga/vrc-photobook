// public_handler の HTTP integration unit test。
//
// 検証範囲（A 案 2026-05-11、unlisted も OGP 配信許可）:
//   - visibility=public + status=generated → 200 / status="generated" / image_url_path=/ogp/<id>?v=<n>
//   - visibility=unlisted + status=generated → 200 / status="generated" / image_url_path=/ogp/<id>?v=<n>
//   - visibility=private + status=generated → 200 / status="not_public" / image_url_path=/og/default.png
//   - hidden_by_operator=true → 200 / status="not_public" / image_url_path=/og/default.png
//   - ErrOgpNotFound → 200 / status="not_found" / image_url_path=/og/default.png
//   - 不正 UUID path → 404 / status="not_found" / image_url_path=/og/default.png
//   - generic error → 500 / status="error"
package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	ogphttp "vrcpb/backend/internal/ogp/interface/http"
	"vrcpb/backend/internal/ogp/internal/usecase"
	photobookid "vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// fakeGetPublicOgp は handler test 用 fake。
type fakeGetPublicOgp struct {
	out usecase.PublicOgpView
	err error
}

func (f *fakeGetPublicOgp) Execute(_ context.Context, _ photobookid.PhotobookID) (usecase.PublicOgpView, error) {
	return f.out, f.err
}

func newRouter(t *testing.T, exec ogphttp.GetPublicOgpExecutor) http.Handler {
	t.Helper()
	h := ogphttp.NewPublicHandlers(exec)
	r := chi.NewRouter()
	r.Get("/api/public/photobooks/{photobookId}/ogp", h.GetOgp)
	return r
}

type ogpResponseBody struct {
	Status       string `json:"status"`
	Version      int    `json:"version"`
	ImageURLPath string `json:"image_url_path"`
}

func TestGetOgp(t *testing.T) {
	validUUID := uuid.New().String()

	tests := []struct {
		name        string
		description string
		pathID      string
		fake        *fakeGetPublicOgp
		wantStatus  int
		wantBody    ogpResponseBody
	}{
		{
			name:        "正常_public_generated_は_generated_と_imageUrlPath_を返す",
			description: "Given: UC が OgpImageStatus=generated + StorageKey 設定済を返す, Then: 200 / status=generated / image_url_path=/ogp/<id>?v=<n>",
			pathID:      validUUID,
			fake: &fakeGetPublicOgp{
				out: usecase.PublicOgpView{
					OgpImageStatus: "generated",
					OgpVersion:     3,
					StorageKey:     "photobooks/x/ogp/y/z.png",
				},
			},
			wantStatus: http.StatusOK,
			wantBody: ogpResponseBody{
				Status:       "generated",
				Version:      3,
				ImageURLPath: "/ogp/" + validUUID + "?v=3",
			},
		},
		{
			name:        "正常_unlisted_generated_も_generated_を返す_A案",
			description: "Given: visibility=unlisted で UseCase が同等の generated を返す, Then: handler 上 public と区別せず generated と image_url_path を返す（A 案 2026-05-11）",
			pathID:      validUUID,
			fake: &fakeGetPublicOgp{
				out: usecase.PublicOgpView{
					OgpImageStatus: "generated",
					OgpVersion:     1,
					StorageKey:     "photobooks/x/ogp/y/z.png",
				},
			},
			wantStatus: http.StatusOK,
			wantBody: ogpResponseBody{
				Status:       "generated",
				Version:      1,
				ImageURLPath: "/ogp/" + validUUID + "?v=1",
			},
		},
		{
			name:        "異常_private_は_not_public_と_default_path_を返す",
			description: "Given: visibility=private で UseCase が OgpImageStatus=not_public を返す, Then: 200 / status=not_public / image_url_path=/og/default.png",
			pathID:      validUUID,
			fake: &fakeGetPublicOgp{
				out: usecase.PublicOgpView{
					OgpImageStatus: "not_public",
					OgpVersion:     2,
				},
			},
			wantStatus: http.StatusOK,
			wantBody: ogpResponseBody{
				Status:       "not_public",
				Version:      2,
				ImageURLPath: "/og/default.png",
			},
		},
		{
			name:        "異常_hidden_by_operator_は_not_public_扱い",
			description: "Given: hidden_by_operator=true で UseCase が not_public を返す, Then: status=not_public / default path",
			pathID:      validUUID,
			fake: &fakeGetPublicOgp{
				out: usecase.PublicOgpView{
					OgpImageStatus: "not_public",
					OgpVersion:     5,
				},
			},
			wantStatus: http.StatusOK,
			wantBody: ogpResponseBody{
				Status:       "not_public",
				Version:      5,
				ImageURLPath: "/og/default.png",
			},
		},
		{
			name:        "異常_pending_は_pending_status_だが_default_path",
			description: "Given: OgpImageStatus=pending（worker 待ち）, Then: status=pending / image_url_path=/og/default.png（crawler は default に倒れる）",
			pathID:      validUUID,
			fake: &fakeGetPublicOgp{
				out: usecase.PublicOgpView{
					OgpImageStatus: "pending",
					OgpVersion:     1,
				},
			},
			wantStatus: http.StatusOK,
			wantBody: ogpResponseBody{
				Status:       "pending",
				Version:      1,
				ImageURLPath: "/og/default.png",
			},
		},
		{
			name:        "異常_UseCase_ErrOgpNotFound_は_not_found_200",
			description: "Given: UseCase が ErrOgpNotFound, Then: 200 / status=not_found / default path（Workers proxy が default に redirect する想定の応答）",
			pathID:      validUUID,
			fake: &fakeGetPublicOgp{
				err: usecase.ErrOgpNotFound,
			},
			wantStatus: http.StatusOK,
			wantBody: ogpResponseBody{
				Status:       "not_found",
				ImageURLPath: "/og/default.png",
			},
		},
		{
			name:        "異常_不正_UUID_path_は_404",
			description: "Given: UUID 形式でない path, Then: 404 / status=not_found / default path",
			pathID:      "not-a-uuid",
			fake:        &fakeGetPublicOgp{},
			wantStatus:  http.StatusNotFound,
			wantBody: ogpResponseBody{
				Status:       "not_found",
				ImageURLPath: "/og/default.png",
			},
		},
		{
			name:        "異常_UseCase_generic_error_は_500",
			description: "Given: UseCase が想定外 error, Then: 500 / status=error / default path",
			pathID:      validUUID,
			fake: &fakeGetPublicOgp{
				err: errors.New("simulated db failure"),
			},
			wantStatus: http.StatusInternalServerError,
			wantBody: ogpResponseBody{
				Status:       "error",
				ImageURLPath: "/og/default.png",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			router := newRouter(t, tt.fake)
			req := httptest.NewRequest(http.MethodGet, "/api/public/photobooks/"+tt.pathID+"/ogp", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("%s\nHTTP status = %d, want %d", tt.description, rec.Code, tt.wantStatus)
			}
			var got ogpResponseBody
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("%s\nresponse body decode: %v\nbody: %s", tt.description, err, rec.Body.String())
			}
			if got != tt.wantBody {
				t.Fatalf("%s\nbody = %+v\nwant   %+v", tt.description, got, tt.wantBody)
			}
			// 共通 header 確認
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=60" {
				t.Errorf("Cache-Control = %q, want \"public, max-age=60\"", cc)
			}
			if xr := rec.Header().Get("X-Robots-Tag"); xr != "noindex, nofollow" {
				t.Errorf("X-Robots-Tag = %q, want \"noindex, nofollow\"", xr)
			}
		})
	}
}
