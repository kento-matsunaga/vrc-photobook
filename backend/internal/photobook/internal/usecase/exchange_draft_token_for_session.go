package usecase

import (
	"context"
	"errors"
	"time"

	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// ErrInvalidDraftToken は draft token 検証失敗の単一エラー。
//
// 該当なし / 期限切れ / status 不一致を区別せず一律に返す（情報漏洩抑止）。
var ErrInvalidDraftToken = errors.New("invalid draft edit token")

// ExchangeDraftTokenForSessionInput は draft token → session 交換の入力。
type ExchangeDraftTokenForSessionInput struct {
	RawToken draft_edit_token.DraftEditToken
	Now      time.Time
}

// ExchangeDraftTokenForSessionOutput は交換結果。
//
// RawSessionToken は呼び出し元（HTTP handler、PR9c）が response body に乗せる前提。
// **ログ出力禁止**。
type ExchangeDraftTokenForSessionOutput struct {
	RawSessionToken session_token.SessionToken
	PhotobookID     photobook_id.PhotobookID
	ExpiresAt       time.Time
}

// ExchangeDraftTokenForSession は raw draft_edit_token を検証し、draft session を発行する UseCase。
//
// 入場フロー（GET 相当）のため touchDraft は **呼ばない**（I-D4: 編集系 API 成功時のみ延長）。
// 設計参照: docs/plan/m2-photobook-session-integration-plan.md §9.5
type ExchangeDraftTokenForSession struct {
	repo   PhotobookRepository
	issuer DraftSessionIssuer
}

// NewExchangeDraftTokenForSession は UseCase を組み立てる。
func NewExchangeDraftTokenForSession(repo PhotobookRepository, issuer DraftSessionIssuer) *ExchangeDraftTokenForSession {
	return &ExchangeDraftTokenForSession{repo: repo, issuer: issuer}
}

// Execute は token 検証 → 有効な draft Photobook 取得 → draft session 発行を行う。
func (u *ExchangeDraftTokenForSession) Execute(
	ctx context.Context,
	in ExchangeDraftTokenForSessionInput,
) (ExchangeDraftTokenForSessionOutput, error) {
	if in.RawToken.IsZero() {
		return ExchangeDraftTokenForSessionOutput{}, ErrInvalidDraftToken
	}
	hash := draft_edit_token_hash.Of(in.RawToken)
	pb, err := u.repo.FindByDraftEditTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, rdb.ErrNotFound) {
			return ExchangeDraftTokenForSessionOutput{}, ErrInvalidDraftToken
		}
		return ExchangeDraftTokenForSessionOutput{}, err
	}
	// repository 側 SQL が status='draft' AND draft_expires_at>now() を要求するため、
	// ここまで来た時点で draft 状態 + 未期限切れが保証されている。二重防壁として確認:
	if !pb.IsDraft() || pb.DraftExpiresAt() == nil {
		return ExchangeDraftTokenForSessionOutput{}, ErrInvalidDraftToken
	}
	expiresAt := *pb.DraftExpiresAt()
	rawSession, err := u.issuer.IssueDraft(ctx, pb.ID(), in.Now, expiresAt)
	if err != nil {
		return ExchangeDraftTokenForSessionOutput{}, err
	}
	return ExchangeDraftTokenForSessionOutput{
		RawSessionToken: rawSession,
		PhotobookID:     pb.ID(),
		ExpiresAt:       expiresAt,
	}, nil
}
