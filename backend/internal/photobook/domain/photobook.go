// Package domain は Photobook 集約のドメインモデルを提供する。
//
// 設計参照:
//   - docs/design/aggregates/photobook/ドメイン設計.md
//   - docs/design/aggregates/photobook/データモデル設計.md
//   - docs/plan/m2-photobook-session-integration-plan.md
//
// 扱う状態: draft / published / deleted（softDelete / restore / purge / hide /
// unhide は MVP 範囲外で別 PR で導入予定）。
//
// 主要な domain メソッド:
//   - NewDraftPhotobook（新規 draft 作成）
//   - RestorePhotobook（DB から復元）
//   - Publish（draft → published、ドメイン側の状態遷移）
//   - ReissueManageUrl（manage_url_token 再発行、ドメイン側の状態遷移）
//   - TouchDraft（draft_expires_at 延長）
//   - SetCoverImage / ClearCoverImage / BumpVersion（編集系の不変インスタンス更新）
//
// **本ファイルは domain ロジックのみ**。DB UPDATE / Session revoke / Outbox 等の
// 副作用は UseCase 層の責務。entity は不変条件を守った新インスタンスを返すのみ。
//
// 不変条件（CHECK 制約と一致）:
//   - status='draft' のとき draft_edit_token_hash NOT NULL / draft_expires_at NOT NULL /
//     public_url_slug NULL / manage_url_token_hash NULL / published_at NULL / deleted_at NULL
//   - status='published' のとき draft_edit_token_hash NULL / draft_expires_at NULL /
//     public_url_slug NOT NULL / manage_url_token_hash NOT NULL / published_at NOT NULL /
//     deleted_at NULL
//   - version >= 0
//   - manage_url_token_version >= 0
package domain

import (
	"errors"
	"fmt"
	"time"

	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_version"
	"vrcpb/backend/internal/photobook/domain/vo/opening_style"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_layout"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_status"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_type"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
)

// 不変条件・状態遷移エラー。
var (
	ErrEmptyTitle             = errors.New("title must not be empty")
	ErrTitleTooLong           = errors.New("title too long (max 80)")
	ErrEmptyCreatorName       = errors.New("creator_display_name must not be empty")
	ErrCreatorNameTooLong     = errors.New("creator_display_name too long (max 50)")
	ErrDraftExpiresInPast     = errors.New("draft_expires_at must be in the future")
	ErrNotDraft               = errors.New("operation requires status=draft")
	ErrNotPublishedOrDeleted  = errors.New("operation requires status in {published, deleted}")
	ErrRightsNotAgreed        = errors.New("rights_agreed must be true to publish")
	ErrInvalidStateForRestore = errors.New("invalid state combination for restore")
	ErrPublishedAtMissing     = errors.New("published_at must be set when status is published or deleted")
	ErrPageLimitExceeded      = errors.New("photobook page limit exceeded (max 30)")
	ErrPhotoLimitExceeded     = errors.New("page photo limit exceeded (max 20)")
)

// MaxPagesPerPhotobook / MaxPhotosPerPage は MVP の上限（PR19 計画 Q6 / Q7）。
const (
	MaxPagesPerPhotobook = 30
	MaxPhotosPerPage     = 20
)

const (
	maxTitleLen        = 80
	maxCreatorNameLen  = 50
	defaultDraftTTL    = 7 * 24 * time.Hour
)

// Photobook は集約ルート。
type Photobook struct {
	id                    photobook_id.PhotobookID
	pbType                photobook_type.PhotobookType
	title                 string
	description           *string
	layout                photobook_layout.PhotobookLayout
	openingStyle          opening_style.OpeningStyle
	visibility            visibility.Visibility
	sensitive             bool
	rightsAgreed          bool
	rightsAgreedAt        *time.Time
	creatorDisplayName    string
	creatorXID            *string
	coverTitle            *string
	coverImageID          *image_id.ImageID
	publicUrlSlug         *slug.Slug
	manageUrlTokenHash    *manage_url_token_hash.ManageUrlTokenHash
	manageUrlTokenVersion manage_url_token_version.ManageUrlTokenVersion
	draftEditTokenHash    *draft_edit_token_hash.DraftEditTokenHash
	draftExpiresAt        *time.Time
	status                photobook_status.PhotobookStatus
	hiddenByOperator      bool
	version               int
	publishedAt           *time.Time
	createdAt             time.Time
	updatedAt             time.Time
	deletedAt             *time.Time
}

