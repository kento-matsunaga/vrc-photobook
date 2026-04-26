// Package domain は Session 認可機構のドメインモデルを提供する。
//
// 集約相当の位置付け:
//   - Session は集約ではなく **認可機構** の概念単位（docs/design/auth/README.md / ドメイン設計.md）
//   - photobook_id は集約間参照（FK は PR9 で追加）
//
// 設計参照:
//   - docs/design/auth/session/ドメイン設計.md
//   - docs/design/auth/session/データモデル設計.md
//   - docs/adr/0003-frontend-token-session-flow.md
//   - docs/plan/m2-session-auth-implementation-plan.md
//
// セキュリティ:
//   - raw SessionToken は本構造体に保持しない（生成時に呼び出し元へ返し、本体は SessionTokenHash のみ持つ）
package domain

import (
	"errors"
	"fmt"
	"time"

	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
)

// 不変条件・ライフサイクル違反を表すエラー。
var (
	ErrExpiresBeforeCreated     = errors.New("expires_at must be after created_at")
	ErrLastUsedOutOfRange       = errors.New("last_used_at must be within [created_at, expires_at]")
	ErrRevokedBeforeCreated     = errors.New("revoked_at must be after created_at")
	ErrDraftMustHaveZeroVersion = errors.New("draft session must have token_version_at_issue = 0")
)

// Session は draft / manage の認可 session を表す。
//
// 不変条件:
//   - expires_at > created_at（CHECK）
//   - last_used_at は NULL または [created_at, expires_at] 内（CHECK）
//   - revoked_at は NULL または created_at 以降（CHECK）
//   - session_type=draft のとき token_version_at_issue=0（CHECK / I-S5）
type Session struct {
	id                  session_id.SessionID
	tokenHash           session_token_hash.SessionTokenHash
	sessionType         session_type.SessionType
	photobookID         photobook_id.PhotobookID
	tokenVersionAtIssue token_version_at_issue.TokenVersionAtIssue
	expiresAt           time.Time
	createdAt           time.Time
	lastUsedAt          *time.Time
	revokedAt           *time.Time
}

// NewSessionParams は Session のコンストラクタ引数。
//
// 通常フローは IssueDraft / IssueManage（後続 PR の usecase）から呼ぶ。
// Restore は永続化層からの復元のため別関数（Restore）に分ける。
type NewSessionParams struct {
	ID                  session_id.SessionID
	TokenHash           session_token_hash.SessionTokenHash
	SessionType         session_type.SessionType
	PhotobookID         photobook_id.PhotobookID
	TokenVersionAtIssue token_version_at_issue.TokenVersionAtIssue
	ExpiresAt           time.Time
	CreatedAt           time.Time
}

// NewSession は新規発行の Session を組み立てる（last_used_at / revoked_at は nil）。
func NewSession(p NewSessionParams) (Session, error) {
	if !p.ExpiresAt.After(p.CreatedAt) {
		return Session{}, fmt.Errorf("%w: created=%s expires=%s",
			ErrExpiresBeforeCreated, p.CreatedAt, p.ExpiresAt)
	}
	if p.SessionType.IsDraft() && !p.TokenVersionAtIssue.IsZero() {
		return Session{}, ErrDraftMustHaveZeroVersion
	}
	return Session{
		id:                  p.ID,
		tokenHash:           p.TokenHash,
		sessionType:         p.SessionType,
		photobookID:         p.PhotobookID,
		tokenVersionAtIssue: p.TokenVersionAtIssue,
		expiresAt:           p.ExpiresAt,
		createdAt:           p.CreatedAt,
		lastUsedAt:          nil,
		revokedAt:           nil,
	}, nil
}

// RestoreSessionParams は永続化層から Session を復元する際の引数。
// すべてのフィールドを完全に復元するため、CreatedAt / LastUsedAt / RevokedAt を含む。
type RestoreSessionParams struct {
	ID                  session_id.SessionID
	TokenHash           session_token_hash.SessionTokenHash
	SessionType         session_type.SessionType
	PhotobookID         photobook_id.PhotobookID
	TokenVersionAtIssue token_version_at_issue.TokenVersionAtIssue
	ExpiresAt           time.Time
	CreatedAt           time.Time
	LastUsedAt          *time.Time
	RevokedAt           *time.Time
}

// RestoreSession は DB から取得したカラムをドメインに復元する。
//
// 不変条件は CHECK 制約で守られている前提だが、二重防壁として再検証する。
func RestoreSession(p RestoreSessionParams) (Session, error) {
	if !p.ExpiresAt.After(p.CreatedAt) {
		return Session{}, ErrExpiresBeforeCreated
	}
	if p.LastUsedAt != nil {
		if p.LastUsedAt.Before(p.CreatedAt) || p.LastUsedAt.After(p.ExpiresAt) {
			return Session{}, ErrLastUsedOutOfRange
		}
	}
	if p.RevokedAt != nil && p.RevokedAt.Before(p.CreatedAt) {
		return Session{}, ErrRevokedBeforeCreated
	}
	if p.SessionType.IsDraft() && !p.TokenVersionAtIssue.IsZero() {
		return Session{}, ErrDraftMustHaveZeroVersion
	}
	return Session{
		id:                  p.ID,
		tokenHash:           p.TokenHash,
		sessionType:         p.SessionType,
		photobookID:         p.PhotobookID,
		tokenVersionAtIssue: p.TokenVersionAtIssue,
		expiresAt:           p.ExpiresAt,
		createdAt:           p.CreatedAt,
		lastUsedAt:          p.LastUsedAt,
		revokedAt:           p.RevokedAt,
	}, nil
}

// Getters

func (s Session) ID() session_id.SessionID                                 { return s.id }
func (s Session) TokenHash() session_token_hash.SessionTokenHash           { return s.tokenHash }
func (s Session) SessionType() session_type.SessionType                    { return s.sessionType }
func (s Session) PhotobookID() photobook_id.PhotobookID                    { return s.photobookID }
func (s Session) TokenVersionAtIssue() token_version_at_issue.TokenVersionAtIssue {
	return s.tokenVersionAtIssue
}
func (s Session) ExpiresAt() time.Time { return s.expiresAt }
func (s Session) CreatedAt() time.Time { return s.createdAt }
func (s Session) LastUsedAt() *time.Time {
	if s.lastUsedAt == nil {
		return nil
	}
	t := *s.lastUsedAt
	return &t
}
func (s Session) RevokedAt() *time.Time {
	if s.revokedAt == nil {
		return nil
	}
	t := *s.revokedAt
	return &t
}

// IsExpired は now の時点で session が期限切れかどうかを返す。
func (s Session) IsExpired(now time.Time) bool {
	return !now.Before(s.expiresAt)
}

// IsRevoked は revoke 済みかを返す。
func (s Session) IsRevoked() bool {
	return s.revokedAt != nil
}

// IsActive は now 時点で「未 revoke かつ 未期限切れ」を返す。
func (s Session) IsActive(now time.Time) bool {
	return !s.IsRevoked() && !s.IsExpired(now)
}
