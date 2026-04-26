// Package sessionintegration は Session 認可機構の操作を **session サブツリー外** から
// 安全に呼び出せるようにする薄い facade。
//
// 配置の理由（Go internal ルール）:
//   - usecase / repository は internal/auth/session/internal/usecase / .../repository 配下にあり、
//     internal/auth/session のサブツリーからしか import できない
//   - Photobook 集約から Session を呼ぶには、session 配下に **公開窓口** が必要
//   - sessionintegration は session サブツリー内なので usecase / repository を import 可能
//   - photobook 側は本パッケージだけを import すればよく、internal の中身を直接触れずに済む
//
// 提供する操作（PR8 既存 UseCase の薄い wrapper）:
//   - IssueDraftWithTx        : draft session 発行（Tx 起点）
//   - IssueManageWithTx       : manage session 発行（Tx 起点）
//   - RevokeAllDraftsWithTx   : Photobook publish 時の draft session 一括 revoke
//   - RevokeAllManageByTokenVersionWithTx : reissueManageUrl 時の旧 manage session 一括 revoke
//
// セキュリティ:
//   - raw SessionToken は戻り値としてのみ返す（呼び出し元が Cookie 発行に使うため）
//   - 本パッケージはログを出さない
package sessionintegration

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	authsessrepo "vrcpb/backend/internal/auth/session/infrastructure/repository/rdb"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
	authuc "vrcpb/backend/internal/auth/session/internal/usecase"
)

// IssueDraftWithTx は tx 起点で draft session を発行する。
//
// 引数の photobookID は外部の UUID（Photobook 集約 ID）。本関数内で session 機構の
// 仮 PhotobookID VO に変換する（PR9 段階で type が分かれているため）。
func IssueDraftWithTx(
	ctx context.Context,
	tx pgx.Tx,
	photobookID uuid.UUID,
	now time.Time,
	expiresAt time.Time,
) (session_token.SessionToken, session_id.SessionID, error) {
	pid, err := photobook_id.FromUUID(photobookID)
	if err != nil {
		return session_token.SessionToken{}, session_id.SessionID{}, err
	}
	repo := authsessrepo.NewSessionRepository(tx)
	uc := authuc.NewIssueDraftSession(repo)
	out, err := uc.Execute(ctx, authuc.IssueDraftSessionInput{
		PhotobookID: pid,
		Now:         now,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return session_token.SessionToken{}, session_id.SessionID{}, err
	}
	return out.RawToken, out.Session.ID(), nil
}

// IssueManageWithTx は tx 起点で manage session を発行する。
func IssueManageWithTx(
	ctx context.Context,
	tx pgx.Tx,
	photobookID uuid.UUID,
	tokenVersion int,
	now time.Time,
	expiresAt time.Time,
) (session_token.SessionToken, session_id.SessionID, error) {
	pid, err := photobook_id.FromUUID(photobookID)
	if err != nil {
		return session_token.SessionToken{}, session_id.SessionID{}, err
	}
	tv, err := token_version_at_issue.New(tokenVersion)
	if err != nil {
		return session_token.SessionToken{}, session_id.SessionID{}, err
	}
	repo := authsessrepo.NewSessionRepository(tx)
	uc := authuc.NewIssueManageSession(repo)
	out, err := uc.Execute(ctx, authuc.IssueManageSessionInput{
		PhotobookID:         pid,
		TokenVersionAtIssue: tv,
		Now:                 now,
		ExpiresAt:           expiresAt,
	})
	if err != nil {
		return session_token.SessionToken{}, session_id.SessionID{}, err
	}
	return out.RawToken, out.Session.ID(), nil
}

// RevokeAllDraftsWithTx は tx 起点で photobook 配下の draft session を一括 revoke する。
//
// Photobook.publishFromDraft の同一 TX 内で呼ぶ前提（I-D7 / I-S9）。
// 影響行数を返す（0 行でもエラーにしない、冪等性のため）。
func RevokeAllDraftsWithTx(
	ctx context.Context,
	tx pgx.Tx,
	photobookID uuid.UUID,
) (int64, error) {
	pid, err := photobook_id.FromUUID(photobookID)
	if err != nil {
		return 0, err
	}
	repo := authsessrepo.NewSessionRepository(tx)
	uc := authuc.NewRevokeAllDrafts(repo)
	return uc.Execute(ctx, pid)
}

// RevokeAllManageByTokenVersionWithTx は tx 起点で旧 version 以下の manage session を
// 一括 revoke する。
//
// reissueManageUrl の同一 TX 内で呼ぶ前提（I-S10）。
func RevokeAllManageByTokenVersionWithTx(
	ctx context.Context,
	tx pgx.Tx,
	photobookID uuid.UUID,
	oldVersion int,
) (int64, error) {
	pid, err := photobook_id.FromUUID(photobookID)
	if err != nil {
		return 0, err
	}
	repo := authsessrepo.NewSessionRepository(tx)
	uc := authuc.NewRevokeAllManageByTokenVersion(repo)
	return uc.Execute(ctx, pid, oldVersion)
}
