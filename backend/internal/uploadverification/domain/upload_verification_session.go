// Package domain は Upload Verification Session のドメインモデルを提供する。
//
// 設計参照:
//   - docs/adr/0005-image-upload-flow.md §Turnstile 検証
//   - docs/plan/m2-upload-verification-plan.md
//
// 役割:
//   - Cloudflare Turnstile 検証成功後に発行される短命 session
//   - 1 検証あたり 30 分 / 20 intent
//   - 対象 Photobook ID に紐付く（他 photobook へ流用不可）
//
// セキュリティ:
//   - raw token は entity に保持しない（発行直後の戻り値経由でのみ伝播）
//   - DB には SHA-256 hash のみを保存
//   - photobook_id 不一致 / 期限切れ / 回数超過 / revoked は consume 時に拒否
package domain

import (
	"errors"
	"time"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/uploadverification/domain/vo/intent_count"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_id"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token_hash"
)

// MVP 既定値（ADR-0005）。
const (
	DefaultTTL          = 30 * time.Minute
	DefaultAllowedCount = 20
)

// 不変条件・状態遷移エラー。
var (
	ErrAllowedNotPositive    = errors.New("allowed_intent_count must be > 0")
	ErrUsedExceedsAllowed    = errors.New("used_intent_count must not exceed allowed_intent_count")
	ErrExpiresInPast         = errors.New("expires_at must be in the future")
	ErrInvalidStateForRestore = errors.New("invalid state combination for restore")
)

// UploadVerificationSession は Turnstile 検証成功後に発行される short-lived session。
type UploadVerificationSession struct {
	id                verification_session_id.VerificationSessionID
	photobookID       photobook_id.PhotobookID
	tokenHash         verification_session_token_hash.VerificationSessionTokenHash
	allowedIntentCount intent_count.IntentCount
	usedIntentCount    intent_count.IntentCount
	expiresAt         time.Time
	createdAt         time.Time
	revokedAt         *time.Time
}

// NewParams は新規発行の引数。
type NewParams struct {
	ID          verification_session_id.VerificationSessionID
	PhotobookID photobook_id.PhotobookID
	TokenHash   verification_session_token_hash.VerificationSessionTokenHash
	Allowed     intent_count.IntentCount
	Now         time.Time
	TTL         time.Duration // 0 なら DefaultTTL
}

// New は新規 UploadVerificationSession を組み立てる。
//
// allowed = 0 なら ErrAllowedNotPositive。TTL = 0 なら 30 分既定。
func New(p NewParams) (UploadVerificationSession, error) {
	if p.Allowed.Int() <= 0 {
		return UploadVerificationSession{}, ErrAllowedNotPositive
	}
	if p.Now.IsZero() {
		return UploadVerificationSession{}, ErrInvalidStateForRestore
	}
	ttl := p.TTL
	if ttl == 0 {
		ttl = DefaultTTL
	}
	if ttl <= 0 {
		return UploadVerificationSession{}, ErrExpiresInPast
	}
	return UploadVerificationSession{
		id:                 p.ID,
		photobookID:        p.PhotobookID,
		tokenHash:          p.TokenHash,
		allowedIntentCount: p.Allowed,
		usedIntentCount:    intent_count.Zero(),
		expiresAt:          p.Now.Add(ttl),
		createdAt:          p.Now,
	}, nil
}

// RestoreParams は DB から復元する引数。
type RestoreParams struct {
	ID                 verification_session_id.VerificationSessionID
	PhotobookID        photobook_id.PhotobookID
	TokenHash          verification_session_token_hash.VerificationSessionTokenHash
	AllowedIntentCount intent_count.IntentCount
	UsedIntentCount    intent_count.IntentCount
	ExpiresAt          time.Time
	CreatedAt          time.Time
	RevokedAt          *time.Time
}

// Restore は DB row を session に復元する。
func Restore(p RestoreParams) (UploadVerificationSession, error) {
	if p.AllowedIntentCount.Int() <= 0 {
		return UploadVerificationSession{}, ErrAllowedNotPositive
	}
	if p.UsedIntentCount.Int() > p.AllowedIntentCount.Int() {
		return UploadVerificationSession{}, ErrUsedExceedsAllowed
	}
	return UploadVerificationSession{
		id:                 p.ID,
		photobookID:        p.PhotobookID,
		tokenHash:          p.TokenHash,
		allowedIntentCount: p.AllowedIntentCount,
		usedIntentCount:    p.UsedIntentCount,
		expiresAt:          p.ExpiresAt,
		createdAt:          p.CreatedAt,
		revokedAt:          clonePtrTime(p.RevokedAt),
	}, nil
}

// CanConsume は現在 consume 可能かを返す（domain 側の即時判定）。
//
// DB レベルの整合性は Repository.ConsumeOne の atomic UPDATE で再保証される。
// 本メソッドは UseCase / handler 層で「短絡的に拒否してよいか」を判定するヘルパ。
func (s UploadVerificationSession) CanConsume(now time.Time) bool {
	if s.revokedAt != nil {
		return false
	}
	if !s.expiresAt.After(now) {
		return false
	}
	if s.usedIntentCount.Int() >= s.allowedIntentCount.Int() {
		return false
	}
	return true
}

// IsRevoked は revoke 済みか。
func (s UploadVerificationSession) IsRevoked() bool { return s.revokedAt != nil }

// アクセサ。
func (s UploadVerificationSession) ID() verification_session_id.VerificationSessionID { return s.id }
func (s UploadVerificationSession) PhotobookID() photobook_id.PhotobookID             { return s.photobookID }
func (s UploadVerificationSession) TokenHash() verification_session_token_hash.VerificationSessionTokenHash {
	return s.tokenHash
}
func (s UploadVerificationSession) AllowedIntentCount() intent_count.IntentCount { return s.allowedIntentCount }
func (s UploadVerificationSession) UsedIntentCount() intent_count.IntentCount    { return s.usedIntentCount }
func (s UploadVerificationSession) ExpiresAt() time.Time                         { return s.expiresAt }
func (s UploadVerificationSession) CreatedAt() time.Time                         { return s.createdAt }
func (s UploadVerificationSession) RevokedAt() *time.Time                        { return clonePtrTime(s.revokedAt) }

func clonePtrTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}
