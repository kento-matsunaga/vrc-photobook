package session_id_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		runs        int
	}{
		{
			name:        "正常_衝突なし1000回",
			description: "Given: なし, When: New を 1000 回, Then: 値が UUID として valid かつ衝突しない",
			runs:        1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(map[uuid.UUID]struct{}, tt.runs)
			for i := 0; i < tt.runs; i++ {
				id, err := session_id.New()
				if err != nil {
					t.Fatalf("New: %v", err)
				}
				u := id.UUID()
				if u == uuid.Nil {
					t.Fatalf("nil UUID")
				}
				if _, dup := seen[u]; dup {
					t.Fatalf("duplicate at run %d", i)
				}
				seen[u] = struct{}{}
			}
		})
	}
}

func TestFromUUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		input       uuid.UUID
		wantErr     error
	}{
		{
			name:        "正常_有効なUUID",
			description: "Given: NewV7 の値, When: FromUUID, Then: エラーなし",
			input:       func() uuid.UUID { v, _ := uuid.NewV7(); return v }(),
		},
		{
			name:        "異常_nil UUID",
			description: "Given: uuid.Nil, When: FromUUID, Then: ErrInvalidSessionID",
			input:       uuid.Nil,
			wantErr:     session_id.ErrInvalidSessionID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := session_id.FromUUID(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !id.Equal(id) {
				t.Fatalf("self-equal must be true")
			}
			if id.UUID() != tt.input {
				t.Errorf("UUID mismatch")
			}
		})
	}
}
