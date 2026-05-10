// manage_actions.go: M-1a Manage safety baseline 用 UseCase 群。
//
// 設計参照:
//   - docs/plan/m-1-manage-mvp-safety-plan.md §3
//   - 業務知識 v4 §3.4 / §6.13
//
// 含む UseCase:
//   - UpdatePhotobookVisibilityFromManage  (visibility 変更、unlisted/private のみ)
//   - UpdatePhotobookSensitiveFromManage   (sensitive flag 切替)
//   - IssueDraftSessionFromManage          (manage→draft session 昇格、draft photobook のみ)
//   - RevokeCurrentManageSession           (現在 Cookie session 1 件 revoke)
//
// セキュリティ:
//   - raw SessionToken は IssueDraftSessionFromManageOutput のみで返す（呼び出し元 handler が
//     Frontend Route Handler 経由で Cookie 化、本 UseCase / package はログを出さない）
//   - manage_url_token / draft_edit_token は触らない
//   - 失敗詳細は Err* に集約、handler は固定文言で返す
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// ErrManagePublicChangeNotAllowed は visibility=public が指定されたときのエラー。
//
// /manage 経路では public 化を許可しない（業務知識 v4 §3.2 / m-1a plan §3.2.2）。
// public 化は publish flow 側でのみ許容する。handler 層で reject されるべきだが、
// 二重防壁として UseCase 層でも検出する。
var ErrManagePublicChangeNotAllowed = errors.New("public visibility change is not allowed from manage")

// ErrManageNotPublished は status≠published で /manage の mutation を呼んだときのエラー。
//
// Repository が status='published' を WHERE 条件にしているため、Repository 段階で
// ErrOptimisticLockConflict として返るが、handler の reason 分離のために本 UseCase 層で
// 区別したい場合の予約エラー（現状未使用、将来 reason="not_published" を出す場合に使う）。
var ErrManageNotPublished = errors.New("photobook is not in published state")

// ErrManageNotDraftForResume は IssueDraftSessionFromManage で photobook が draft で
// ない場合に返すエラー。
//
// MVP では publish 後の draft 復帰は提供しない（unpublish は M-1b 範囲）。本エラーは
// 「現在 MVP では公開済みフォトブックの再編集はできません」UI 文言にマップされる想定。
var ErrManageNotDraftForResume = errors.New("photobook is not in draft state, edit resume not allowed")

// =============================================================================
// UpdatePhotobookVisibilityFromManage
// =============================================================================

// UpdatePhotobookVisibilityFromManageInput は visibility 変更の入力。
//
// Visibility は呼び出し元（handler）で `unlisted` / `private` に絞られている前提。
// `public` が来た場合は ErrManagePublicChangeNotAllowed を返す（handler でも reject、二重防壁）。
type UpdatePhotobookVisibilityFromManageInput struct {
	PhotobookID     photobook_id.PhotobookID
	Visibility      visibility.Visibility
	ExpectedVersion int
	Now             time.Time
}

// UpdatePhotobookVisibilityFromManage は published photobook の visibility を更新する UseCase。
type UpdatePhotobookVisibilityFromManage struct{ pool *pgxpool.Pool }

// NewUpdatePhotobookVisibilityFromManage は UseCase を組み立てる。
func NewUpdatePhotobookVisibilityFromManage(pool *pgxpool.Pool) *UpdatePhotobookVisibilityFromManage {
	return &UpdatePhotobookVisibilityFromManage{pool: pool}
}

// Execute は visibility=$visibility / version+1 / updated_at=$now を SQL で 1 行 UPDATE する。
//
// status='published' AND version=$expected で WHERE。0 行影響は ErrOptimisticLockConflict。
// public が指定された場合は ErrManagePublicChangeNotAllowed を即返し、SQL に到達させない。
func (u *UpdatePhotobookVisibilityFromManage) Execute(
	ctx context.Context,
	in UpdatePhotobookVisibilityFromManageInput,
) error {
	if in.Visibility.Equal(visibility.Public()) {
		return ErrManagePublicChangeNotAllowed
	}
	repo := photobookrdb.NewPhotobookRepository(u.pool)
	return repo.UpdateVisibilityFromManage(ctx, in.PhotobookID, in.Visibility.String(), in.ExpectedVersion, in.Now)
}

// =============================================================================
// UpdatePhotobookSensitiveFromManage
// =============================================================================

// UpdatePhotobookSensitiveFromManageInput は sensitive 切替の入力。
type UpdatePhotobookSensitiveFromManageInput struct {
	PhotobookID     photobook_id.PhotobookID
	Sensitive       bool
	ExpectedVersion int
	Now             time.Time
}

// UpdatePhotobookSensitiveFromManage は published photobook の sensitive を切替える UseCase。
type UpdatePhotobookSensitiveFromManage struct{ pool *pgxpool.Pool }

// NewUpdatePhotobookSensitiveFromManage は UseCase を組み立てる。
func NewUpdatePhotobookSensitiveFromManage(pool *pgxpool.Pool) *UpdatePhotobookSensitiveFromManage {
	return &UpdatePhotobookSensitiveFromManage{pool: pool}
}