// NewDraftPhotobookParams は draft 作成の引数。
//
// draft 作成時は業務知識 v4 / 設計書の最小必須項目のみを引数で受け取る。
// layout / opening_style / visibility / sensitive 等は既定値を VO 側で固定し、
// 編集 UseCase（UpdateSettings 等）で後から上書きする設計。
type NewDraftPhotobookParams struct {
	ID                  photobook_id.PhotobookID
	Type                photobook_type.PhotobookType
	Title               string
	Layout              photobook_layout.PhotobookLayout
	OpeningStyle        opening_style.OpeningStyle
	Visibility          visibility.Visibility
	CreatorDisplayName  string
	RightsAgreed        bool
	DraftEditTokenHash  draft_edit_token_hash.DraftEditTokenHash
	Now                 time.Time
	DraftTTL            time.Duration // 0 なら 7 日（既定）
}

// NewDraftPhotobook は新規 draft Photobook を組み立てる。
//
// draft 状態の不変条件:
//   - draft_edit_token_hash 設定
//   - draft_expires_at = now + ttl
//   - public_url_slug / manage_url_token_hash / published_at は nil
func NewDraftPhotobook(p NewDraftPhotobookParams) (Photobook, error) {
	if err := validateTitle(p.Title); err != nil {
		return Photobook{}, err
	}
	if err := validateCreatorName(p.CreatorDisplayName); err != nil {
		return Photobook{}, err
	}
	ttl := p.DraftTTL
	if ttl == 0 {
		ttl = defaultDraftTTL
	}
	if ttl <= 0 {
		return Photobook{}, ErrDraftExpiresInPast
	}
	expires := p.Now.Add(ttl)

	hashCopy := p.DraftEditTokenHash
	pb := Photobook{
		id:                    p.ID,
		pbType:                p.Type,
		title:                 p.Title,
		layout:                p.Layout,
		openingStyle:          p.OpeningStyle,
		visibility:            p.Visibility,
		sensitive:             false,
		rightsAgreed:          p.RightsAgreed,
		creatorDisplayName:    p.CreatorDisplayName,
		manageUrlTokenVersion: manage_url_token_version.Zero(),
		draftEditTokenHash:    &hashCopy,
		draftExpiresAt:        &expires,
		status:                photobook_status.Draft(),
		hiddenByOperator:      false,
		version:               0,
		createdAt:             p.Now,
		updatedAt:             p.Now,
	}
	if p.RightsAgreed {
		t := p.Now
		pb.rightsAgreedAt = &t
	}
	return pb, nil
}

// RestorePhotobookParams は DB から取り出した行をドメインに復元する引数。
type RestorePhotobookParams struct {
	ID                    photobook_id.PhotobookID
	Type                  photobook_type.PhotobookType
	Title                 string
	Description           *string
	Layout                photobook_layout.PhotobookLayout
	OpeningStyle          opening_style.OpeningStyle
	Visibility            visibility.Visibility
	Sensitive             bool
	RightsAgreed          bool
	RightsAgreedAt        *time.Time
	CreatorDisplayName    string
	CreatorXID            *string
	CoverTitle            *string
	CoverImageID          *image_id.ImageID
	PublicUrlSlug         *slug.Slug
	ManageUrlTokenHash    *manage_url_token_hash.ManageUrlTokenHash
	ManageUrlTokenVersion manage_url_token_version.ManageUrlTokenVersion
	DraftEditTokenHash    *draft_edit_token_hash.DraftEditTokenHash
	DraftExpiresAt        *time.Time
	Status                photobook_status.PhotobookStatus
	HiddenByOperator      bool
	Version               int
	PublishedAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             *time.Time
}

