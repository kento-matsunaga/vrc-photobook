package image_usage_kind_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/image/domain/vo/image_usage_kind"
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		description string
		input       string
		want        string
		wantErr     bool
	}{
		{name: "正常_photo", description: "Given: photo, Then: photo", input: "photo", want: "photo"},
		{name: "正常_cover", description: "Given: cover, Then: cover", input: "cover", want: "cover"},
		{name: "正常_ogp", description: "Given: ogp, Then: ogp", input: "ogp", want: "ogp"},
		{name: "異常_creator_avatar", description: "Given: creator_avatar (Phase 2), Then: error", input: "creator_avatar", wantErr: true},
		{name: "異常_空文字", description: "Given: '', Then: error", input: "", wantErr: true},
		{name: "異常_未知の値", description: "Given: random, Then: error", input: "icon", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := image_usage_kind.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, image_usage_kind.ErrInvalidImageUsageKind) {
					t.Errorf("err = %v", err)
				}
				return
			}
			if got.String() != tt.want {
				t.Errorf("got %q want %q", got.String(), tt.want)
			}
		})
	}
}

func TestPredicates(t *testing.T) {
	t.Parallel()
	if !image_usage_kind.Photo().IsPhoto() {
		t.Error("Photo IsPhoto false")
	}
	if !image_usage_kind.Cover().IsCover() {
		t.Error("Cover IsCover false")
	}
	if !image_usage_kind.Ogp().IsOgp() {
		t.Error("Ogp IsOgp false")
	}
	if image_usage_kind.Photo().Equal(image_usage_kind.Cover()) {
		t.Error("Photo == Cover should be false")
	}
}
