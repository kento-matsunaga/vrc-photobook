package image_status_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/image/domain/vo/image_status"
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "正常_uploading", input: "uploading", want: "uploading"},
		{name: "正常_processing", input: "processing", want: "processing"},
		{name: "正常_available", input: "available", want: "available"},
		{name: "正常_failed", input: "failed", want: "failed"},
		{name: "正常_deleted", input: "deleted", want: "deleted"},
		{name: "正常_purged", input: "purged", want: "purged"},
		{name: "異常_rejected_は不採用", input: "rejected", wantErr: true},
		{name: "異常_空文字", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := image_status.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, image_status.ErrInvalidImageStatus) {
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
