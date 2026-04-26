package display_order_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/photobook/domain/vo/display_order"
)

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   int
		wantErr bool
	}{
		{name: "正常_0", input: 0},
		{name: "正常_29", input: 29},
		{name: "正常_100", input: 100},
		{name: "異常_負値", input: -1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := display_order.New(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, display_order.ErrNegativeDisplayOrder) {
					t.Errorf("err = %v", err)
				}
				return
			}
			if got.Int() != tt.input {
				t.Errorf("got %d want %d", got.Int(), tt.input)
			}
		})
	}
}

func TestZero(t *testing.T) {
	t.Parallel()
	if display_order.Zero().Int() != 0 {
		t.Errorf("Zero must be 0")
	}
}
