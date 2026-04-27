// Package usecase は Session 認可機構の UseCase を提供する。
//
// 公開する UseCase:
//   - IssueDraftSession / IssueManageSession（発行）
//   - ValidateSession（検証）
//   - TouchSession / RevokeSession（個別更新）
//   - RevokeAllDrafts / RevokeAllManageByTokenVersion（一括失効）
//
// Photobook 側 token（draft_edit_token / manage_url_token）の本物検証 → 上記
// IssueXxxSession 呼び出しは photobook 集約の UseCase（ExchangeDraftTokenForSession /
// ExchangeManageTokenForSession）で行い、HTTP endpoint は internal/http/router.go で配線済。
//
// セキュリティ:
//   - raw SessionToken は戻り値としてのみ扱う（戻したあと、呼び出し元が Cookie へ書き込む）
//   - 本パッケージはログを出さない（観測点は middleware / handler 側に集約）
package usecase

import (
	"context"

	"vrcpb/backend/internal/auth/session/domain"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
)

// SessionRepository は usecase が依存する永続化操作。
//
// 実装は infrastructure/repository/rdb.SessionRepository（実 DB）と、
// テスト用の in-memory fake（usecase/tests/fake_repository.go）を用意する。
type SessionRepository interface {
	Create(ctx context.Context, s domain.Session) error
	FindActiveByHash(
		ctx context.Context,
		hash session_token_hash.SessionTokenHash,
		t session_type.SessionType,
		pid photobook_id.PhotobookID,
	) (domain.Session, error)
	Touch(ctx context.Context, id session_id.SessionID) error
	Revoke(ctx context.Context, id session_id.SessionID) error
	RevokeAllDrafts(ctx context.Context, pid photobook_id.PhotobookID) (int64, error)
	RevokeAllManageByTokenVersion(
		ctx context.Context,
		pid photobook_id.PhotobookID,
		oldVersion int,
	) (int64, error)
}
