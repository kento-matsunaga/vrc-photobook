// Package rdb は Session 認可機構の RDB Repository 実装を提供する。
//
// 設計参照:
//   - docs/design/auth/session/データモデル設計.md
//   - docs/plan/m2-session-auth-implementation-plan.md §9
//
// セキュリティ:
//   - すべての参照系は sqlc の FindActiveSessionByHash を経由する
//     （revoked_at IS NULL AND expires_at > now() を必ず通る）
//   - raw SQL を直接書ける経路は本パッケージ外には公開しない
package rdb

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/auth/session/domain"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/infrastructure/repository/rdb/marshaller"
	"vrcpb/backend/internal/auth/session/infrastructure/repository/rdb/sqlcgen"
)

// ErrNotFound は session が存在しない / 既に revoke 済 / 期限切れのときに返す。
var ErrNotFound = errors.New("session not found")

// SessionRepository は session の永続化操作を提供する。
type SessionRepository struct {
	q *sqlcgen.Queries
}

// NewSessionRepository は pgx pool または tx（sqlcgen.DBTX を満たすもの）を受け取って Repository を作る。
func NewSessionRepository(db sqlcgen.DBTX) *SessionRepository {
	return &SessionRepository{q: sqlcgen.New(db)}
}

// Create は新規 session を INSERT する。
func (r *SessionRepository) Create(ctx context.Context, s domain.Session) error {
	params, err := marshaller.ToCreateParams(s)
	if err != nil {
		return err
	}
	return r.q.CreateSession(ctx, params)
}

// FindActiveByHash は session_token_hash + session_type + photobook_id で
// 有効な（未 revoke / 未期限切れ）session を 1 件取り出す。
//
// 該当なし時は ErrNotFound を返す。
func (r *SessionRepository) FindActiveByHash(
	ctx context.Context,
	hash session_token_hash.SessionTokenHash,
	t session_type.SessionType,
	pid photobook_id.PhotobookID,
) (domain.Session, error) {
	row, err := r.q.FindActiveSessionByHash(ctx, sqlcgen.FindActiveSessionByHashParams{
		SessionTokenHash: hash.Bytes(),
		SessionType:      t.String(),
		PhotobookID:      pgtype.UUID{Bytes: pid.UUID(), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, ErrNotFound
		}
		return domain.Session{}, err
	}
	return marshaller.FromRow(row)
}

// Touch は last_used_at を now() に更新する。
// session が無効（revoke / 期限切れ）の場合は ErrNotFound。
func (r *SessionRepository) Touch(ctx context.Context, id session_id.SessionID) error {
	rows, err := r.q.TouchSession(ctx, pgtype.UUID{Bytes: id.UUID(), Valid: true})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Revoke は session_id に対して revoked_at = now() を立てる。
// 既に revoke 済 / 存在しない場合は ErrNotFound。
func (r *SessionRepository) Revoke(ctx context.Context, id session_id.SessionID) error {
	rows, err := r.q.RevokeSessionByID(ctx, pgtype.UUID{Bytes: id.UUID(), Valid: true})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeAllDrafts は photobook_id 配下の全 draft session を revoke する。
// 影響行数を返す（ゼロ行でもエラーにしない、Photobook publish の冪等性のため）。
func (r *SessionRepository) RevokeAllDrafts(
	ctx context.Context,
	pid photobook_id.PhotobookID,
) (int64, error) {
	return r.q.RevokeAllDraftsByPhotobook(ctx, pgtype.UUID{Bytes: pid.UUID(), Valid: true})
}

// RevokeAllManageByTokenVersion は photobook_id 配下の manage session のうち、
// token_version_at_issue <= oldVersion のものを revoke する。
// 影響行数を返す。
func (r *SessionRepository) RevokeAllManageByTokenVersion(
	ctx context.Context,
	pid photobook_id.PhotobookID,
	oldVersion int,
) (int64, error) {
	return r.q.RevokeAllManageByTokenVersion(ctx, sqlcgen.RevokeAllManageByTokenVersionParams{
		PhotobookID:         pgtype.UUID{Bytes: pid.UUID(), Valid: true},
		TokenVersionAtIssue: int32(oldVersion),
	})
}

// DeleteExpired は GC 対象（revoke から 30 日 / 期限切れから 7 日）の session を物理削除する。
// 後続 PR の cmd/ops から呼び出す前提。
func (r *SessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	return r.q.DeleteExpiredSessions(ctx)
}
