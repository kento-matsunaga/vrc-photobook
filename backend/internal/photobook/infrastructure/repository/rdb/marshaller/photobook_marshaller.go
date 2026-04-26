// Package marshaller はドメインの Photobook と sqlc 生成物 Photobook row の相互変換を担う。
//
// 設計方針（.agents/rules/domain-standard.md §インフラ）:
//   - VO ↔ プリミティブ変換は本パッケージに閉じる
//   - DB エラーはそのまま返し、ビジネス例外への変換は repository が担う
package marshaller

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/photobook/domain"
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
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb/sqlcgen"
)

var ErrInvalidRow = errors.New("invalid photobook row from db")

// ToCreateParams は draft Photobook を sqlc CreateDraftPhotobookParams に変換する。
//
// CreateDraftPhotobook は status='draft' を SQL 側で固定するため、
// 状態遷移後の Photobook（published 等）には使えない。draft 専用。
func ToCreateParams(p domain.Photobook) (sqlcgen.CreateDraftPhotobookParams, error) {
	if !p.IsDraft() {
		return sqlcgen.CreateDraftPhotobookParams{}, errors.New("ToCreateParams expects status=draft")
	}
	if p.DraftEditTokenHash() == nil || p.DraftExpiresAt() == nil {
		return sqlcgen.CreateDraftPhotobookParams{}, errors.New("draft requires draft_edit_token_hash and draft_expires_at")
	}
	return sqlcgen.CreateDraftPhotobookParams{
		ID:                  uuidToPg(p.ID().UUID()),
		Type:                p.Type().String(),
		Title:               p.Title(),
		Description:         p.Description(),
		Layout:              p.Layout().String(),
		OpeningStyle:        p.OpeningStyle().String(),
		Visibility:          p.Visibility().String(),
		Sensitive:           p.Sensitive(),
		RightsAgreed:        p.RightsAgreed(),
		RightsAgreedAt:      timePtrToPg(p.RightsAgreedAt()),
		CreatorDisplayName:  p.CreatorDisplayName(),
		CreatorXID:          p.CreatorXID(),
		CoverTitle:          p.CoverTitle(),
		CoverImageID:        coverImageIDToPg(p.CoverImageID()),
		DraftEditTokenHash:  p.DraftEditTokenHash().Bytes(),
		DraftExpiresAt:      timeToPg(*p.DraftExpiresAt()),
		Version:             int32(p.Version()),
		CreatedAt:           timeToPg(p.CreatedAt()),
		UpdatedAt:           timeToPg(p.UpdatedAt()),
	}, nil
}

// FromRow は sqlcgen.Photobook をドメイン Photobook に変換する。
func FromRow(row sqlcgen.Photobook) (domain.Photobook, error) {
	if !row.ID.Valid {
		return domain.Photobook{}, ErrInvalidRow
	}
	id, err := photobook_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return domain.Photobook{}, err
	}
	pbType, err := photobook_type.Parse(row.Type)
	if err != nil {
		return domain.Photobook{}, err
	}
	layout, err := photobook_layout.Parse(row.Layout)
	if err != nil {
		return domain.Photobook{}, err
	}
	style, err := opening_style.Parse(row.OpeningStyle)
	if err != nil {
		return domain.Photobook{}, err
	}
	vis, err := visibility.Parse(row.Visibility)
	if err != nil {
		return domain.Photobook{}, err
	}
	status, err := photobook_status.Parse(row.Status)
	if err != nil {
		return domain.Photobook{}, err
	}
	ver, err := manage_url_token_version.New(int(row.ManageUrlTokenVersion))
	if err != nil {
		return domain.Photobook{}, err
	}

	var coverImageID *image_id.ImageID
	if row.CoverImageID.Valid {
		c, err := image_id.FromUUID(row.CoverImageID.Bytes)
		if err == nil {
			coverImageID = &c
		}
	}
	var publicSlug *slug.Slug
	if row.PublicUrlSlug != nil {
		s, err := slug.Parse(*row.PublicUrlSlug)
		if err != nil {
			return domain.Photobook{}, err
		}
		publicSlug = &s
	}
	var manageHash *manage_url_token_hash.ManageUrlTokenHash
	if len(row.ManageUrlTokenHash) > 0 {
		h, err := manage_url_token_hash.FromBytes(row.ManageUrlTokenHash)
		if err != nil {
			return domain.Photobook{}, err
		}
		manageHash = &h
	}
	var draftHash *draft_edit_token_hash.DraftEditTokenHash
	if len(row.DraftEditTokenHash) > 0 {
		h, err := draft_edit_token_hash.FromBytes(row.DraftEditTokenHash)
		if err != nil {
			return domain.Photobook{}, err
		}
		draftHash = &h
	}

	return domain.RestorePhotobook(domain.RestorePhotobookParams{
		ID:                    id,
		Type:                  pbType,
		Title:                 row.Title,
		Description:           row.Description,
		Layout:                layout,
		OpeningStyle:          style,
		Visibility:            vis,
		Sensitive:             row.Sensitive,
		RightsAgreed:          row.RightsAgreed,
		RightsAgreedAt:        pgToTimePtr(row.RightsAgreedAt),
		CreatorDisplayName:    row.CreatorDisplayName,
		CreatorXID:            row.CreatorXID,
		CoverTitle:            row.CoverTitle,
		CoverImageID:          coverImageID,
		PublicUrlSlug:         publicSlug,
		ManageUrlTokenHash:    manageHash,
		ManageUrlTokenVersion: ver,
		DraftEditTokenHash:    draftHash,
		DraftExpiresAt:        pgToTimePtr(row.DraftExpiresAt),
		Status:                status,
		HiddenByOperator:      row.HiddenByOperator,
		Version:               int(row.Version),
		PublishedAt:           pgToTimePtr(row.PublishedAt),
		CreatedAt:             row.CreatedAt.Time,
		UpdatedAt:             row.UpdatedAt.Time,
		DeletedAt:             pgToTimePtr(row.DeletedAt),
	})
}

// === helpers ===

func uuidToPg(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func coverImageIDToPg(p *image_id.ImageID) pgtype.UUID {
	if p == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: p.UUID(), Valid: true}
}

func timeToPg(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func timePtrToPg(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func pgToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	x := t.Time
	return &x
}
