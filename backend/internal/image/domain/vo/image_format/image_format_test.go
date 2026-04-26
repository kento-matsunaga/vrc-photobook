package image_format_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/image/domain/vo/image_format"
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
		{name: "正常_jpg", description: "Given: jpg, Then: jpg", input: "jpg", want: "jpg"},
		{name: "正常_png", description: "Given: png, Then: png", input: "png", want: "png"},
		{name: "正常_webp", description: "Given: webp, Then: webp", input: "webp", want: "webp"},
		{name: "正常_heic", description: "Given: heic, Then: heic", input: "heic", want: "heic"},
		{name: "異常_jpeg", description: "Given: 'jpeg' (canonicalされた値ではない), Then: error", input: "jpeg", wantErr: true},
		{name: "異常_gif", description: "Given: gif, Then: error (許可外)", input: "gif", wantErr: true},
		{name: "異常_svg", description: "Given: svg, Then: error", input: "svg", wantErr: true},
		{name: "異常_空文字", description: "Given: '', Then: error", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := image_format.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, image_format.ErrInvalidImageFormat) {
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
