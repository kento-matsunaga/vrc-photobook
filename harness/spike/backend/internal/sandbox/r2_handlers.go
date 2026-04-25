// R2 接続検証用 sandbox エンドポイント。
//
// セキュリティ方針:
//   - presigned URL / storage_key / R2 認証情報をログに出さない（slog 出力対象から除外）
//   - エラー詳細はサーバ側で捕捉、レスポンスには分類キーのみ返す
//   - storage_key は ADR-0005 の命名規則に揃える
package sandbox

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	r2 "vrcpb/spike-backend/internal/r2"
)

// 共通定数。M1 PoC のため固定値。本実装では UseCase が ID を渡す。
const (
	pocPhotobookID  = "00000000-0000-0000-0000-000000000001"
	pocPathPrefix   = "photobooks/"
	maxByteSize     = 10 * 1024 * 1024 // 10MB
	presignDuration = 15 * time.Minute  // ADR-0005
	listMaxKeys     = 5
)

// 許可 content_type と拡張子の対応。
// SVG / GIF など明示的に拒否したい形式は含めない（含まれていなければ unsupported）。
var allowedContentTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/heic": "heic",
	"image/heif": "heif",
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

// R2HeadBucket は R2 への接続確認（HeadBucket）を行う。
func R2HeadBucket(client *r2.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			writeError(w, http.StatusServiceUnavailable, "r2_not_configured")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := client.HeadBucket(ctx); err != nil {
			// エラー詳細はクライアントに返さない（サーバ側ログで追跡）
			writeError(w, http.StatusBadGateway, "r2_headbucket_failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// R2List は最大 listMaxKeys 件のオブジェクトを返す。
func R2List(client *r2.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			writeError(w, http.StatusServiceUnavailable, "r2_not_configured")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		objects, err := client.ListObjects(ctx, listMaxKeys)
		if err != nil {
			writeError(w, http.StatusBadGateway, "r2_list_failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"count":   len(objects),
			"objects": objects,
		})
	}
}

// presignPutRequest は POST /sandbox/r2-presign-put のリクエスト body。
type presignPutRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
}

// presignPutResponse はレスポンス body。
// upload_url は raw な presigned URL。レスポンスに含めるが、サーバ側ログには出さない方針。
type presignPutResponse struct {
	UploadURL        string `json:"upload_url"`
	StorageKey       string `json:"storage_key"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

// R2PresignPut は presigned PUT URL を発行する。
//
// バリデーション:
//   - filename 必須
//   - content_type は allowedContentTypes に含まれること
//   - byte_size は 1〜10MB
//
// storage_key は ADR-0005 の命名規則:
//   photobooks/{photobook_id}/images/{image_id}/original/{random}.{ext}
func R2PresignPut(client *r2.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			writeError(w, http.StatusServiceUnavailable, "r2_not_configured")
			return
		}

		var req presignPutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if strings.TrimSpace(req.Filename) == "" {
			writeError(w, http.StatusBadRequest, "filename_required")
			return
		}
		if req.ByteSize <= 0 {
			writeError(w, http.StatusBadRequest, "byte_size_invalid")
			return
		}
		if req.ByteSize > maxByteSize {
			writeError(w, http.StatusBadRequest, "file_too_large")
			return
		}
		ext, ok := allowedContentTypes[req.ContentType]
		if !ok {
			// SVG / GIF / その他は unsupported_format
			writeError(w, http.StatusBadRequest, "unsupported_format")
			return
		}

		key, err := generateStorageKey(ext)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "storage_key_generation_failed")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		uploadURL, err := client.PresignPut(ctx, key, req.ContentType, req.ByteSize, presignDuration)
		if err != nil {
			writeError(w, http.StatusBadGateway, "r2_presign_failed")
			return
		}

		writeJSON(w, http.StatusOK, presignPutResponse{
			UploadURL:        uploadURL,
			StorageKey:       key,
			ExpiresInSeconds: int(presignDuration / time.Second),
		})
	}
}

// R2HeadObject は ?key=... のオブジェクト存在を確認する。
//
// バリデーション:
//   - key 空文字 → 400
//   - key が photobooks/ で始まらない → 400（パストラバーサル等防止）
//   - key に "../" を含む → 400
func R2HeadObject(client *r2.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			writeError(w, http.StatusServiceUnavailable, "r2_not_configured")
			return
		}
		key := r.URL.Query().Get("key")
		if key == "" {
			writeError(w, http.StatusBadRequest, "key_required")
			return
		}
		if !strings.HasPrefix(key, pocPathPrefix) {
			writeError(w, http.StatusBadRequest, "key_prefix_invalid")
			return
		}
		if strings.Contains(key, "../") {
			writeError(w, http.StatusBadRequest, "key_traversal_forbidden")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		head, err := client.HeadObject(ctx, key)
		if err != nil {
			// エラー詳細は返さない。NoSuchKey でも 404 ではなく 502 系でラフに返す（PoC のため）
			writeError(w, http.StatusBadGateway, "r2_headobject_failed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"content_length": head.ContentLength,
			"content_type":   head.ContentType,
			"etag":           head.ETag,
		})
	}
}

// generateStorageKey は ADR-0005 の命名規則に従って storage_key を生成する。
// photobook_id / image_id は M1 PoC では固定 + 都度生成（image_id は新規ランダム）。
func generateStorageKey(ext string) (string, error) {
	imageID, err := newRandomID()
	if err != nil {
		return "", err
	}
	random12, err := newRandomBase64URL(12)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("photobooks/%s/images/%s/original/%s.%s",
		pocPhotobookID, imageID, random12, ext), nil
}

// newRandomID は RFC 4122 v4 風の UUID 文字列を返す（PoC の image_id 用）。
// 本実装の DB 内部 ID は UUIDv7（ADR-0001）だが、PoC ではダミー識別子で十分。
func newRandomID() (string, error) {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// newRandomBase64URL は n バイトの暗号論的乱数を base64url で返す（パディングなし）。
func newRandomBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := crand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// 静的検査: サンドボックス内で errors パッケージを使う（将来拡張用）。
var _ = errors.New
