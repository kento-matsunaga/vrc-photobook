// Package http は imageupload の HTTP handler を提供する。
//
// 設計参照:
//   - docs/plan/m2-r2-presigned-url-plan.md §9
//   - docs/adr/0005-image-upload-flow.md
//
// 公開 endpoint:
//   - POST /api/photobooks/{id}/images/upload-intent
//   - POST /api/photobooks/{id}/images/{imageId}/complete
//
// セキュリティ:
//   - presigned URL / R2 credentials / raw token / Cookie はログに出さない
//   - 失敗時の詳細は body に出さない（情報漏洩抑止）
//   - draft session middleware で認可された context 前提（pid を URL と再確認）
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	authmiddleware "vrcpb/backend/internal/auth/session/middleware"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/imageupload/internal/usecase"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// 失敗時 body（固定文言、情報漏洩抑止）。
const (
	bodyBadRequest       = `{"status":"bad_request"}`
	bodyUnauthorized     = `{"status":"unauthorized"}`
	bodyUploadVerifFailed = `{"status":"upload_verification_failed"}`
	bodyValidationFailed = `{"status":"upload_validation_failed"}`
	bodyServerError      = `{"status":"internal_error"}`
)

// Clock は時刻取得を抽象化（テスト用に固定可能）。
type Clock interface{ Now() time.Time }

// SystemClock は time.Now を返す。
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// Handlers は imageupload の HTTP handler 群。
type Handlers struct {
	issue    *usecase.IssueUploadIntent
	complete *usecase.CompleteUpload
	clock    Clock
}

// NewHandlers は Handlers を組み立てる。
func NewHandlers(
	issue *usecase.IssueUploadIntent,
	complete *usecase.CompleteUpload,
	clock Clock,
) *Handlers {
	if clock == nil {
		clock = SystemClock{}
	}
	return &Handlers{issue: issue, complete: complete, clock: clock}
}

// === upload-intent ===

type uploadIntentRequest struct {
	ContentType      string `json:"content_type"`
	DeclaredByteSize int64  `json:"declared_byte_size"`
	SourceFormat     string `json:"source_format"`
}

type uploadIntentResponse struct {
	ImageID         string            `json:"image_id"`
	UploadURL       string            `json:"upload_url"`
	RequiredHeaders map[string]string `json:"required_headers"`
	StorageKey      string            `json:"storage_key"`
	ExpiresAt       string            `json:"expires_at"`
}

// UploadIntent は POST /api/photobooks/{id}/images/upload-intent。
//
// 認可:
//   - draft session Cookie（middleware で context に Session を入れる前提）
//   - Authorization: Bearer <upload_verification_token> 必須
func (h *Handlers) UploadIntent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	pid, ok := pidFromURL(r)
	if !ok {
		writeFixed(w, http.StatusBadRequest, bodyBadRequest)
		return
	}

	// draft session middleware が context に置いた Session を再確認
	sess, ok := authmiddleware.SessionFromContext(r.Context())
	if !ok {
		writeFixed(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}
	if sess.PhotobookID().UUID() != pid.UUID() {
		writeFixed(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}

	rawToken := bearerToken(r)
	if rawToken == "" {
		writeFixed(w, http.StatusUnauthorized, bodyUploadVerifFailed)
		return
	}

	var req uploadIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFixed(w, http.StatusBadRequest, bodyBadRequest)
		return
	}

	out, err := h.issue.Execute(r.Context(), usecase.IssueUploadIntentInput{
		PhotobookID:             pid,
		UploadVerificationToken: rawToken,
		ContentType:             req.ContentType,
		DeclaredByteSize:        req.DeclaredByteSize,
		SourceFormat:            req.SourceFormat,
		Now:                     h.clock.Now(),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrUploadVerificationFailed):
			writeFixed(w, http.StatusForbidden, bodyUploadVerifFailed)
		case errors.Is(err, usecase.ErrInvalidUploadParameters):
			writeFixed(w, http.StatusBadRequest, bodyBadRequest)
		default:
			writeFixed(w, http.StatusInternalServerError, bodyServerError)
		}
		return
	}
	resp := uploadIntentResponse{
		ImageID:         out.ImageID.String(),
		UploadURL:       out.UploadURL,
		RequiredHeaders: out.RequiredHeaders,
		StorageKey:      out.StorageKey.String(),
		ExpiresAt:       out.ExpiresAt.UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// === complete ===

type completeRequest struct {
	StorageKey string `json:"storage_key"`
}

type completeResponse struct {
	ImageID string `json:"image_id"`
	Status  string `json:"status"`
}

// Complete は POST /api/photobooks/{id}/images/{imageId}/complete。
//
// 認可:
//   - draft session Cookie 必須
func (h *Handlers) Complete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	pid, ok := pidFromURL(r)
	if !ok {
		writeFixed(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	imgID, ok := imageIDFromURL(r)
	if !ok {
		writeFixed(w, http.StatusBadRequest, bodyBadRequest)
		return
	}

	sess, ok := authmiddleware.SessionFromContext(r.Context())
	if !ok {
		writeFixed(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}
	if sess.PhotobookID().UUID() != pid.UUID() {
		writeFixed(w, http.StatusUnauthorized, bodyUnauthorized)
		return
	}

	var req completeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFixed(w, http.StatusBadRequest, bodyBadRequest)
		return
	}

	out, err := h.complete.Execute(r.Context(), usecase.CompleteUploadInput{
		PhotobookID: pid,
		ImageID:     imgID,
		StorageKey:  req.StorageKey,
		Now:         h.clock.Now(),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrImageNotFound),
			errors.Is(err, usecase.ErrImageNotUploading),
			errors.Is(err, usecase.ErrStorageKeyMismatch):
			writeFixed(w, http.StatusNotFound, bodyBadRequest)
		case errors.Is(err, usecase.ErrUploadValidationFailed):
			writeFixed(w, http.StatusUnprocessableEntity, bodyValidationFailed)
		default:
			writeFixed(w, http.StatusInternalServerError, bodyServerError)
		}
		return
	}
	resp := completeResponse{
		ImageID: out.ImageID.String(),
		Status:  out.Status,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// === helpers ===

// pidFromURL は chi URL param `{id}` を photobook_id に変換する。
func pidFromURL(r *http.Request) (photobook_id.PhotobookID, bool) {
	raw := chi.URLParam(r, "id")
	if raw == "" {
		return photobook_id.PhotobookID{}, false
	}
	pid, err := parsePhotobookID(raw)
	if err != nil {
		return photobook_id.PhotobookID{}, false
	}
	return pid, true
}

// imageIDFromURL は chi URL param `{imageId}` を image_id に変換する。
func imageIDFromURL(r *http.Request) (image_id.ImageID, bool) {
	raw := chi.URLParam(r, "imageId")
	if raw == "" {
		return image_id.ImageID{}, false
	}
	id, err := parseImageID(raw)
	if err != nil {
		return image_id.ImageID{}, false
	}
	return id, true
}

// bearerToken は Authorization header から Bearer token を取り出す。
func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix {
		return ""
	}
	return auth[len(prefix):]
}

func writeFixed(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
