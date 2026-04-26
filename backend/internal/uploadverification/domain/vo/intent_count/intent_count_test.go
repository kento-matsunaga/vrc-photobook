package intent_count_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/uploadverification/domain/vo/intent_count"
)

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   int
		wantErr bool
	}{
		{name: "正常_0", input: 0},
		{name: "正常_20", input: 20},
		{name: "正常_100", input: 100},
		{name: "異常_負値", input: -1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := intent_count.New(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, intent_count.ErrNegativeCount) {
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

func TestZeroAndDefault(t *testing.T) {
	t.Parallel()
	if intent_count.Zero().Int() != 0 {
		t.Errorf("Zero must be 0")
	}
	if intent_count.Default().Int() != 20 {
		t.Errorf("Default must be 20")
	}
}

func TestIncrement(t *testing.T) {
	t.Parallel()
	c := intent_count.MustNew(5)
	if c.Increment().Int() != 6 {
		t.Errorf("Increment failed")
	}
	if c.Int() != 5 {
		t.Errorf("original must be unchanged")
	}
}
