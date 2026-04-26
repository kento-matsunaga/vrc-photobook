package storage_key_test

import (
	"errors"
	"strings"
	"testing"

	"vrcpb/backend/internal/image/domain/vo/image_format"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/image/domain/vo/storage_key"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

func newIDs(t *testing.T) (photobook_id.PhotobookID, image_id.ImageID) {
	t.Helper()
	pid, err := photobook_id.New()
	if err != nil {
		t.Fatalf("photobook_id.New: %v", err)
	}
	iid, err := image_id.New()
	if err != nil {
		t.Fatalf("image_id.New: %v", err)
	}
	return pid, iid
}

func TestGenerateForVariant_Display(t *testing.T) {
	t.Parallel()
	pid, iid := newIDs(t)
	got, err := storage_key.GenerateForVariant(pid, iid, variant_kind.Display())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	s := got.String()
	want := "photobooks/" + pid.String() + "/images/" + iid.String() + "/display/"
	if !strings.HasPrefix(s, want) {
		t.Errorf("prefix mismatch: %q", s)
	}
	if !strings.HasSuffix(s, ".webp") {
		t.Errorf("suffix mismatch: %q", s)
	}
	// 16 文字 base64url の random 部 + ".webp"
	tail := strings.TrimPrefix(s, want)
	if len(tail) != len("XXXXXXXXXXXXXXXX.webp") {
		t.Errorf("tail length = %d", len(tail))
	}
}

func TestGenerateForVariant_Thumbnail(t *testing.T) {
	t.Parallel()
	pid, iid := newIDs(t)
	got, err := storage_key.GenerateForVariant(pid, iid, variant_kind.Thumbnail())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(got.String(), "/thumbnail/") {
		t.Errorf("thumbnail not in path: %q", got.String())
	}
	if !strings.HasSuffix(got.String(), ".webp") {
		t.Errorf("thumbnail must end with .webp")
	}
}

func TestGenerateForVariant_OriginalAndOgpRouting(t *testing.T) {
	t.Parallel()
	pid, iid := newIDs(t)
	if _, err := storage_key.GenerateForVariant(pid, iid, variant_kind.Original()); err == nil {
		t.Error("original should route to GenerateForOriginal")
	}
	if _, err := storage_key.GenerateForVariant(pid, iid, variant_kind.Ogp()); err == nil {
		t.Error("ogp should route to GenerateForOgp")
	}
}

func TestGenerateForOriginal(t *testing.T) {
	t.Parallel()
	pid, iid := newIDs(t)
	tests := []struct {
		name string
		fmt  image_format.ImageFormat
		ext  string
	}{
		{name: "正常_jpg", fmt: image_format.Jpg(), ext: ".jpg"},
		{name: "正常_png", fmt: image_format.Png(), ext: ".png"},
		{name: "正常_webp", fmt: image_format.Webp(), ext: ".webp"},
		{name: "正常_heic", fmt: image_format.Heic(), ext: ".heic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := storage_key.GenerateForOriginal(pid, iid, tt.fmt)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if !strings.Contains(got.String(), "/original/") {
				t.Errorf("original not in path: %q", got.String())
			}
			if !strings.HasSuffix(got.String(), tt.ext) {
				t.Errorf("ext mismatch: %q want %q", got.String(), tt.ext)
			}
		})
	}
}

func TestGenerateForOgp(t *testing.T) {
	t.Parallel()
	pid, iid := newIDs(t)
	got, err := storage_key.GenerateForOgp(pid, iid)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	want := "photobooks/" + pid.String() + "/ogp/" + iid.String() + "/"
	if !strings.HasPrefix(got.String(), want) {
		t.Errorf("prefix mismatch: %q", got.String())
	}
	if !strings.HasSuffix(got.String(), ".png") {
		t.Errorf("ogp must end with .png")
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "正常_有効なpath", input: "photobooks/abc/images/def/display/aaaa.webp"},
		{name: "異常_空文字", input: "", wantErr: storage_key.ErrEmptyStorageKey},
		{name: "異常_prefix違反", input: "images/abc/display/x.webp", wantErr: storage_key.ErrInvalidStorageKey},
		{name: "異常_長すぎる", input: strings.Repeat("a", 1024), wantErr: storage_key.ErrStorageKeyTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := storage_key.Parse(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("err = %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestRandomnessAcrossCalls(t *testing.T) {
	t.Parallel()
	pid, iid := newIDs(t)
	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		got, err := storage_key.GenerateForVariant(pid, iid, variant_kind.Display())
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if _, dup := seen[got.String()]; dup {
			t.Fatalf("duplicate at %d", i)
		}
		seen[got.String()] = struct{}{}
	}
}
