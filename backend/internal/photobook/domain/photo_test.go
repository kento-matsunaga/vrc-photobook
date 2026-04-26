package domain_test

import (
	"testing"
	"time"

	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
)

func newPhotoID(t *testing.T) photo_id.PhotoID {
	t.Helper()
	id, err := photo_id.New()
	if err != nil {
		t.Fatalf("photo_id.New: %v", err)
	}
	return id
}

func newImgID(t *testing.T) image_id.ImageID {
	t.Helper()
	id, err := image_id.New()
	if err != nil {
		t.Fatalf("image_id.New: %v", err)
	}
	return id
}

func newPageIDForPhoto(t *testing.T) page_id.PageID {
	t.Helper()
	id, err := page_id.New()
	if err != nil {
		t.Fatalf("page_id.New: %v", err)
	}
	return id
}

func TestNewPhoto(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	cap1, _ := caption.New("photo caption")

	tests := []struct {
		name    string
		params  func(t *testing.T) domain.NewPhotoParams
		wantErr bool
	}{
		{
			name: "正常_caption_あり",
			params: func(t *testing.T) domain.NewPhotoParams {
				return domain.NewPhotoParams{
					ID:           newPhotoID(t),
					PageID:       newPageIDForPhoto(t),
					ImageID:      newImgID(t),
					DisplayOrder: display_order.Zero(),
					Caption:      &cap1,
					Now:          now,
				}
			},
		},
		{
			name: "異常_now_zero",
			params: func(t *testing.T) domain.NewPhotoParams {
				return domain.NewPhotoParams{
					ID:           newPhotoID(t),
					PageID:       newPageIDForPhoto(t),
					ImageID:      newImgID(t),
					DisplayOrder: display_order.Zero(),
				}
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.params(t)
			got, err := domain.NewPhoto(p)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !got.ID().Equal(p.ID) {
				t.Errorf("ID mismatch")
			}
			if !got.ImageID().Equal(p.ImageID) {
				t.Errorf("ImageID mismatch")
			}
		})
	}
}
