// PhotobookPublishedHandler の単体テスト（fake OgpGenerator 経由、DB 不要）。
package handlers_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/outbox/contract"
	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
	"vrcpb/backend/internal/outbox/internal/usecase/handlers"
)

type fakeGenerator struct {
	called      bool
	gotID       uuid.UUID
	returnRes   contract.OgpGenerateResult
	returnErr   error
}

func (f *fakeGenerator) GenerateForPhotobook(_ context.Context, id uuid.UUID, _ time.Time) (contract.OgpGenerateResult, error) {
	f.called = true
	f.gotID = id
	return f.returnRes, f.returnErr
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func mkEvent(t *testing.T, payload string) outboxusecase.EventTarget {
	t.Helper()
	return outboxusecase.EventTarget{
		ID:            uuid.New(),
		AggregateType: "photobook",
		AggregateID:   uuid.New(),
		EventType:     "photobook.published",
		Payload:       []byte(payload),
		Attempts:      0,
	}
}

func TestPhotobookPublishedHandler(t *testing.T) {
	pid := uuid.New()
	tests := []struct {
		name        string
		description string
		gen         *fakeGenerator
		payload     string
		wantErrIs   error // errors.Is で照合（nil の場合は err==nil 期待）
		wantCalled  bool
	}{
		{
			name:        "正常_payload有効_generator成功なら nil 返却",
			description: "Given: 有効 payload + generator success, When: Handle, Then: nil",
			gen: &fakeGenerator{
				returnRes: contract.OgpGenerateResult{OgpImageID: uuid.New(), Generated: true},
			},
			payload:    `{"event_version":1,"photobook_id":"` + pid.String() + `"}`,
			wantErrIs:  nil,
			wantCalled: true,
		},
		{
			name:        "正常_NotPublishedSkippable は nil 返却（permanent skip）",
			description: "Given: generator が ErrNotPublishedSkippable, When: Handle, Then: nil（processed 扱い）",
			gen: &fakeGenerator{
				returnErr: contract.ErrNotPublishedSkippable,
			},
			payload:    `{"photobook_id":"` + pid.String() + `"}`,
			wantErrIs:  nil,
			wantCalled: true,
		},
		{
			name:        "異常_payload broken は decode error",
			description: "Given: 不正 JSON, When: Handle, Then: error 返却（worker が retry/dead に倒す）",
			gen:         &fakeGenerator{},
			payload:     `{not-json}`,
			wantErrIs:   nil, // wantErrIs nil + 実際は err != nil の混乱を避けるため別検査
			wantCalled:  false,
		},
		{
			name:        "異常_photobook_id 無し / 不正 UUID は error",
			description: "Given: photobook_id が UUID 形式でない, When: Handle, Then: error",
			gen:         &fakeGenerator{},
			payload:     `{"photobook_id":"not-a-uuid"}`,
			wantErrIs:   nil,
			wantCalled:  false,
		},
		{
			name:        "異常_generator が一般 error なら そのまま伝播",
			description: "Given: generator が transient error, When: Handle, Then: error（worker が retry）",
			gen: &fakeGenerator{
				returnErr: errors.New("transient db error"),
			},
			payload:    `{"photobook_id":"` + pid.String() + `"}`,
			wantErrIs:  nil,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handlers.NewPhotobookPublishedHandler(tt.gen, discardLogger())
			ev := mkEvent(t, tt.payload)
			err := h.Handle(context.Background(), ev)

			switch tt.name {
			case "正常_payload有効_generator成功なら nil 返却",
				"正常_NotPublishedSkippable は nil 返却（permanent skip）":
				if err != nil {
					t.Errorf("expected nil, got %v", err)
				}
			case "異常_payload broken は decode error",
				"異常_photobook_id 無し / 不正 UUID は error":
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			case "異常_generator が一般 error なら そのまま伝播":
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			}
			if tt.gen.called != tt.wantCalled {
				t.Errorf("called=%v want %v", tt.gen.called, tt.wantCalled)
			}
		})
	}
}
