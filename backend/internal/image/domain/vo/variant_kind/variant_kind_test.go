package variant_kind_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/image/domain/vo/variant_kind"
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
		{name: "正常_original", description: "Given: original", input: "original", want: "original"},
		{name: "正常_display", description: "Given: display", input: "display", want: "display"},
		{name: "正常_thumbnail", description: "Given: thumbnail", input: "thumbnail", want: "thumbnail"},
		{name: "正常_ogp", description: "Given: ogp", input: "ogp", want: "ogp"},
		{name: "異常_main", description: "Given: 'main', Then: error", input: "main", wantErr: true},
		{name: "異常_空文字", description: "Given: '', Then: error", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := variant_kind.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, variant_kind.ErrInvalidVariantKind) {
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
	if !variant_kind.Display().IsDisplay() {
		t.Error("Display IsDisplay false")
	}
	if !variant_kind.Thumbnail().IsThumbnail() {
		t.Error("Thumbnail IsThumbnail false")
	}
	if !variant_kind.Original().IsOriginal() {
		t.Error("Original IsOriginal false")
	}
	if !variant_kind.Ogp().IsOgp() {
		t.Error("Ogp IsOgp false")
	}
}
