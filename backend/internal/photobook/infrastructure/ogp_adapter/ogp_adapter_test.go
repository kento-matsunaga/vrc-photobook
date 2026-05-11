// Package ogp_adapter_test は SyncOutcome の値変換と nil-inner ガードを単体検証する。
//
// DB / R2 / renderer を伴う end-to-end は ogp/internal/usecase の既存 test に委ね、
// 本 test では adapter が
//   - ogpintegration.SyncOutcome → usecase.OgpSyncOutcome を 1:1 で変換する
//   - inner=nil の場合に Error outcome を返す（フェイルセーフ）
// の 2 点だけを confirm する。
package ogp_adapter_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/ogp/ogpintegration"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/ogp_adapter"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

func TestMapOutcome(t *testing.T) {
	tests := []struct {
		name        string
		description string
		input       ogpintegration.SyncOutcome
		want        usecase.OgpSyncOutcome
	}{
		{
			name:        "正常_success_mapping",
			description: "Given: ogpintegration.Success, When: MapOutcome, Then: usecase.Success",
			input:       ogpintegration.SyncOutcomeSuccess,
			want:        usecase.OgpSyncOutcomeSuccess,
		},
		{
			name:        "正常_timeout_mapping",
			description: "Given: ogpintegration.Timeout, When: MapOutcome, Then: usecase.Timeout",
			input:       ogpintegration.SyncOutcomeTimeout,
			want:        usecase.OgpSyncOutcomeTimeout,
		},
		{
			name:        "正常_not_published_mapping",
			description: "Given: ogpintegration.NotPublished, When: MapOutcome, Then: usecase.NotPublished",
			input:       ogpintegration.SyncOutcomeNotPublished,
			want:        usecase.OgpSyncOutcomeNotPublished,
		},
		{
			name:        "正常_photobook_missing_mapping",
			description: "Given: ogpintegration.PhotobookMissing, When: MapOutcome, Then: usecase.PhotobookMissing",
			input:       ogpintegration.SyncOutcomePhotobookMissing,
			want:        usecase.OgpSyncOutcomePhotobookMissing,
		},
		{
			name:        "正常_error_mapping",
			description: "Given: ogpintegration.Error, When: MapOutcome, Then: usecase.Error",
			input:       ogpintegration.SyncOutcomeError,
			want:        usecase.OgpSyncOutcomeError,
		},
		{
			name:        "異常_unknown_outcome_error_fallback",
			description: "Given: 未知の SyncOutcome 値, When: MapOutcome, Then: defensive に usecase.Error に倒す",
			input:       ogpintegration.SyncOutcome("__unknown__"),
			want:        usecase.OgpSyncOutcomeError,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := ogp_adapter.MapOutcome(tt.input)
			if got != tt.want {
				t.Fatalf("%s\nMapOutcome(%q) = %q, want %q", tt.description, tt.input, got, tt.want)
			}
		})
	}
}

func TestSyncGenerator_GenerateSync_NilInner_ReturnsError(t *testing.T) {
	// Given: inner=nil で構築された SyncGenerator
	// When: GenerateSync 呼び出し
	// Then: 内部 renderer / UC を呼ばずに Error outcome を即座に返す（フェイルセーフ）
	gen := ogp_adapter.NewSyncGenerator(nil)
	pid, err := photobook_id.FromUUID(uuid.New())
	if err != nil {
		t.Fatalf("photobook_id.FromUUID: %v", err)
	}
	got := gen.GenerateSync(context.Background(), pid, time.Now().UTC())
	if got != usecase.OgpSyncOutcomeError {
		t.Fatalf("GenerateSync with nil inner = %q, want %q", got, usecase.OgpSyncOutcomeError)
	}
}
