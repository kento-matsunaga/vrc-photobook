// Turnstile + upload_verification_sessions の sandbox エンドポイント。
//
// セキュリティ方針:
//   - secret / Turnstile token / verification_session_token そのものをログに出さない
//   - DB には raw token を保存せず SHA-256 ハッシュのみ保存（bytea）
//   - クライアントへは「分類キー（拒否理由カテゴリ）」だけ返し、内部詳細は出さない
//
// マッピング:
//   - POST /sandbox/turnstile/verify         → Turnstile siteverify + セッション発行
//   - POST /sandbox/upload-intent/consume    → アトミック消費（残数確認 + +1）
package sandbox

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/spike-backend/internal/config"
	"vrcpb/spike-backend/internal/db/sqlcgen"
	"vrcpb/spike-backend/internal/turnstile"
)

// turnstileVerifyRequest は POST /sandbox/turnstile/verify のリクエスト body。
type turnstileVerifyRequest struct {
	TurnstileToken string `json:"turnstile_token"`
	PhotobookID    string `json:"photobook_id"`
}

// turnstileVerifyResponse はセッション発行レスポンス。
//
// 返す verification_session_token は raw 値（クライアントは以降この値を提示する）。
// DB 上には SHA-256 ハッシュのみ保存しており、漏洩時もハッシュからの復元は困難。
type turnstileVerifyResponse struct {
	VerificationSessionToken string `json:"verification_session_token"`
	ExpiresInSeconds         int    `json:"expires_in_seconds"`
	AllowedIntentCount       int32  `json:"allowed_intent_count"`
}

// TurnstileVerify は Turnstile siteverify を呼んで成功なら upload_verification_session を発行する。
//
// フロー:
//  1. token の事前バリデーション（空文字・photobook_id UUID 形式）
//  2. turnstile.Verify
//  3. 成功時のみ 32 バイト乱数を base64url で生成 → SHA-256 で hash
//  4. CreateUploadVerificationSession で永続化
//  5. raw token をレスポンスに返す（DB には hash のみ）
func TurnstileVerify(client *turnstile.Client, queries *sqlcgen.Queries, cfg config.UploadVerificationConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if client == nil || queries == nil {
			writeError(w, http.StatusServiceUnavailable, "turnstile_not_configured")
			return
		}

		var req turnstileVerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if strings.TrimSpace(req.TurnstileToken) == "" {
			writeError(w, http.StatusBadRequest, "turnstile_token_required")
			return
		}
		photobookUUID, err := uuid.Parse(strings.TrimSpace(req.PhotobookID))
		if err != nil {
			writeError(w, http.StatusBadRequest, "photobook_id_invalid")
			return
		}

		// Turnstile siteverify。サーバ側ログには error-codes を残すが、レスポンスには出さない。
		result, err := client.Verify(r.Context(), req.TurnstileToken)
		if err != nil {
			slog.Warn("turnstile verify call failed", "error", err.Error())
			writeError(w, http.StatusBadGateway, "turnstile_call_failed")
			return
		}
		if !result.Success {
			slog.Info("turnstile verify rejected",
				"error_codes", result.ErrorCodes,
				"mock_mode", client.IsMock())
			writeError(w, http.StatusForbidden, "turnstile_rejected")
			return
		}

		// 32 バイト = 256bit の暗号論的乱数 → base64url（パディングなし）
		rawToken, err := newRandomBase64URL(32)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "token_generation_failed")
			return
		}
		hash := sha256.Sum256([]byte(rawToken))

		sessionID, err := newPgUUIDv4()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "session_id_generation_failed")
			return
		}
		expiresAt := pgtype.Timestamptz{Time: time.Now().UTC().Add(cfg.SessionTTL), Valid: true}

		row, err := queries.CreateUploadVerificationSession(r.Context(),
			sqlcgen.CreateUploadVerificationSessionParams{
				ID:                 sessionID,
				SessionTokenHash:   hash[:],
				PhotobookID:        toPgUUID(photobookUUID),
				AllowedIntentCount: cfg.AllowedIntentCount,
				ExpiresAt:          expiresAt,
			})
		if err != nil {
			slog.Warn("create upload verification session failed", "error", err.Error())
			writeError(w, http.StatusInternalServerError, "session_create_failed")
			return
		}

		writeJSON(w, http.StatusOK, turnstileVerifyResponse{
			VerificationSessionToken: rawToken,
			ExpiresInSeconds:         int(cfg.SessionTTL / time.Second),
			AllowedIntentCount:       row.AllowedIntentCount,
		})
	}
}

// uploadIntentConsumeRequest は POST /sandbox/upload-intent/consume のリクエスト body。
type uploadIntentConsumeRequest struct {
	VerificationSessionToken string `json:"verification_session_token"`
	PhotobookID              string `json:"photobook_id"`
}

// uploadIntentConsumeResponse は消費成功時のレスポンス。
type uploadIntentConsumeResponse struct {
	Consumed           bool  `json:"consumed"`
	UsedIntentCount    int32 `json:"used_intent_count"`
	AllowedIntentCount int32 `json:"allowed_intent_count"`
	Remaining          int32 `json:"remaining"`
}

// UploadIntentConsume はアトミック条件 UPDATE で intent を 1 消費する。
//
// 拒否分類:
//   - body 不正 → 400
//   - hash 一致しない / 期限切れ / revoked / 残数なし のいずれか → 403 (consume_rejected)
//     ※ どれに該当するかをクライアントには返さない（情報漏洩を避ける）。
//     PoC では理由を slog に出して観察できるようにする。
func UploadIntentConsume(queries *sqlcgen.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if queries == nil {
			writeError(w, http.StatusServiceUnavailable, "consume_not_configured")
			return
		}

		var req uploadIntentConsumeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if strings.TrimSpace(req.VerificationSessionToken) == "" {
			writeError(w, http.StatusBadRequest, "session_token_required")
			return
		}
		photobookUUID, err := uuid.Parse(strings.TrimSpace(req.PhotobookID))
		if err != nil {
			writeError(w, http.StatusBadRequest, "photobook_id_invalid")
			return
		}

		hash := sha256.Sum256([]byte(req.VerificationSessionToken))

		row, err := queries.ConsumeUploadVerificationIntent(r.Context(),
			sqlcgen.ConsumeUploadVerificationIntentParams{
				SessionTokenHash: hash[:],
				PhotobookID:      toPgUUID(photobookUUID),
			})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// hash mismatch / expired / revoked / exhausted のいずれか。
				// 内訳は GetUploadVerificationSessionByHash を呼ばずに集約して 403 で返す（情報量を絞る）。
				slog.Info("upload intent consume rejected", "reason", "no_rows")
				writeError(w, http.StatusForbidden, "consume_rejected")
				return
			}
			slog.Warn("upload intent consume failed", "error", err.Error())
			writeError(w, http.StatusInternalServerError, "consume_failed")
			return
		}

		writeJSON(w, http.StatusOK, uploadIntentConsumeResponse{
			Consumed:           true,
			UsedIntentCount:    row.UsedIntentCount,
			AllowedIntentCount: row.AllowedIntentCount,
			Remaining:          row.AllowedIntentCount - row.UsedIntentCount,
		})
	}
}

// newPgUUIDv4 は uuid v4 を pgtype.UUID に変換した値を返す。
func newPgUUIDv4() (pgtype.UUID, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return pgtype.UUID{}, err
	}
	return toPgUUID(id), nil
}

func toPgUUID(id uuid.UUID) pgtype.UUID {
	var pg pgtype.UUID
	copy(pg.Bytes[:], id[:])
	pg.Valid = true
	return pg
}

