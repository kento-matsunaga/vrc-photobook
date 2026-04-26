package usecase

import (
	"context"

	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
)

var _ session_id.SessionID // type 参照保持（package が使われていないと判定されないため）

// RevokeSession は単一 session を明示破棄する UseCase。
//
// 明示破棄では元の draft_edit_token / manage_url_token は失効させない（別端末からの再入場を妨げない、
// 設計書 §3.3）。Cookie の削除は呼び出し元の handler 側で cookie.BuildClear を使う。
type RevokeSession struct {
	repo SessionRepository
}

// NewRevokeSession は UseCase を組み立てる。
func NewRevokeSession(repo SessionRepository) *RevokeSession {
	return &RevokeSession{repo: repo}
}

// Execute は session_id に対して revoked_at = now() を立てる。
func (u *RevokeSession) Execute(ctx context.Context, id session_id.SessionID) error {
	return u.repo.Revoke(ctx, id)
}

// RevokeAllDrafts は publishFromDraft 時に draft session を一括 revoke する UseCase。
//
// 呼び出し元は Photobook.publishFromDraft の同一 TX 内で呼ぶ（PR9 で接続）。
// 影響行数は 0 でもエラーにせず、冪等性を保つ。
type RevokeAllDrafts struct {
	repo SessionRepository
}

// NewRevokeAllDrafts は UseCase を組み立てる。
func NewRevokeAllDrafts(repo SessionRepository) *RevokeAllDrafts {
	return &RevokeAllDrafts{repo: repo}
}

// Execute は photobook_id 配下の全 draft session を revoke する。
func (u *RevokeAllDrafts) Execute(ctx context.Context, pid photobook_id.PhotobookID) (int64, error) {
	return u.repo.RevokeAllDrafts(ctx, pid)
}

// RevokeAllManageByTokenVersion は manage URL 再発行時に旧 version 以下の manage session を
// 一括 revoke する UseCase。
//
// 呼び出し元は Photobook.reissueManageUrl の同一 TX 内で呼ぶ（PR9 で接続）。
// oldVersion は再発行前の Photobook.manage_url_token_version（再発行直前の値）。
type RevokeAllManageByTokenVersion struct {
	repo SessionRepository
}

// NewRevokeAllManageByTokenVersion は UseCase を組み立てる。
func NewRevokeAllManageByTokenVersion(repo SessionRepository) *RevokeAllManageByTokenVersion {
	return &RevokeAllManageByTokenVersion{repo: repo}
}

// Execute は photobook_id 配下の manage session のうち、token_version_at_issue <= oldVersion を revoke。
func (u *RevokeAllManageByTokenVersion) Execute(
	ctx context.Context,
	pid photobook_id.PhotobookID,
	oldVersion int,
) (int64, error) {
	return u.repo.RevokeAllManageByTokenVersion(ctx, pid, oldVersion)
}

