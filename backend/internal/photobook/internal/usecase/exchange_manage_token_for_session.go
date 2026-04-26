package usecase

import (
	"context"
	"errors"
	"time"

	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// ErrInvalidManageToken は manage token 検証失敗の単一エラー。
var ErrInvalidManageToken = errors.New("invalid manage url token")

// ExchangeManageTokenForSessionInput は manage token → session 交換の入力。
type ExchangeManageTokenForSessionInput struct {
	RawToken          manage_url_token.ManageUrlToken
	Now               time.Time
	ManageSessionTTL  time.Duration
}

// ExchangeManageTokenForSessionOutput は交換結果。
type ExchangeManageTokenForSessionOutput struct {
	RawSessionToken     session_token.SessionToken
	PhotobookID         photobook_id.PhotobookID
	TokenVersionAtIssue int
	ExpiresAt           time.Time
}

// ExchangeManageTokenForSession は raw manage_url_token を検証し、manage session を発行する UseCase。
//
// status IN ('published','deleted') の Photobook で動く。
// 設計参照: docs/plan/m2-photobook-session-integration-plan.md §9.6
type ExchangeManageTokenForSession struct {
	repo   PhotobookRepository
	issuer ManageSessionIssuer
}

// NewExchangeManageTokenForSession は UseCase を組み立てる。
func NewExchangeManageTokenForSession(repo PhotobookRepository, issuer ManageSessionIssuer) *ExchangeManageTokenForSession {
	return &ExchangeManageTokenForSession{repo: repo, issuer: issuer}
}

// Execute は token 検証 → manage session 発行を行う。
//
// TokenVersionAtIssue は発行時点の Photobook.manage_url_token_version の snapshot。
// 後の reissueManageUrl で oldVersion 以下を一括 revoke するため必須。
func (u *ExchangeManageTokenForSession) Execute(
	ctx context.Context,
	in ExchangeManageTokenForSessionInput,
) (ExchangeManageTokenForSessionOutput, error) {
	if in.RawToken.IsZero() {
		return ExchangeManageTokenForSessionOutput{}, ErrInvalidManageToken
	}
	if in.ManageSessionTTL <= 0 {
		return ExchangeManageTokenForSessionOutput{}, errors.New("manage session ttl must be positive")
	}
	hash := manage_url_token_hash.Of(in.RawToken)
	pb, err := u.repo.FindByManageUrlTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, rdb.ErrNotFound) {
			return ExchangeManageTokenForSessionOutput{}, ErrInvalidManageToken
		}
		return ExchangeManageTokenForSessionOutput{}, err
	}
	version := pb.ManageUrlTokenVersion().Int()
	expiresAt := in.Now.Add(in.ManageSessionTTL)
	rawSession, err := u.issuer.IssueManage(ctx, pb.ID(), version, in.Now, expiresAt)
	if err != nil {
		return ExchangeManageTokenForSessionOutput{}, err
	}
	return ExchangeManageTokenForSessionOutput{
		RawSessionToken:     rawSession,
		PhotobookID:         pb.ID(),
		TokenVersionAtIssue: version,
		ExpiresAt:           expiresAt,
	}, nil
}