// Execute は sensitive=$sensitive / version+1 / updated_at=$now を SQL で 1 行 UPDATE する。
//
// status='published' AND version=$expected で WHERE。0 行影響は ErrOptimisticLockConflict。
func (u *UpdatePhotobookSensitiveFromManage) Execute(
	ctx context.Context,
	in UpdatePhotobookSensitiveFromManageInput,
) error {
	repo := photobookrdb.NewPhotobookRepository(u.pool)
	return repo.UpdateSensitiveFromManage(ctx, in.PhotobookID, in.Sensitive, in.ExpectedVersion, in.Now)
}

// =============================================================================
// IssueDraftSessionFromManage
// =============================================================================

// IssueDraftSessionFromManageInput は manage→draft session 昇格の入力。
type IssueDraftSessionFromManageInput struct {
	PhotobookID photobook_id.PhotobookID
	Now         time.Time
}

// IssueDraftSessionFromManageOutput は昇格結果。
//
// RawSessionToken は呼び出し元（handler）が response body に乗せ、Frontend Route Handler が
// Cookie 化する前提。**ログ出力禁止**。
type IssueDraftSessionFromManageOutput struct {
	RawSessionToken session_token.SessionToken
	ExpiresAt       time.Time
}

// IssueDraftSessionFromManage は manage Cookie session 認可下で draft session を発行する UseCase。
//
// 業務知識 v4 §3.4 「フォトブック内容編集の導線を提供する」を実現する forward-compatible
// plumbing。MVP では status='draft' photobook のみ受け付け、published / deleted は
// ErrManageNotDraftForResume を返す（unpublish は M-1b 範囲）。
type IssueDraftSessionFromManage struct {
	pool   *pgxpool.Pool
	issuer DraftSessionIssuer
}

// NewIssueDraftSessionFromManage は UseCase を組み立てる。
func NewIssueDraftSessionFromManage(pool *pgxpool.Pool, issuer DraftSessionIssuer) *IssueDraftSessionFromManage {
	return &IssueDraftSessionFromManage{pool: pool, issuer: issuer}
}

// Execute は photobook 状態を確認し、draft なら session 発行、それ以外は ErrManageNotDraftForResume。
//
// ExpiresAt は photobook.draft_expires_at（draft 状態では NOT NULL）を採用する。
// 認可は middleware で完了済の前提（manage session が photobook と一致していることを
// middleware が担保）、本 UseCase は再認証しない。
func (u *IssueDraftSessionFromManage) Execute(
	ctx context.Context,
	in IssueDraftSessionFromManageInput,
) (IssueDraftSessionFromManageOutput, error) {
	repo := photobookrdb.NewPhotobookRepository(u.pool)
	pb, err := repo.FindByID(ctx, in.PhotobookID)
	if err != nil {
		if errors.Is(err, photobookrdb.ErrNotFound) {
			return IssueDraftSessionFromManageOutput{}, ErrManageNotFound
		}
		return IssueDraftSessionFromManageOutput{}, err
	}
	if !pb.Status().IsDraft() {
		return IssueDraftSessionFromManageOutput{}, ErrManageNotDraftForResume
	}
	expires := pb.DraftExpiresAt()
	if expires == nil {
		// draft 状態なら CHECK 制約上 NOT NULL のはずだが、二重防壁。
		return IssueDraftSessionFromManageOutput{}, ErrManageNotDraftForResume
	}
	rawSession, err := u.issuer.IssueDraft(ctx, pb.ID(), in.Now, *expires)
	if err != nil {
		return IssueDraftSessionFromManageOutput{}, err
	}
	return IssueDraftSessionFromManageOutput{
		RawSessionToken: rawSession,
		ExpiresAt:       *expires,
	}, nil
}

// =============================================================================
// RevokeCurrentManageSession
// =============================================================================

// RevokeCurrentManageSessionInput は現在 session の明示破棄入力。
//
// SessionID は middleware が context にセットした domain.Session.ID() を呼び出し元
// （handler）が UUID に変換して渡す。
type RevokeCurrentManageSessionInput struct {
	SessionID uuid.UUID
}

// RevokeCurrentManageSession は現在 manage session を 1 件 revoke する UseCase。
//
// 元の manage_url_token は失効させない（別端末からの再入場を妨げない、設計書 §3.3）。
// Cookie の削除（Max-Age=-1 / 空 Value）は呼び出し元 handler / Workers Route Handler の責務。
type RevokeCurrentManageSession struct {
	revoker CurrentSessionRevoker
}

// NewRevokeCurrentManageSession は UseCase を組み立てる。
func NewRevokeCurrentManageSession(revoker CurrentSessionRevoker) *RevokeCurrentManageSession {
	return &RevokeCurrentManageSession{revoker: revoker}
}

// Execute は session_id を revoke する。冪等（既 revoked / 不存在でも error にしない場合あり、
// repository 実装に従う）。
func (u *RevokeCurrentManageSession) Execute(ctx context.Context, in RevokeCurrentManageSessionInput) error {
	return u.revoker.RevokeOne(ctx, in.SessionID)
}
