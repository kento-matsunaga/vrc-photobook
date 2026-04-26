package usecase

import (
	"context"
	"errors"
	"time"

	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// ErrDraftConflict は楽観ロック失敗（version 不一致 / status≠draft）に集約するエラー。
var ErrDraftConflict = errors.New("draft conflict (version mismatch or not in draft state)")

// TouchDraftInput は draft 延長の入力。
type TouchDraftInput struct {
	PhotobookID     photobook_id.PhotobookID
	ExpectedVersion int
	Now             time.Time
	DraftTTL        time.Duration
}

// TouchDraft は draft_expires_at を now+ttl に延長する UseCase。
//
// 編集系 API 成功時のみ呼ぶ前提（GET / プレビューでは呼ばない、I-D4）。
type TouchDraft struct {
	repo PhotobookRepository
}

// NewTouchDraft は UseCase を組み立てる。
func NewTouchDraft(repo PhotobookRepository) *TouchDraft {
	return &TouchDraft{repo: repo}
}

// Execute は draft Photobook の draft_expires_at を更新する。
//
// 楽観ロック失敗（version 不一致 / status≠draft）は ErrDraftConflict として返す。
func (u *TouchDraft) Execute(ctx context.Context, in TouchDraftInput) error {
	ttl := in.DraftTTL
	if ttl == 0 {
		ttl = 7 * 24 * time.Hour
	}
	if ttl <= 0 {
		return domain.ErrDraftExpiresInPast
	}
	newExpires := in.Now.Add(ttl)
	err := u.repo.TouchDraft(ctx, in.PhotobookID, newExpires, in.ExpectedVersion)
	if err != nil {
		if errors.Is(err, rdb.ErrOptimisticLockConflict) {
			return ErrDraftConflict
		}
		return err
	}
	return nil
}
