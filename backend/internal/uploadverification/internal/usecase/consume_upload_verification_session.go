package usecase

import (
	"context"
	"errors"
	"time"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_id"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token_hash"
	uploadrdb "vrcpb/backend/internal/uploadverification/infrastructure/repository/rdb"
)

// ConsumeRepository は Consume UseCase が依存する Repository 操作。
type ConsumeRepository interface {
	ConsumeOne(
		ctx context.Context,
		tokenHash verification_session_token_hash.VerificationSessionTokenHash,
		pid photobook_id.PhotobookID,
		now time.Time,
	) (uploadrdb.ConsumeOutput, error)
}

// ConsumeInput は Consume の入力。
//
// RawToken は Authorization: Bearer header から取り出した raw token を VO 化した値。
// PhotobookID は draft session middleware で context から取得した値（呼び出し側責務）。
// Now は期限境界判定用（test の Clock 固定 / 監査時刻整合のため Application 層から渡す）。
type ConsumeInput struct {
	RawToken    verification_session_token.VerificationSessionToken
	PhotobookID photobook_id.PhotobookID
	Now         time.Time
}

// ConsumeOutput は consume 結果。Frontend に戻す情報は最小限。
type ConsumeOutput struct {
	SessionID          verification_session_id.VerificationSessionID
	UsedIntentCount    int
	AllowedIntentCount int
	Remaining          int // = Allowed - Used（UI 表示用）
}

// ConsumeUploadVerificationSession は atomic UPDATE で 1 回 consume する UseCase。
type ConsumeUploadVerificationSession struct {
	repo ConsumeRepository
}

// NewConsumeUploadVerificationSession は UseCase を組み立てる。
func NewConsumeUploadVerificationSession(repo ConsumeRepository) *ConsumeUploadVerificationSession {
	return &ConsumeUploadVerificationSession{repo: repo}
}

// Execute は consume を実行する。失敗時は ErrUploadVerificationFailed を返す
// （理由を外部に区別して出さない、bot 学習防止）。
//
// in.Now が zero の場合はサーバ time.Now() を使う（後方互換）。期限境界の test 用途
// では明示的に渡すこと。
func (u *ConsumeUploadVerificationSession) Execute(ctx context.Context, in ConsumeInput) (ConsumeOutput, error) {
	hash := verification_session_token_hash.Of(in.RawToken)
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	out, err := u.repo.ConsumeOne(ctx, hash, in.PhotobookID, now)
	if err != nil {
		if errors.Is(err, uploadrdb.ErrUploadVerificationFailed) {
			return ConsumeOutput{}, ErrUploadVerificationFailed
		}
		return ConsumeOutput{}, err
	}
	return ConsumeOutput{
		SessionID:          out.ID,
		UsedIntentCount:    out.UsedIntentCount,
		AllowedIntentCount: out.AllowedIntentCount,
		Remaining:          out.AllowedIntentCount - out.UsedIntentCount,
	}, nil
}