// RestorePhotobook は DB row をドメインに復元する。
//
// 状態整合性は CHECK で守られている前提だが、二重防壁として再検証する。
func RestorePhotobook(p RestorePhotobookParams) (Photobook, error) {
	if p.Version < 0 {
		return Photobook{}, fmt.Errorf("invalid version: %d", p.Version)
	}
	switch {
	case p.Status.IsDraft():
		if p.DraftEditTokenHash == nil || p.DraftExpiresAt == nil {
			return Photobook{}, ErrInvalidStateForRestore
		}
		if p.PublicUrlSlug != nil || p.ManageUrlTokenHash != nil || p.PublishedAt != nil || p.DeletedAt != nil {
			return Photobook{}, ErrInvalidStateForRestore
		}
	case p.Status.IsPublished():
		if p.DraftEditTokenHash != nil || p.DraftExpiresAt != nil {
			return Photobook{}, ErrInvalidStateForRestore
		}
		if p.PublicUrlSlug == nil || p.ManageUrlTokenHash == nil || p.PublishedAt == nil {
			return Photobook{}, ErrPublishedAtMissing
		}
		if p.DeletedAt != nil {
			return Photobook{}, ErrInvalidStateForRestore
		}
	case p.Status.IsDeleted():
		if p.PublicUrlSlug == nil || p.ManageUrlTokenHash == nil || p.PublishedAt == nil || p.DeletedAt == nil {
			return Photobook{}, ErrInvalidStateForRestore
		}
	}
	return Photobook{
		id:                    p.ID,
		pbType:                p.Type,
		title:                 p.Title,
		description:           p.Description,
		layout:                p.Layout,
		openingStyle:          p.OpeningStyle,
		visibility:            p.Visibility,
		sensitive:             p.Sensitive,
		rightsAgreed:          p.RightsAgreed,
		rightsAgreedAt:        p.RightsAgreedAt,
		creatorDisplayName:    p.CreatorDisplayName,
		creatorXID:            p.CreatorXID,
		coverTitle:            p.CoverTitle,
		coverImageID:          p.CoverImageID,
		publicUrlSlug:         p.PublicUrlSlug,
		manageUrlTokenHash:    p.ManageUrlTokenHash,
		manageUrlTokenVersion: p.ManageUrlTokenVersion,
		draftEditTokenHash:    p.DraftEditTokenHash,
		draftExpiresAt:        p.DraftExpiresAt,
		status:                p.Status,
		hiddenByOperator:      p.HiddenByOperator,
		version:               p.Version,
		publishedAt:           p.PublishedAt,
		createdAt:             p.CreatedAt,
		updatedAt:             p.UpdatedAt,
		deletedAt:             p.DeletedAt,
	}, nil
}

// CanPublish は publish 条件 (I7) を満たすかを返す。
//
// 確認項目:
//   - status=draft
//   - rights_agreed=true
//   - creator_display_name 非空（コンストラクタで保証されているはずだが二重チェック）
func (p Photobook) CanPublish() error {
	if !p.status.IsDraft() {
		return ErrNotDraft
	}
	if !p.rightsAgreed {
		return ErrRightsNotAgreed
	}
	if p.creatorDisplayName == "" {
		return ErrEmptyCreatorName
	}
	return nil
}

// Publish は draft → published の状態遷移を行う。
//
// **DB 副作用は持たない**。新しい Photobook 値を返すのみ（不変）。
// UseCase 層はこの新値を repository.PublishFromDraft で永続化し、
// 同一 TX 内で Session revoke + Outbox INSERT を行う。
func (p Photobook) Publish(
	publishedSlug slug.Slug,
	manageHash manage_url_token_hash.ManageUrlTokenHash,
	now time.Time,
) (Photobook, error) {
	if err := p.CanPublish(); err != nil {
		return Photobook{}, err
	}
	out := p
	out.status = photobook_status.Published()
	out.publicUrlSlug = &publishedSlug
	out.manageUrlTokenHash = &manageHash
	out.manageUrlTokenVersion = manage_url_token_version.Zero()
	out.draftEditTokenHash = nil
	out.draftExpiresAt = nil
	publishedAt := now
	out.publishedAt = &publishedAt
	out.updatedAt = now
	out.version = p.version + 1
	return out, nil
}

