package failure_reason_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/image/domain/vo/failure_reason"
)

func TestParseAll12(t *testing.T) {
	t.Parallel()
	all := []string{
		"file_too_large",
		"size_mismatch",
		"unsupported_format",
		"svg_not_allowed",
		"animated_image_not_allowed",
		"dimensions_too_large",
		"decode_failed",
		"exif_strip_failed",
		"heic_conversion_failed",
		"variant_generation_failed",
		"object_not_found",
		"unknown",
	}
	for _, raw := range all {
		raw := raw
		t.Run("正常_"+raw, func(t *testing.T) {
			got, err := failure_reason.Parse(raw)
			if err != nil {
				t.Fatalf("Parse(%q): %v", raw, err)
			}
			if got.String() != raw {
				t.Errorf("got %q want %q", got.String(), raw)
			}
		})
	}
}

func TestParseUnknown(t *testing.T) {
	t.Parallel()
	bad := []string{"", "rejected", "magic_mismatch", "metadata_strip_failed"}
	for _, b := range bad {
		b := b
		t.Run("異常_"+b, func(t *testing.T) {
			_, err := failure_reason.Parse(b)
			if !errors.Is(err, failure_reason.ErrInvalidFailureReason) {
				t.Fatalf("err = %v want ErrInvalidFailureReason", err)
			}
		})
	}
}
