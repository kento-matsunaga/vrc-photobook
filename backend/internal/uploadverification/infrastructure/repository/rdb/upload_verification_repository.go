// Package rdb は UploadVerificationSession の RDB Repository。
//
// 設計参照:
//   - docs/plan/m2-upload-verification-plan.md §7 atomic consume
//
// セキュリティ:
//   - consume は単一 UPDATE で atomic（PostgreSQL row-level lock で並行直列化）
//   - 0 行影響は ErrUploadVerificationFailed に集約（理由を外に出さない）
//   - raw token は本パッケージに渡されない（hash のみ受け取る）
package rdb

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/uploadverification/domain"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_id"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token_hash"
	"vrcpb/backend/internal/uploadverification/infrastructure/repository/rdb/marshaller"
	"vrcpb/backend/internal/uploadverification/infrastructure/repository/rdb/sqlcgen"
)

// ビジネス例外。
var (
	ErrNotFound                 = errors.New("upload verification session not found")
	ErrUploadVerificationFailed = errors.New("upload verification failed (expired / used up / revoked / mismatch)")
)

// ConsumeOutput は ConsumeOne の結果。
type ConsumeOutput struct {
	ID                 verification_session_id.VerificationSessionID
	UsedIntentCount    int
	AllowedIntentCount int
}

// UploadVerificationSessionRepository は upload_verification_sessions への永続化。
type UploadVerificationSessionRepository struct {
	q *sqlcgen.Queries
}

// NewUploadVerificationSessionRepository は pgx pool / tx から Repository を作る。
func NewUploadVerificationSessionRepository(db sqlcgen.DBTX) *UploadVerificationSessionRepository {
	return &UploadVerificationSessionRepository{q: sqlcgen.New(db)}
}

// Create は新規 session を INSERT する。
func (r *UploadVerificationSessionRepository) Create(
	ctx context.Context,
	s domain.UploadVerificationSession,
) error {
	return r.q.CreateUploadVerificationSession(ctx, marshaller.ToCreateParams(s))
}

// FindByID は id 一致の session を返す。
func (r *UploadVerificationSessionRepository) FindByID(
	ctx context.Context,
	id verification_session_id.VerificationSessionID,
) (domain.UploadVerificationSession, error) {
	row, err := r.q.FindUploadVerificationSessionByID(ctx,
		pgtype.UUID{Bytes: id.UUID(), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.UploadVerificationSession{}, ErrNotFound
		}
		return domain.UploadVerificationSession{}, err
	}
	return marshaller.FromRow(row)
}

// ConsumeOne は token_hash + photobook_id でセッションを atomic に 1 回 consume する。
//
// 0 行影響時は ErrUploadVerificationFailed（理由を区別しない）。
func (r *UploadVerificationSessionRepository) ConsumeOne(
	ctx context.Context,
	tokenHash verification_session_token_hash.VerificationSessionTokenHash,
	pid photobook_id.PhotobookID,
) (ConsumeOutput, error) {
	row, err := r.q.ConsumeUploadVerificationSession(ctx, sqlcgen.ConsumeUploadVerificationSessionParams{
		SessionTokenHash: tokenHash.Bytes(),
		PhotobookID:      pgtype.UUID{Bytes: pid.UUID(), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConsumeOutput{}, ErrUploadVerificationFailed
		}
		return ConsumeOutput{}, err
	}
	id, err := verification_session_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return ConsumeOutput{}, err
	}
	return ConsumeOutput{
		ID:                 id,
		UsedIntentCount:    int(row.UsedIntentCount),
		AllowedIntentCount: int(row.AllowedIntentCount),
	}, nil
}

// Revoke は明示 revoke。0 行影響でも error は返さない（既に revoke 済 / 不存在）。
// 呼び出し側は冪等扱い。
func (r *UploadVerificationSessionRepository) Revoke(
	ctx context.Context,
	id verification_session_id.VerificationSessionID,
	revokedAt pgtype.Timestamptz,
) error {
	if _, err := r.q.RevokeUploadVerificationSession(ctx, sqlcgen.RevokeUploadVerificationSessionParams{
		ID:        pgtype.UUID{Bytes: id.UUID(), Valid: true},
		RevokedAt: revokedAt,
	}); err != nil {
		return err
	}
	return nil
}
