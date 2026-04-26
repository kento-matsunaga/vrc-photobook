package mime_type_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/image/domain/vo/mime_type"
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
		{name: "正常_image/jpeg", description: "Given: image/jpeg", input: "image/jpeg", want: "image/jpeg"},
		{name: "正常_image/png", description: "Given: image/png", input: "image/png", want: "image/png"},
		{name: "正常_image/webp", description: "Given: image/webp", input: "image/webp", want: "image/webp"},
		{name: "異常_image/heic", description: "Given: heic は variant に出ない", input: "image/heic", wantErr: true},
		{name: "異常_image/svg+xml", description: "Given: svg は禁止", input: "image/svg+xml", wantErr: true},
		{name: "異常_text/html", description: "Given: text/html, Then: error", input: "text/html", wantErr: true},
		{name: "異常_空文字", description: "Given: '', Then: error", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mime_type.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, mime_type.ErrInvalidMimeType) {
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