// ReissueManageUrl は manage_url_token を新規発行する状態遷移。
//
// 対象状態: published / deleted。
// 副作用は持たない（UseCase 層で repository.ReissueManageUrl 呼び出し +
// Session revoke を同一 TX 内で行う）。
//
// 戻り値の Photobook は manage_url_token_hash 更新済 + version+1 +
// manage_url_token_version+1 の新値。
func (p Photobook) ReissueManageUrl(
	newHash manage_url_token_hash.ManageUrlTokenHash,
	now time.Time,
) (Photobook, manage_url_token_version.ManageUrlTokenVersion, error) {
	if !p.status.IsPublished() && !p.status.IsDeleted() {
		return Photobook{}, manage_url_token_version.Zero(), ErrNotPublishedOrDeleted
	}
	out := p
	out.manageUrlTokenHash = &newHash
	oldVersion := p.manageUrlTokenVersion
	out.manageUrlTokenVersion = oldVersion.Increment()
	out.updatedAt = now
	out.version = p.version + 1
	return out, oldVersion, nil
}

// CanEdit は draft 状態で編集系操作（page/photo 追加削除、cover 変更）を許可するかを返す。
//
// I-D6 / 業務知識 v4 §3.2 に従い、published / deleted / purged は不可。
func (p Photobook) CanEdit() error {
	if !p.status.IsDraft() {
		return ErrNotDraft
	}
	return nil
}

// BumpVersion は version+1 + updated_at = now の新インスタンスを返す（不変）。
//
// page/photo/cover 系の編集操作で UseCase 側が SQL を実行する前に取得する想定。
// Repository 側 UPDATE は WHERE version=$expected を持ち、整合は実 DB で再保証される。
func (p Photobook) BumpVersion(now time.Time) Photobook {
	out := p
	out.version = p.version + 1
	out.updatedAt = now
	return out
}

// SetCoverImage は cover_image_id を設定した新インスタンスを返す。
//
// 状態は draft 必須（CanEdit を呼ぶこと）。version+1 / updated_at 更新は呼び出し側で
// BumpVersion を経由する。
//
// 注意: image の owner_photobook_id 整合 / status==available の検証は Repository / UseCase
// 層で実施する。本メソッドは domain 不変条件のみ。
func (p Photobook) SetCoverImage(id image_id.ImageID, now time.Time) (Photobook, error) {
	if err := p.CanEdit(); err != nil {
		return Photobook{}, err
	}
	out := p
	cp := id
	out.coverImageID = &cp
	out.version = p.version + 1
	out.updatedAt = now
	return out, nil
}

// ClearCoverImage は cover_image_id を nil にした新インスタンスを返す。
func (p Photobook) ClearCoverImage(now time.Time) (Photobook, error) {
	if err := p.CanEdit(); err != nil {
		return Photobook{}, err
	}
	out := p
	out.coverImageID = nil
	out.version = p.version + 1
	out.updatedAt = now
	return out, nil
}

// TouchDraft は draft_expires_at を now + ttl に延長する状態遷移。
//
// 編集系 API 成功時のみ呼ぶ前提（GET / プレビューでは呼ばない、I-D4）。
// 副作用は持たない（UseCase 層で repository.TouchDraft 呼び出し）。
func (p Photobook) TouchDraft(now time.Time, ttl time.Duration) (Photobook, error) {
	if !p.status.IsDraft() {
		return Photobook{}, ErrNotDraft
	}
	if ttl == 0 {
		ttl = defaultDraftTTL
	}
	if ttl <= 0 {
		return Photobook{}, ErrDraftExpiresInPast
	}
	out := p
	expires := now.Add(ttl)
	out.draftExpiresAt = &expires
	out.updatedAt = now
	out.version = p.version + 1
	return out, nil
}

