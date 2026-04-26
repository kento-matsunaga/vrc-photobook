package domain_test

import (
	"testing"
	"time"

	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

func newPageID(t *testing.T) page_id.PageID {
	t.Helper()
	id, err := page_id.New()
	if err != nil {
		t.Fatalf("page_id.New: %v", err)
	}
	return id
}

func newPhotobookIDForPage(t *testing.T) photobook_id.PhotobookID {
	t.Helper()
	id, err := photobook_id.New()
	if err != nil {
		t.Fatalf("photobook_id.New: %v", err)
	}
	return id
}

func TestNewPage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	cap1, _ := caption.New("first page")

	tests := []struct {
		name    string
		params  func(t *testing.T) domain.NewPageParams
		wantErr bool
	}{
		{
			name: "正常_caption_あり",
			params: func(t *testing.T) domain.NewPageParams {
				return domain.NewPageParams{
					ID:           newPageID(t),
					PhotobookID:  newPhotobookIDForPage(t),
					DisplayOrder: display_order.Zero(),
					Caption:      &cap1,
					Now:          now,
				}
			},
		},
		{
			name: "正常_caption_nil",
			params: func(t *testing.T) domain.NewPageParams {
				return domain.NewPageParams{
					ID:           newPageID(t),
					PhotobookID:  newPhotobookIDForPage(t),
					DisplayOrder: display_order.Zero(),
					Caption:      nil,
					Now:          now,
				}
			},
		},
		{
			name: "異常_now_zero",
			params: func(t *testing.T) domain.NewPageParams {
				return domain.NewPageParams{
					ID:           newPageID(t),
					PhotobookID:  newPhotobookIDForPage(t),
					DisplayOrder: display_order.Zero(),
				}
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.params(t)
			got, err := domain.NewPage(p)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !got.ID().Equal(p.ID) {
				t.Errorf("ID mismatch")
			}
			if !got.DisplayOrder().Equal(p.DisplayOrder) {
				t.Errorf("DisplayOrder mismatch")
			}
		})
	}
}

func TestPageReorder(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	p, _ := domain.NewPage(domain.NewPageParams{
		ID:           newPageID(t),
		PhotobookID:  newPhotobookIDForPage(t),
		DisplayOrder: display_order.Zero(),
		Now:          now,
	})
	newOrder, _ := display_order.New(5)
	out := p.Reorder(newOrder, now.Add(time.Second))
	if out.DisplayOrder().Int() != 5 {
		t.Errorf("DisplayOrder not updated")
	}
	if !out.UpdatedAt().Equal(now.Add(time.Second)) {
		t.Errorf("updated_at not bumped")
	}
}
