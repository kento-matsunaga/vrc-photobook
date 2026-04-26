// Package tests は Session ドメインのテスト用 Builder を提供する。
//
// 方針（.agents/rules/testing.md §Builder パターン）:
//   - Builder は t を保持しない（Build(t) で受け取る）
//   - メソッドテストで前提条件を構築するために使う
//   - コンストラクタテスト（NewSession / RestoreSession 等）では使わず、引数を直接渡す
package tests

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/auth/session/domain"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
)

// SessionBuilder は Session の前提条件を組み立てるための Builder。
//
// 使用例:
//
//	s := tests.NewSessionBuilder().
//	    AsManage().
//	    WithTokenVersion(3).
//	    Build(t)
type SessionBuilder struct {
	id                  session_id.SessionID
	tokenHash           session_token_hash.SessionTokenHash
	sessionType         session_type.SessionType
	photobookID         photobook_id.PhotobookID
	tokenVersionAtIssue token_version_at_issue.TokenVersionAtIssue
	createdAt           time.Time
	expiresAt           time.Time
	hasCustomHash       bool
	hasCustomID         bool
	hasCustomPhotobook  bool
}

// NewSessionBuilder は draft session の既定値を持つ Builder を返す。
func NewSessionBuilder() *SessionBuilder {
	now := time.Now().UTC().Truncate(time.Second)
	return &SessionBuilder{
		sessionType:         session_type.Draft(),
		tokenVersionAtIssue: token_version_at_issue.Zero(),
		createdAt:           now,
		expiresAt:           now.Add(24 * time.Hour),
	}
}

// AsDraft は session_type=draft で組み立てる（既定）。
func (b *SessionBuilder) AsDraft() *SessionBuilder {
	b.sessionType = session_type.Draft()
	b.tokenVersionAtIssue = token_version_at_issue.Zero()
	return b
}

// AsManage は session_type=manage で組み立てる。
func (b *SessionBuilder) AsManage() *SessionBuilder {
	b.sessionType = session_type.Manage()
	return b
}

// WithID は id を上書きする。
func (b *SessionBuilder) WithID(id session_id.SessionID) *SessionBuilder {
	b.id = id
	b.hasCustomID = true
	return b
}

// WithTokenHash は token hash を上書きする。
func (b *SessionBuilder) WithTokenHash(h session_token_hash.SessionTokenHash) *SessionBuilder {
	b.tokenHash = h
	b.hasCustomHash = true
	return b
}

// WithPhotobookID は photobook_id を上書きする。
func (b *SessionBuilder) WithPhotobookID(pid photobook_id.PhotobookID) *SessionBuilder {
	b.photobookID = pid
	b.hasCustomPhotobook = true
	return b
}

// WithTokenVersion は token_version_at_issue を上書きする（manage session で使用）。
func (b *SessionBuilder) WithTokenVersion(v int) *SessionBuilder {
	tv, _ := token_version_at_issue.New(v)
	b.tokenVersionAtIssue = tv
	return b
}

// WithCreatedAt は created_at を上書きする。
func (b *SessionBuilder) WithCreatedAt(t time.Time) *SessionBuilder {
	b.createdAt = t
	return b
}

// WithExpiresAt は expires_at を上書きする。
func (b *SessionBuilder) WithExpiresAt(t time.Time) *SessionBuilder {
	b.expiresAt = t
	return b
}

// Build は Session を生成する。t.Helper を呼ぶ。
func (b *SessionBuilder) Build(t *testing.T) domain.Session {
	t.Helper()
	if !b.hasCustomID {
		id, err := session_id.New()
		if err != nil {
			t.Fatalf("session_id.New: %v", err)
		}
		b.id = id
	}
	if !b.hasCustomHash {
		tok, err := session_token.Generate()
		if err != nil {
			t.Fatalf("session_token.Generate: %v", err)
		}
		b.tokenHash = session_token_hash.Of(tok)
	}
	if !b.hasCustomPhotobook {
		pid, err := photobook_id.FromUUID(uuid.New())
		if err != nil {
			t.Fatalf("photobook_id.FromUUID: %v", err)
		}
		b.photobookID = pid
	}
	s, err := domain.NewSession(domain.NewSessionParams{
		ID:                  b.id,
		TokenHash:           b.tokenHash,
		SessionType:         b.sessionType,
		PhotobookID:         b.photobookID,
		TokenVersionAtIssue: b.tokenVersionAtIssue,
		CreatedAt:           b.createdAt,
		ExpiresAt:           b.expiresAt,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return s
}
