// Package session_adapter は Photobook UseCase の session ports interface を、
// session 機構の sessionintegration 経由で実装する薄い adapter。
//
// 配置の理由:
//   - Photobook UseCase は session の internal/usecase を直接 import できない
//     （Go internal ルール）
//   - sessionintegration は session 配下の facade で、internal/usecase を呼べる
//   - 本 adapter は photobook 側に居住し、sessionintegration を呼ぶ「橋渡し」だけを行う
//   - 依存方向: photobook/usecase → photobook/infrastructure/session_adapter →
//                 auth/session/sessionintegration → auth/session/internal/usecase
//
// セキュリティ:
//   - raw SessionToken は本 adapter を通って戻る（呼び出し元の Photobook UseCase が response に乗せる）
//   - 本 adapter はログを出さない
package session_adapter

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/sessionintegration"
	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// === Issuer ===

// DraftIssuer は usecase.DraftSessionIssuer を実装する。
//
// Exchange*ForSession から呼ばれる。pool を保持し、内部で WithTx で session INSERT する。
type DraftIssuer struct {
	pool *pgxpool.Pool
}

// NewDraftIssuer は DraftIssuer を作る。
func NewDraftIssuer(pool *pgxpool.Pool) *DraftIssuer {
	return &DraftIssuer{pool: pool}
}

// IssueDraft は draft session を 1 トランザクションで発行する。
func (a *DraftIssuer) IssueDraft(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	now time.Time,
	expiresAt time.Time,
) (session_token.SessionToken, error) {
	var raw session_token.SessionToken
	err := database.WithTx(ctx, a.pool, func(tx pgx.Tx) error {
		rawTok, _, err := sessionintegration.IssueDraftWithTx(ctx, tx, photobookID.UUID(), now, expiresAt)
		if err != nil {
			return err
		}
		raw = rawTok
		return nil
	})
	if err != nil {
		return session_token.SessionToken{}, err
	}
	return raw, nil
}

// ManageIssuer は usecase.ManageSessionIssuer を実装する。
type ManageIssuer struct {
	pool *pgxpool.Pool
}

// NewManageIssuer は ManageIssuer を作る。
func NewManageIssuer(pool *pgxpool.Pool) *ManageIssuer {
	return &ManageIssuer{pool: pool}
}

// IssueManage は manage session を 1 トランザクションで発行する。
func (a *ManageIssuer) IssueManage(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	tokenVersion int,
	now time.Time,
	expiresAt time.Time,
) (session_token.SessionToken, error) {
	var raw session_token.SessionToken
	err := database.WithTx(ctx, a.pool, func(tx pgx.Tx) error {
		rawTok, _, err := sessionintegration.IssueManageWithTx(ctx, tx, photobookID.UUID(), tokenVersion, now, expiresAt)
		if err != nil {
			return err
		}
		raw = rawTok
		return nil
	})
	if err != nil {
		return session_token.SessionToken{}, err
	}
	return raw, nil
}

// === Revoker (TX 内で使う) ===

// txDraftRevoker は usecase.DraftSessionRevoker を tx 起点で実装する。
type txDraftRevoker struct {
	tx pgx.Tx
}

// RevokeAllDrafts は同一 tx 内で draft session を一括 revoke する。
func (r *txDraftRevoker) RevokeAllDrafts(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
) (int64, error) {
	return sessionintegration.RevokeAllDraftsWithTx(ctx, r.tx, photobookID.UUID())
}

// NewDraftRevokerFactory は usecase.DraftSessionRevokerFactory に渡す関数を返す。
func NewDraftRevokerFactory() usecase.DraftSessionRevokerFactory {
	return func(tx pgx.Tx) usecase.DraftSessionRevoker {
		return &txDraftRevoker{tx: tx}
	}
}

// txManageRevoker は usecase.ManageSessionRevoker を tx 起点で実装する。
type txManageRevoker struct {
	tx pgx.Tx
}

// RevokeAllManageByTokenVersion は同一 tx 内で旧 version 以下の manage session を一括 revoke する。
func (r *txManageRevoker) RevokeAllManageByTokenVersion(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	oldVersion int,
) (int64, error) {
	return sessionintegration.RevokeAllManageByTokenVersionWithTx(ctx, r.tx, photobookID.UUID(), oldVersion)
}

// NewManageRevokerFactory は usecase.ManageSessionRevokerFactory に渡す関数を返す。
func NewManageRevokerFactory() usecase.ManageSessionRevokerFactory {
	return func(tx pgx.Tx) usecase.ManageSessionRevoker {
		return &txManageRevoker{tx: tx}
	}
}

// === Single revoke (TX 不要) ===

// CurrentRevoker は usecase.CurrentSessionRevoker を実装する。
//
// M-1a: /api/manage/photobooks/{id}/session-revoke から、現在 Cookie session を
// 1 件 revoke するために使う。pool 起点で TX を張らず単一 SQL UPDATE する。
type CurrentRevoker struct {
	pool *pgxpool.Pool
}

// NewCurrentRevoker は CurrentRevoker を作る。
func NewCurrentRevoker(pool *pgxpool.Pool) *CurrentRevoker {
	return &CurrentRevoker{pool: pool}
}

// 静的型チェック: ports.CurrentSessionRevoker を満たすことを compile-time で保証。
var _ usecase.CurrentSessionRevoker = (*CurrentRevoker)(nil)

// RevokeOne は session_id 一致の単一 session を revoke する。
func (a *CurrentRevoker) RevokeOne(ctx context.Context, sessionID uuid.UUID) error {
	return sessionintegration.RevokeOneSession(ctx, a.pool, sessionID)
}
