package action_id_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"vrcpb/backend/internal/moderation/domain/vo/action_id"
)

func TestNewIsUUIDv7(t *testing.T) {
	id, err := action_id.New()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if id.IsZero() {
		t.Error("New() should not be zero")
	}
	if id.UUID().Version() != 7 {
		t.Errorf("New() should produce UUIDv7, got version=%d", id.UUID().Version())
	}
}

func TestFromUUID(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          uuid.UUID
		wantErr     bool
	}{
		{
			name:        "正常_非nil_UUID",
			description: "Given: 有効 UUID, When: FromUUID, Then: ok",
			in:          uuid.MustParse("019dd1bb-774f-7341-91a4-fd0fbd279320"),
		},
		{
			name:        "異常_nil_UUID",
			description: "Given: nil UUID, When: FromUUID, Then: ErrInvalidActionID",
			in:          uuid.Nil,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := action_id.FromUUID(tt.in)
			if tt.wantErr {
				if !errors.Is(err, action_id.ErrInvalidActionID) {
					t.Errorf("expected ErrInvalidActionID, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.UUID() != tt.in {
				t.Errorf("got %v want %v", got.UUID(), tt.in)
			}
		})
	}
}

func TestEqualAndIsZero(t *testing.T) {
	a := action_id.MustParse("019dd1bb-774f-7341-91a4-fd0fbd279320")
	b := action_id.MustParse("019dd1bb-774f-7341-91a4-fd0fbd279320")
	c := action_id.MustParse("019dd1bb-77ce-799d-a349-0a01479098f2")
	if !a.Equal(b) {
		t.Error("equal UUIDs should be Equal")
	}
	if a.Equal(c) {
		t.Error("different UUIDs should NOT be Equal")
	}
	var zero action_id.ActionID
	if !zero.IsZero() {
		t.Error("zero value should IsZero=true")
	}
	if a.IsZero() {
		t.Error("non-zero ActionID IsZero should be false")
	}
}
