package caption_test

import (
	"errors"
	"strings"
	"testing"

	"vrcpb/backend/internal/photobook/domain/vo/caption"
)

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "正常_空文字", input: ""},
		{name: "正常_短い文字列", input: "hello"},
		{name: "正常_境界200rune", input: strings.Repeat("あ", 200)},
		{name: "正常_改行tab許容", input: "first\nsecond\tthird"},
		{name: "異常_201rune", input: strings.Repeat("あ", 201), wantErr: caption.ErrTooLong},
		{name: "異常_NUL文字", input: "abc\x00def", wantErr: caption.ErrControlCharNotAllowed},
		{name: "異常_BEL文字", input: "abc\x07def", wantErr: caption.ErrControlCharNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := caption.New(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	t.Parallel()
	a, _ := caption.New("hello")
	b, _ := caption.New("hello")
	c, _ := caption.New("world")
	if !a.Equal(b) {
		t.Errorf("equal expected")
	}
	if a.Equal(c) {
		t.Errorf("not equal expected")
	}
}
