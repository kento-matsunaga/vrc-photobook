package usecase

import (
	"context"

	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
)

// TouchSession は last_used_at を now() に更新する UseCase。
//
// 編集系 API 成功時のみ呼ぶ（GET / プレビューでは呼ばない、設計書 §ドメイン操作仕様）。
// repository が ErrNotFound を返した場合（session が無効化済 / 不在）は、
// 呼び出し元はリクエスト処理を継続してよい（touch 自体はベストエフォート）。
//
// 本 UseCase はビジネス例外を作らず、repository.Touch のエラーをそのまま返す。
type TouchSession struct {
	repo SessionRepository
}

// NewTouchSession は UseCase を組み立てる。
func NewTouchSession(repo SessionRepository) *TouchSession {
	return &TouchSession{repo: repo}
}

// Execute は session_id に対して last_used_at = now() を立てる。
func (u *TouchSession) Execute(ctx context.Context, id session_id.SessionID) error {
	return u.repo.Touch(ctx, id)
}
