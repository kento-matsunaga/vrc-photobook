// Package tests は usecase / middleware のテスト用ヘルパを提供する。
//
// FakeRepository は usecase.SessionRepository を実装する in-memory 実装。
// 実 DB を使わずに UseCase / middleware の振る舞いを検証する。
//
// 設計:
//   - rdb.SessionRepository（実 DB）と同じ意味論を持つ
//   - 「revoked_at IS NULL AND expires_at > now()」相当のフィルタを FindActiveByHash で再現
//   - スレッドセーフは保証しない（並行アクセスのテストでは別途 mutex を被せる）
package tests

import (
	"context"
	"errors"
	"time"

	"vrcpb/backend/internal/auth/session/domain"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/infrastructure/repository/rdb"
)

// FakeRepository は in-memory な usecase.SessionRepository 実装。
type FakeRepository struct {
	// items は session_id → 内部レコード のマップ。
	items map[session_id.SessionID]*record

	// Now はテストから時刻を制御するためのフック。nil の場合は time.Now を使う。
	Now func() time.Time

	// CreateErr / FindErr / TouchErr / RevokeErr / RevokeAllDraftsErr / RevokeAllManageErr
	// はテストから注入可能なエラー。nil なら通常動作。
	CreateErr            error
	FindErr              error
	TouchErr             error
	RevokeErr            error
	RevokeAllDraftsErr   error
	RevokeAllManageErr   error

	// CreateCalls 等は呼び出し回数。
	CreateCalls            int
	FindCalls              int
	TouchCalls             int
	RevokeCalls            int
	RevokeAllDraftsCalls   int
	RevokeAllManageCalls   int
}

// record は内部レコード。Session は immutable なので mutable な lastUsed / revoked を別管理する。
type record struct {
	session    domain.Session
	lastUsedAt *time.Time
	revokedAt  *time.Time
}

// NewFakeRepository はからの FakeRepository を作る。
func NewFakeRepository() *FakeRepository {
	return &FakeRepository{items: make(map[session_id.SessionID]*record)}
}

func (r *FakeRepository) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now().UTC()
}

// Create は session を保存する。同一 hash の重複は unique 違反として扱う。
func (r *FakeRepository) Create(_ context.Context, s domain.Session) error {
	r.CreateCalls++
	if r.CreateErr != nil {
		return r.CreateErr
	}
	for _, ex := range r.items {
		if ex.session.TokenHash().Equal(s.TokenHash()) {
			return errors.New("fake: duplicate session_token_hash")
		}
	}
	r.items[s.ID()] = &record{session: s}
	return nil
}

// FindActiveByHash は hash + type + photobook_id 完全一致 + 未 revoke + 未期限切れ の session を返す。
func (r *FakeRepository) FindActiveByHash(
	_ context.Context,
	hash session_token_hash.SessionTokenHash,
	t session_type.SessionType,
	pid photobook_id.PhotobookID,
) (domain.Session, error) {
	r.FindCalls++
	if r.FindErr != nil {
		return domain.Session{}, r.FindErr
	}
	now := r.now()
	for _, rec := range r.items {
		s := rec.session
		if !s.TokenHash().Equal(hash) {
			continue
		}
		if !s.SessionType().Equal(t) {
			continue
		}
		if !s.PhotobookID().Equal(pid) {
			continue
		}
		if rec.revokedAt != nil {
			continue
		}
		if !now.Before(s.ExpiresAt()) {
			continue
		}
		// 復元: lastUsedAt / revokedAt を反映した Session を返す
		out, err := domain.RestoreSession(domain.RestoreSessionParams{
			ID:                  s.ID(),
			TokenHash:           s.TokenHash(),
			SessionType:         s.SessionType(),
			PhotobookID:         s.PhotobookID(),
			TokenVersionAtIssue: s.TokenVersionAtIssue(),
			ExpiresAt:           s.ExpiresAt(),
			CreatedAt:           s.CreatedAt(),
			LastUsedAt:          rec.lastUsedAt,
			RevokedAt:           rec.revokedAt,
		})
		if err != nil {
			return domain.Session{}, err
		}
		return out, nil
	}
	return domain.Session{}, rdb.ErrNotFound
}

// Touch は last_used_at を now() に更新する。
func (r *FakeRepository) Touch(_ context.Context, id session_id.SessionID) error {
	r.TouchCalls++
	if r.TouchErr != nil {
		return r.TouchErr
	}
	rec, ok := r.items[id]
	if !ok || rec.revokedAt != nil || !r.now().Before(rec.session.ExpiresAt()) {
		return rdb.ErrNotFound
	}
	now := r.now()
	rec.lastUsedAt = &now
	return nil
}

// Revoke は revoked_at を now() に立てる。
func (r *FakeRepository) Revoke(_ context.Context, id session_id.SessionID) error {
	r.RevokeCalls++
	if r.RevokeErr != nil {
		return r.RevokeErr
	}
	rec, ok := r.items[id]
	if !ok || rec.revokedAt != nil {
		return rdb.ErrNotFound
	}
	now := r.now()
	rec.revokedAt = &now
	return nil
}

// RevokeAllDrafts は photobook_id 配下の draft session すべてを revoke する。
func (r *FakeRepository) RevokeAllDrafts(
	_ context.Context,
	pid photobook_id.PhotobookID,
) (int64, error) {
	r.RevokeAllDraftsCalls++
	if r.RevokeAllDraftsErr != nil {
		return 0, r.RevokeAllDraftsErr
	}
	now := r.now()
	var n int64
	for _, rec := range r.items {
		if rec.revokedAt != nil {
			continue
		}
		if !rec.session.PhotobookID().Equal(pid) {
			continue
		}
		if !rec.session.SessionType().IsDraft() {
			continue
		}
		t := now
		rec.revokedAt = &t
		n++
	}
	return n, nil
}

// RevokeAllManageByTokenVersion は manage で token_version_at_issue <= oldVersion を revoke。
func (r *FakeRepository) RevokeAllManageByTokenVersion(
	_ context.Context,
	pid photobook_id.PhotobookID,
	oldVersion int,
) (int64, error) {
	r.RevokeAllManageCalls++
	if r.RevokeAllManageErr != nil {
		return 0, r.RevokeAllManageErr
	}
	now := r.now()
	var n int64
	for _, rec := range r.items {
		if rec.revokedAt != nil {
			continue
		}
		if !rec.session.PhotobookID().Equal(pid) {
			continue
		}
		if !rec.session.SessionType().IsManage() {
			continue
		}
		if rec.session.TokenVersionAtIssue().Int() > oldVersion {
			continue
		}
		t := now
		rec.revokedAt = &t
		n++
	}
	return n, nil
}

// MarkRevoked はテスト直接操作用。明示的に revoke したい場合に使う。
func (r *FakeRepository) MarkRevoked(id session_id.SessionID, at time.Time) {
	if rec, ok := r.items[id]; ok {
		rec.revokedAt = &at
	}
}
