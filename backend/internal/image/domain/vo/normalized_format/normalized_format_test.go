package normalized_format_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/image/domain/vo/normalized_format"
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
		{name: "正常_webp", description: "Given: webp, Then: webp", input: "webp", want: "webp"},
		{name: "異常_png_は不可", description: "Given: png (正規化形式に含まれない), Then: error", input: "png", wantErr: true},
		{name: "異常_heic_は不可", description: "Given: heic, Then: error", input: "heic", wantErr: true},
		{name: "異常_空文字", description: "Given: '', Then: error", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalized_format.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, normalized_format.ErrInvalidNormalizedFormat) {
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
