package usecase

import (
	"context"
	"fmt"
	"time"

	"vrcpb/backend/internal/auth/session/domain"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
)

// IssueDraftSessionInput は draft session 発行の入力。
//
// expires_at は呼び出し元（後続 PR の Photobook UseCase）が draft_expires_at を渡す。
// 業務知識 v4 §6.4 / 設計書: max 7 日。
type IssueDraftSessionInput struct {
	PhotobookID photobook_id.PhotobookID
	Now         time.Time
	ExpiresAt   time.Time
}

// IssueDraftSessionOutput は発行結果。
//
// RawToken は **Cookie へ書き込むためだけに使う**。ログ出力禁止。
// 呼び出し元は本値を Cookie へ Set-Cookie した後、参照を破棄する想定。
type IssueDraftSessionOutput struct {
	Session  domain.Session
	RawToken session_token.SessionToken
}

// IssueDraftSession は draft session を新規発行する UseCase。
type IssueDraftSession struct {
	repo SessionRepository
}

// NewIssueDraftSession は UseCase を組み立てる。
func NewIssueDraftSession(repo SessionRepository) *IssueDraftSession {
	return &IssueDraftSession{repo: repo}
}

// Execute は raw SessionToken を生成、SHA-256 hash を Session に保存し、Repository.Create を呼ぶ。
func (u *IssueDraftSession) Execute(
	ctx context.Context,
	in IssueDraftSessionInput,
) (IssueDraftSessionOutput, error) {
	id, err := session_id.New()
	if err != nil {
		return IssueDraftSessionOutput{}, fmt.Errorf("session id: %w", err)
	}
	tok, err := session_token.Generate()
	if err != nil {
		return IssueDraftSessionOutput{}, fmt.Errorf("token generate: %w", err)
	}
	s, err := domain.NewSession(domain.NewSessionParams{
		ID:                  id,
		TokenHash:           session_token_hash.Of(tok),
		SessionType:         session_type.Draft(),
		PhotobookID:         in.PhotobookID,
		TokenVersionAtIssue: token_version_at_issue.Zero(),
		CreatedAt:           in.Now,
		ExpiresAt:           in.ExpiresAt,
	})
	if err != nil {
		return IssueDraftSessionOutput{}, fmt.Errorf("new session: %w", err)
	}
	if err := u.repo.Create(ctx, s); err != nil {
		return IssueDraftSessionOutput{}, fmt.Errorf("create session: %w", err)
	}
	return IssueDraftSessionOutput{Session: s, RawToken: tok}, nil
}
