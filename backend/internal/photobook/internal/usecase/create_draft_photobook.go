package usecase

import (
	"context"
	"fmt"
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

// CreateDraftPhotobookInput は draft Photobook 作成の入力。
//
// PR9b 段階では編集 UI が無いため、最小必須項目のみを引数に取る。
// type / layout / opening_style は呼び出し側で決定（既定は Memory / Simple / Light）。
type CreateDraftPhotobookInput struct {
	Type               photobook_type.PhotobookType
	Title              string
	Layout             photobook_layout.PhotobookLayout
	OpeningStyle       opening_style.OpeningStyle
	Visibility         visibility.Visibility
	CreatorDisplayName string
	RightsAgreed       bool
	Now                time.Time
	DraftTTL           time.Duration
}

// CreateDraftPhotobookOutput は発行結果。
//
// RawDraftToken は **作成者に渡すためだけに使う**。ログ出力禁止。
type CreateDraftPhotobookOutput struct {
	Photobook     domain.Photobook
	RawDraftToken draft_edit_token.DraftEditToken
}

// CreateDraftPhotobook は draft Photobook を新規作成する UseCase。
type CreateDraftPhotobook struct {
	repo PhotobookRepository
}

// NewCreateDraftPhotobook は UseCase を組み立てる。
func NewCreateDraftPhotobook(repo PhotobookRepository) *CreateDraftPhotobook {
	return &CreateDraftPhotobook{repo: repo}
}

// Execute は raw DraftEditToken を生成し、SHA-256 hash を Photobook に保存する。
func (u *CreateDraftPhotobook) Execute(
	ctx context.Context,
	in CreateDraftPhotobookInput,
) (CreateDraftPhotobookOutput, error) {
	id, err := photobook_id.New()
	if err != nil {
		return CreateDraftPhotobookOutput{}, fmt.Errorf("photobook id: %w", err)
	}
	tok, err := draft_edit_token.Generate()
	if err != nil {
		return CreateDraftPhotobookOutput{}, fmt.Errorf("draft token: %w", err)
	}
	pb, err := domain.NewDraftPhotobook(domain.NewDraftPhotobookParams{
		ID:                 id,
		Type:               in.Type,
		Title:              in.Title,
		Layout:             in.Layout,
		OpeningStyle:       in.OpeningStyle,
		Visibility:         in.Visibility,
		CreatorDisplayName: in.CreatorDisplayName,
		RightsAgreed:       in.RightsAgreed,
		DraftEditTokenHash: draft_edit_token_hash.Of(tok),
		Now:                in.Now,
		DraftTTL:           in.DraftTTL,
	})
	if err != nil {
		return CreateDraftPhotobookOutput{}, err
	}
	if err := u.repo.CreateDraft(ctx, pb); err != nil {
		return CreateDraftPhotobookOutput{}, fmt.Errorf("create draft: %w", err)
	}
	return CreateDraftPhotobookOutput{Photobook: pb, RawDraftToken: tok}, nil
}
