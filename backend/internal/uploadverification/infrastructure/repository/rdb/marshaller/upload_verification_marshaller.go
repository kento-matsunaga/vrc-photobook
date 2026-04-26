// Package marshaller は UploadVerificationSession と sqlc 生成物 row の相互変換。
//
// 設計方針:
//   - VO ↔ プリミティブ変換は本パッケージに閉じる
//   - DB エラーはそのまま返し、ビジネス例外への変換は repository が担う
package marshaller

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/uploadverification/domain"
	"vrcpb/backend/internal/uploadverification/domain/vo/intent_count"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_id"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token_hash"
	"vrcpb/backend/internal/uploadverification/infrastructure/repository/rdb/sqlcgen"
)

// ErrInvalidRow は DB row を VO に変換できないとき。
var ErrInvalidRow = errors.New("invalid upload verification session row from db")

// ToCreateParams は新規 UploadVerificationSession を sqlc CreateUploadVerificationSessionParams
// に変換する。
func ToCreateParams(s domain.UploadVerificationSession) sqlcgen.CreateUploadVerificationSessionParams {
	return sqlcgen.CreateUploadVerificationSessionParams{
		ID:                 pgtype.UUID{Bytes: s.ID().UUID(), Valid: true},
		PhotobookID:        pgtype.UUID{Bytes: s.PhotobookID().UUID(), Valid: true},
		SessionTokenHash:   s.TokenHash().Bytes(),
		AllowedIntentCount: int32(s.AllowedIntentCount().Int()),
		ExpiresAt:          pgtype.Timestamptz{Time: s.ExpiresAt(), Valid: true},
		CreatedAt:          pgtype.Timestamptz{Time: s.CreatedAt(), Valid: true},
	}
}

// FromRow は sqlcgen.UploadVerificationSession を domain に復元する。
func FromRow(row sqlcgen.UploadVerificationSession) (domain.UploadVerificationSession, error) {
	if !row.ID.Valid {
		return domain.UploadVerificationSession{}, ErrInvalidRow
	}
	id, err := verification_session_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return domain.UploadVerificationSession{}, err
	}
	if !row.PhotobookID.Valid {
		return domain.UploadVerificationSession{}, ErrInvalidRow
	}
	pid, err := photobook_id.FromUUID(row.PhotobookID.Bytes)
	if err != nil {
		return domain.UploadVerificationSession{}, err
	}
	tokenHash, err := verification_session_token_hash.FromBytes(row.SessionTokenHash)
	if err != nil {
		return domain.UploadVerificationSession{}, err
	}
	allowed, err := intent_count.New(int(row.AllowedIntentCount))
	if err != nil {
		return domain.UploadVerificationSession{}, err
	}
	used, err := intent_count.New(int(row.UsedIntentCount))
	if err != nil {
		return domain.UploadVerificationSession{}, err
	}
	var revokedAt *time.Time
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		revokedAt = &t
	}
	return domain.Restore(domain.RestoreParams{
		ID:                 id,
		PhotobookID:        pid,
		TokenHash:          tokenHash,
		AllowedIntentCount: allowed,
		UsedIntentCount:    used,
		ExpiresAt:          row.ExpiresAt.Time,
		CreatedAt:          row.CreatedAt.Time,
		RevokedAt:          revokedAt,
	})
}
