package usecase

import (
	"context"
	"errors"

	"vrcpb/backend/internal/auth/session/domain"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
)

// ErrSessionInvalid は session が無効（不一致 / 期限切れ / revoke 済 / 不明 token）のときに返す。
//
// 認可レイヤーの主要エラー。クライアントには 401 unauthorized で返す前提で、
// 本エラーから内部詳細を分けず単一のセンチネルにする（情報漏洩抑止）。
var ErrSessionInvalid = errors.New("session invalid")

// ValidateSessionInput は検証の入力。
//
// RawToken は Cookie から取り出した値。**ログには出さない**。
type ValidateSessionInput struct {
	RawToken    session_token.SessionToken
	PhotobookID photobook_id.PhotobookID
	SessionType session_type.SessionType
}

// ValidateSessionOutput は検証成功時に返される Session。
type ValidateSessionOutput struct {
	Session domain.Session
}

// ValidateSession は raw token を hash 化し、Repository.FindActiveByHash で
// session_type / photobook_id / revoked_at IS NULL / expires_at > now() を確認する。
//
// repository の query 自体が以下をすべて WHERE に含めるため、
// 本 UseCase は「該当なし」を一律 ErrSessionInvalid として返す。
//
//   - session_token_hash 不一致
//   - session_type 不一致
//   - photobook_id 不一致
//   - revoked_at IS NOT NULL
//   - expires_at <= now()
type ValidateSession struct {
	repo SessionRepository
}

// NewValidateSession は UseCase を組み立てる。
func NewValidateSession(repo SessionRepository) *ValidateSession {
	return &ValidateSession{repo: repo}
}

// Execute は session を検証して、有効ならドメイン Session を返す。
func (u *ValidateSession) Execute(
	ctx context.Context,
	in ValidateSessionInput,
) (ValidateSessionOutput, error) {
	if in.RawToken.IsZero() {
		return ValidateSessionOutput{}, ErrSessionInvalid
	}
	hash := session_token_hash.Of(in.RawToken)
	s, err := u.repo.FindActiveByHash(ctx, hash, in.SessionType, in.PhotobookID)
	if err != nil {
		// repository.ErrNotFound でも、その他の DB エラーでも、
		// クライアントから見て区別する必要がないので一律 ErrSessionInvalid を返す。
		// ただし内部ログ用に元のエラーを wrap して呼び出し元 middleware で記録できるようにする。
		return ValidateSessionOutput{}, errSessionInvalidWithCause(err)
	}
	return ValidateSessionOutput{Session: s}, nil
}

// errSessionInvalidWithCause は ErrSessionInvalid を errors.Is で判定可能にしつつ、
// cause を error chain として保持する（内部ログ用）。
type sessionInvalidErr struct{ cause error }

func (e *sessionInvalidErr) Error() string { return ErrSessionInvalid.Error() }
func (e *sessionInvalidErr) Unwrap() error { return e.cause }
func (e *sessionInvalidErr) Is(target error) bool {
	return target == ErrSessionInvalid
}

func errSessionInvalidWithCause(cause error) error {
	return &sessionInvalidErr{cause: cause}
}
