package byte_size_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/image/domain/vo/byte_size"
)

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		description string
		input       int64
		wantErr     bool
	}{
		{name: "正常_1byte", description: "Given: 1, Then: 成功", input: 1},
		{name: "正常_10MB", description: "Given: 10485760, Then: 成功（境界）", input: 10 * 1024 * 1024},
		{name: "異常_0", description: "Given: 0, Then: error", input: 0, wantErr: true},
		{name: "異常_負値", description: "Given: -1, Then: error", input: -1, wantErr: true},
		{name: "異常_10MB+1", description: "Given: 10485761, Then: error", input: 10*1024*1024 + 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := byte_size.New(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, byte_size.ErrByteSizeOutOfRange) {
					t.Errorf("err = %v", err)
				}
				return
			}
			if got.Int64() != tt.input {
				t.Errorf("got %d want %d", got.Int64(), tt.input)
			}
		})
	}
}

func TestMaxBytes(t *testing.T) {
	t.Parallel()
	if byte_size.MaxBytes() != 10*1024*1024 {
		t.Errorf("MaxBytes = %d", byte_size.MaxBytes())
	}
}
