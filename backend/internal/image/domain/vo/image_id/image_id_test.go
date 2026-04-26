package image_id_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"vrcpb/backend/internal/image/domain/vo/image_id"
)

func TestNewAndUUIDv7(t *testing.T) {
	t.Parallel()
	id, err := image_id.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if id.UUID() == uuid.Nil {
		t.Fatalf("nil uuid")
	}
	if v := id.UUID().Version(); v != 7 {
		t.Errorf("uuid version = %d, want 7", v)
	}
}

func TestFromUUID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		description string
		input       uuid.UUID
		wantErr     bool
	}{
		{
			name:        "正常_有効なUUID",
			description: "Given: 非nil UUID, When: FromUUID, Then: 成功",
			input:       uuid.New(),
		},
		{
			name:        "異常_nil_UUID",
			description: "Given: uuid.Nil, When: FromUUID, Then: ErrInvalidImageID",
			input:       uuid.Nil,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := image_id.FromUUID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, image_id.ErrInvalidImageID) {
				t.Errorf("err = %v, want ErrInvalidImageID", err)
			}
		})
	}
}

func TestEqualAndString(t *testing.T) {
	t.Parallel()
	id1, _ := image_id.New()
	id2, _ := image_id.FromUUID(id1.UUID())
	id3, _ := image_id.New()
	if !id1.Equal(id2) {
		t.Errorf("equal expected")
	}
	if id1.Equal(id3) {
		t.Errorf("not equal expected")
	}
	if id1.String() == "" {
		t.Errorf("String() should not be empty")
	}
}
