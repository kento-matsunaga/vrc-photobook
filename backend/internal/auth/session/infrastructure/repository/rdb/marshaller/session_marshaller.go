// Package marshaller はドメインの Session と sqlc 生成物の Session row 構造の相互変換を担う。
//
// VO ↔ プリミティブ変換は本パッケージに閉じる（.agents/rules/domain-standard.md §インフラ）。
package marshaller

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/auth/session/domain"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
	"vrcpb/backend/internal/auth/session/infrastructure/repository/rdb/sqlcgen"
)

// ErrInvalidRow は DB から取り出した行が想定外（Valid=false の必須カラム等）のときのエラー。
var ErrInvalidRow = errors.New("invalid session row from db")

// ToCreateParams はドメインの Session を sqlc CreateSessionParams に変換する。
func ToCreateParams(s domain.Session) (sqlcgen.CreateSessionParams, error) {
	return sqlcgen.CreateSessionParams{
		ID:                  pgtype.UUID{Bytes: s.ID().UUID(), Valid: true},
		SessionTokenHash:    s.TokenHash().Bytes(),
		SessionType:         s.SessionType().String(),
		PhotobookID:         pgtype.UUID{Bytes: s.PhotobookID().UUID(), Valid: true},
		TokenVersionAtIssue: int32(s.TokenVersionAtIssue().Int()),
		ExpiresAt:           pgtype.Timestamptz{Time: s.ExpiresAt(), Valid: true},
		CreatedAt:           pgtype.Timestamptz{Time: s.CreatedAt(), Valid: true},
	}, nil
}

// FromRow は sqlc が返す Session row をドメインの Session に復元する。
func FromRow(row sqlcgen.Session) (domain.Session, error) {
	if !row.ID.Valid {
		return domain.Session{}, ErrInvalidRow
	}
	if !row.PhotobookID.Valid {
		return domain.Session{}, ErrInvalidRow
	}
	if !row.ExpiresAt.Valid || !row.CreatedAt.Valid {
		return domain.Session{}, ErrInvalidRow
	}

	id, err := session_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return domain.Session{}, err
	}
	hash, err := session_token_hash.FromBytes(row.SessionTokenHash)
	if err != nil {
		return domain.Session{}, err
	}
	st, err := session_type.Parse(row.SessionType)
	if err != nil {
		return domain.Session{}, err
	}
	pid, err := photobook_id.FromUUID(row.PhotobookID.Bytes)
	if err != nil {
		return domain.Session{}, err
	}
	ver, err := token_version_at_issue.New(int(row.TokenVersionAtIssue))
	if err != nil {
		return domain.Session{}, err
	}

	var lastUsed *time.Time
	if row.LastUsedAt.Valid {
		t := row.LastUsedAt.Time
		lastUsed = &t
	}
	var revoked *time.Time
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		revoked = &t
	}

	return domain.RestoreSession(domain.RestoreSessionParams{
		ID:                  id,
		TokenHash:           hash,
		SessionType:         st,
		PhotobookID:         pid,
		TokenVersionAtIssue: ver,
		ExpiresAt:           row.ExpiresAt.Time,
		CreatedAt:           row.CreatedAt.Time,
		LastUsedAt:          lastUsed,
		RevokedAt:           revoked,
	})
}
