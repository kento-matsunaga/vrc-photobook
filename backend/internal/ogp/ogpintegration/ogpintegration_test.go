// Package ogpintegration_test は SyncOutcome 分類ロジックを単体検証する。
//
// renderer / R2 / DB を伴う end-to-end は ogp/internal/usecase の既存 test に委ね、
// 本 test は ClassifyOgpErr の error → outcome マッピングだけを table 駆動で確認する。
package ogpintegration_test

import (
	"context"
	"errors"
	"testing"

	"vrcpb/backend/internal/ogp/ogpintegration"
	ogpusecase "vrcpb/backend/internal/ogp/internal/usecase"
)

func TestClassifyOgpErr(t *testing.T) {
	tests := []struct {
		name        string
		description string
		err         error
		want        ogpintegration.SyncOutcome
	}{
		{
			name:        "正常_nil_success",
			description: "Given: err=nil, When: classify, Then: success",
			err:         nil,
			want:        ogpintegration.SyncOutcomeSuccess,
		},
		{
			name:        "正常_context_deadline_exceeded_timeout",
			description: "Given: ctx deadline exceeded, When: classify, Then: timeout",
			err:         context.DeadlineExceeded,
			want:        ogpintegration.SyncOutcomeTimeout,
		},
		{
			name:        "正常_context_canceled_timeout",
			description: "Given: ctx canceled, When: classify, Then: timeout (キャンセルは timeout 扱い)",
			err:         context.Canceled,
			want:        ogpintegration.SyncOutcomeTimeout,
		},
		{
			name:        "正常_photobook_not_found_photobook_missing",
			description: "Given: ErrPhotobookNotFound, When: classify, Then: photobook_missing",
			err:         ogpusecase.ErrPhotobookNotFound,
			want:        ogpintegration.SyncOutcomePhotobookMissing,
		},
		{
			name:        "正常_not_published_not_published",
			description: "Given: ErrNotPublished, When: classify, Then: not_published",
			err:         ogpusecase.ErrNotPublished,
			want:        ogpintegration.SyncOutcomeNotPublished,
		},
		{
			name:        "正常_generic_error_error",
			description: "Given: generic error, When: classify, Then: error (default)",
			err:         errors.New("some other failure"),
			want:        ogpintegration.SyncOutcomeError,
		},
		{
			name:        "正常_wrapped_timeout_error_timeout",
			description: "Given: errors.Wrap(context.DeadlineExceeded), When: classify, Then: timeout (errors.Is チェーン)",
			err: func() error {
				return errors.Join(errors.New("render: deadline"), context.DeadlineExceeded)
			}(),
			want: ogpintegration.SyncOutcomeTimeout,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := ogpintegration.ClassifyOgpErr(tt.err)
			if got != tt.want {
				t.Fatalf("%s\nClassifyOgpErr(%v) = %q, want %q", tt.description, tt.err, got, tt.want)
			}
		})
	}
}

func TestSyncOutcomeString(t *testing.T) {
	tests := []struct {
		name        string
		description string
		outcome     ogpintegration.SyncOutcome
		want        string
	}{
		{
			name:        "正常_success",
			description: "Given: SyncOutcomeSuccess, When: String, Then: \"success\" (小文字 snake_case)",
			outcome:     ogpintegration.SyncOutcomeSuccess,
			want:        "success",
		},
		{
			name:        "正常_timeout",
			description: "Given: SyncOutcomeTimeout, When: String, Then: \"timeout\"",
			outcome:     ogpintegration.SyncOutcomeTimeout,
			want:        "timeout",
		},
		{
			name:        "正常_not_published",
			description: "Given: SyncOutcomeNotPublished, When: String, Then: \"not_published\"",
			outcome:     ogpintegration.SyncOutcomeNotPublished,
			want:        "not_published",
		},
		{
			name:        "正常_photobook_missing",
			description: "Given: SyncOutcomePhotobookMissing, When: String, Then: \"photobook_missing\"",
			outcome:     ogpintegration.SyncOutcomePhotobookMissing,
			want:        "photobook_missing",
		},
		{
			name:        "正常_error",
			description: "Given: SyncOutcomeError, When: String, Then: \"error\"",
			outcome:     ogpintegration.SyncOutcomeError,
			want:        "error",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.outcome.String()
			if got != tt.want {
				t.Fatalf("%s\nString() = %q, want %q", tt.description, got, tt.want)
			}
		})
	}
}
