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

// IssueManageSessionInput は manage session 発行の入力。
//
// TokenVersionAtIssue は発行時点での Photobook.manage_url_token_version を渡す。
// reissueManageUrl 時の一括 revoke（I-S10）に使う。
type IssueManageSessionInput struct {
	PhotobookID         photobook_id.PhotobookID
	TokenVersionAtIssue token_version_at_issue.TokenVersionAtIssue
	Now                 time.Time
	ExpiresAt           time.Time
}

// IssueManageSessionOutput は発行結果。RawToken はログ禁止。
type IssueManageSessionOutput struct {
	Session  domain.Session
	RawToken session_token.SessionToken
}

// IssueManageSession は manage session を新規発行する UseCase。
type IssueManageSession struct {
	repo SessionRepository
}

// NewIssueManageSession は UseCase を組み立てる。
func NewIssueManageSession(repo SessionRepository) *IssueManageSession {
	return &IssueManageSession{repo: repo}
}

// Execute は raw SessionToken を生成、SHA-256 hash を Session に保存し、Repository.Create を呼ぶ。
func (u *IssueManageSession) Execute(
	ctx context.Context,
	in IssueManageSessionInput,
) (IssueManageSessionOutput, error) {
	id, err := session_id.New()
	if err != nil {
		return IssueManageSessionOutput{}, fmt.Errorf("session id: %w", err)
	}
	tok, err := session_token.Generate()
	if err != nil {
		return IssueManageSessionOutput{}, fmt.Errorf("token generate: %w", err)
	}
	s, err := domain.NewSession(domain.NewSessionParams{
		ID:                  id,
		TokenHash:           session_token_hash.Of(tok),
		SessionType:         session_type.Manage(),
		PhotobookID:         in.PhotobookID,
		TokenVersionAtIssue: in.TokenVersionAtIssue,
		CreatedAt:           in.Now,
		ExpiresAt:           in.ExpiresAt,
	})
	if err != nil {
		return IssueManageSessionOutput{}, fmt.Errorf("new session: %w", err)
	}
	if err := u.repo.Create(ctx, s); err != nil {
		return IssueManageSessionOutput{}, fmt.Errorf("create session: %w", err)
	}
	return IssueManageSessionOutput{Session: s, RawToken: tok}, nil
}
