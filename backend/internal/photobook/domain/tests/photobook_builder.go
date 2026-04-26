// Package tests は Photobook ドメインのテスト用 Builder を提供する。
//
// 方針（.agents/rules/testing.md §Builder パターン）:
//   - Builder は t を保持しない（Build(t) で受け取る）
//   - メソッドテストの前提条件構築に使う
//   - コンストラクタテスト（NewDraftPhotobook / RestorePhotobook）では使わず、引数を直接渡す
package tests

import (
	"testing"
	"time"

	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/opening_style"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_layout"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_type"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
)

// PhotobookBuilder は draft Photobook の前提条件を組み立てる Builder。
type PhotobookBuilder struct {
	id                 *photobook_id.PhotobookID
	pbType             photobook_type.PhotobookType
	title              string
	layout             photobook_layout.PhotobookLayout
	openingStyle       opening_style.OpeningStyle
	visibility         visibility.Visibility
	creatorDisplayName string
	rightsAgreed       bool
	now                time.Time
	ttl                time.Duration
	tokenHash          *draft_edit_token_hash.DraftEditTokenHash
}

// NewPhotobookBuilder は既定値の draft Photobook Builder を返す。
//
// 既定値:
//   - type=memory, layout=simple, opening_style=light, visibility=unlisted
//   - title="Test Photobook"
//   - creator_display_name="Tester"
//   - rights_agreed=true（PR9b の publish テスト便宜）
//   - ttl=7 日
func NewPhotobookBuilder() *PhotobookBuilder {
	return &PhotobookBuilder{
		pbType:             photobook_type.Memory(),
		title:              "Test Photobook",
		layout:             photobook_layout.Simple(),
		openingStyle:       opening_style.Light(),
		visibility:         visibility.Unlisted(),
		creatorDisplayName: "Tester",
		rightsAgreed:       true,
		now:                time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
		ttl:                7 * 24 * time.Hour,
	}
}

// WithID は photobook_id を上書きする。
func (b *PhotobookBuilder) WithID(id photobook_id.PhotobookID) *PhotobookBuilder {
	b.id = &id
	return b
}

// WithTitle は title を上書きする。
func (b *PhotobookBuilder) WithTitle(s string) *PhotobookBuilder {
	b.title = s
	return b
}

// WithCreatorName は creator_display_name を上書きする。
func (b *PhotobookBuilder) WithCreatorName(s string) *PhotobookBuilder {
	b.creatorDisplayName = s
	return b
}

// WithRightsAgreed は rights_agreed を上書きする。
func (b *PhotobookBuilder) WithRightsAgreed(v bool) *PhotobookBuilder {
	b.rightsAgreed = v
	return b
}

// WithNow は now を上書きする（draft_expires_at の起点）。
func (b *PhotobookBuilder) WithNow(t time.Time) *PhotobookBuilder {
	b.now = t
	return b
}

// WithDraftTTL は draft_expires_at の TTL を上書きする。
func (b *PhotobookBuilder) WithDraftTTL(d time.Duration) *PhotobookBuilder {
	b.ttl = d
	return b
}

// WithTokenHash は draft_edit_token_hash を上書きする（特定 hash を仕込みたいとき）。
func (b *PhotobookBuilder) WithTokenHash(h draft_edit_token_hash.DraftEditTokenHash) *PhotobookBuilder {
	b.tokenHash = &h
	return b
}

// Build は draft Photobook を生成する。
func (b *PhotobookBuilder) Build(t *testing.T) domain.Photobook {
	t.Helper()
	id := b.id
	if id == nil {
		v, err := photobook_id.New()
		if err != nil {
			t.Fatalf("photobook_id.New: %v", err)
		}
		id = &v
	}
	hash := b.tokenHash
	if hash == nil {
		tok, err := draft_edit_token.Generate()
		if err != nil {
			t.Fatalf("draft_edit_token.Generate: %v", err)
		}
		h := draft_edit_token_hash.Of(tok)
		hash = &h
	}
	pb, err := domain.NewDraftPhotobook(domain.NewDraftPhotobookParams{
		ID:                 *id,
		Type:               b.pbType,
		Title:              b.title,
		Layout:             b.layout,
		OpeningStyle:       b.openingStyle,
		Visibility:         b.visibility,
		CreatorDisplayName: b.creatorDisplayName,
		RightsAgreed:       b.rightsAgreed,
		DraftEditTokenHash: *hash,
		Now:                b.now,
		DraftTTL:           b.ttl,
	})
	if err != nil {
		t.Fatalf("NewDraftPhotobook: %v", err)
	}
	return pb
}