// === Getters ===

func (p Photobook) ID() photobook_id.PhotobookID                          { return p.id }
func (p Photobook) Type() photobook_type.PhotobookType                    { return p.pbType }
func (p Photobook) Title() string                                         { return p.title }
func (p Photobook) Description() *string                                  { return p.description }
func (p Photobook) Layout() photobook_layout.PhotobookLayout              { return p.layout }
func (p Photobook) OpeningStyle() opening_style.OpeningStyle              { return p.openingStyle }
func (p Photobook) Visibility() visibility.Visibility                     { return p.visibility }
func (p Photobook) Sensitive() bool                                       { return p.sensitive }
func (p Photobook) RightsAgreed() bool                                    { return p.rightsAgreed }
func (p Photobook) RightsAgreedAt() *time.Time                            { return clonePtrTime(p.rightsAgreedAt) }
func (p Photobook) CreatorDisplayName() string                            { return p.creatorDisplayName }
func (p Photobook) CreatorXID() *string                                   { return clonePtrString(p.creatorXID) }
func (p Photobook) CoverTitle() *string                                   { return clonePtrString(p.coverTitle) }
func (p Photobook) CoverImageID() *image_id.ImageID                       { return p.coverImageID }
func (p Photobook) PublicUrlSlug() *slug.Slug                             { return p.publicUrlSlug }
func (p Photobook) ManageUrlTokenHash() *manage_url_token_hash.ManageUrlTokenHash {
	return p.manageUrlTokenHash
}
func (p Photobook) ManageUrlTokenVersion() manage_url_token_version.ManageUrlTokenVersion {
	return p.manageUrlTokenVersion
}
func (p Photobook) DraftEditTokenHash() *draft_edit_token_hash.DraftEditTokenHash {
	return p.draftEditTokenHash
}
func (p Photobook) DraftExpiresAt() *time.Time                            { return clonePtrTime(p.draftExpiresAt) }
func (p Photobook) Status() photobook_status.PhotobookStatus              { return p.status }
func (p Photobook) HiddenByOperator() bool                                { return p.hiddenByOperator }
func (p Photobook) Version() int                                          { return p.version }
func (p Photobook) PublishedAt() *time.Time                               { return clonePtrTime(p.publishedAt) }
func (p Photobook) CreatedAt() time.Time                                  { return p.createdAt }
func (p Photobook) UpdatedAt() time.Time                                  { return p.updatedAt }
func (p Photobook) DeletedAt() *time.Time                                 { return clonePtrTime(p.deletedAt) }

func (p Photobook) IsDraft() bool     { return p.status.IsDraft() }
func (p Photobook) IsPublished() bool { return p.status.IsPublished() }

// === helpers ===

// validateTitle は draft 作成時の title 検証（length-only）。
//
// title は業務知識 v4 §3.1 で **任意**。draft 作成時に空文字を許容する（初回入口で
// ユーザに必須記入を強要しない設計）。publish 時に title の必須性が必要な場合は
// CanPublish 等の publish-time path で別途検証する。
//
// ErrEmptyTitle 定数は publish_handler のエラーマッピング（情報漏洩抑止のため
// 理由を区別しない）が引き続き参照するため保持する。
func validateTitle(s string) error {
	if len([]rune(s)) > maxTitleLen {
		return ErrTitleTooLong
	}
	return nil
}

// validateCreatorName は draft 作成時の creator_display_name 検証（length-only）。
//
// creator_display_name は業務知識 v4 §3.1 で **任意**。draft 作成時に空文字を許容する。
// publish 時の必須性は CanPublish (creatorDisplayName == "" を直接 check) で別途担保される。
func validateCreatorName(s string) error {
	if len([]rune(s)) > maxCreatorNameLen {
		return ErrCreatorNameTooLong
	}
	return nil
}

func clonePtrTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

func clonePtrString(s *string) *string {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}
