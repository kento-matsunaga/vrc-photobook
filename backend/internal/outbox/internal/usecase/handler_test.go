// HandlerRegistry 単体テスト（DB 不要）。
package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
)

type stubHandler struct {
	called bool
	err    error
}

func (h *stubHandler) Handle(_ context.Context, _ outboxusecase.EventTarget) error {
	h.called = true
	return h.err
}

func TestHandlerRegistry(t *testing.T) {
	tests := []struct {
		name        string
		description string
		setup       func(r *outboxusecase.HandlerRegistry) (handler *stubHandler, lookupKey string)
		wantFound   bool
		wantHandle  bool // Handle まで通したか
	}{
		{
			name:        "正常_登録した key で lookup できる",
			description: "Given: registry に handler を Register / When: Lookup / Then: 同 handler が返る",
			setup: func(r *outboxusecase.HandlerRegistry) (*stubHandler, string) {
				h := &stubHandler{}
				r.Register("photobook.published", h)
				return h, "photobook.published"
			},
			wantFound:  true,
			wantHandle: true,
		},
		{
			name:        "異常_未登録 key は lookup 失敗",
			description: "Given: 何も登録していない / When: Lookup('unknown.event') / Then: ok=false",
			setup: func(r *outboxusecase.HandlerRegistry) (*stubHandler, string) {
				return nil, "unknown.event"
			},
			wantFound:  false,
			wantHandle: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := outboxusecase.NewHandlerRegistry()
			expectHandler, key := tt.setup(r)

			h, ok := r.Lookup(key)
			if ok != tt.wantFound {
				t.Fatalf("Lookup ok=%v want %v", ok, tt.wantFound)
			}
			if !tt.wantFound {
				return
			}
			if err := h.Handle(context.Background(), outboxusecase.EventTarget{ID: uuid.New()}); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if !expectHandler.called {
				t.Errorf("expected handler not called")
			}
		})
	}
}

func TestHandlerRegistryDoubleRegisterPanics(t *testing.T) {
	r := outboxusecase.NewHandlerRegistry()
	r.Register("photobook.published", &stubHandler{})
	defer func() {
		if rec := recover(); rec == nil {
			t.Errorf("expected panic on double Register, got nil")
		}
	}()
	r.Register("photobook.published", &stubHandler{})
}

func TestErrUnknownEventTypeIsExported(t *testing.T) {
	// 簡単な存在確認: errors.Is で比較できる sentinel error であること
	if !errors.Is(outboxusecase.ErrUnknownEventType, outboxusecase.ErrUnknownEventType) {
		t.Errorf("ErrUnknownEventType should be a comparable sentinel")
	}
}
