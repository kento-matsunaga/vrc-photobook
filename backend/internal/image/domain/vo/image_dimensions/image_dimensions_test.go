package image_dimensions_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
)

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		description string
		w, h        int
		wantErr     error
	}{
		{name: "正常_1x1", description: "Given: 最小有効値, Then: 成功", w: 1, h: 1},
		{name: "正常_8000x5000_40MP丁度", description: "Given: 8000x5000=40MP(境界), Then: 成功", w: 8000, h: 5000},
		{name: "正常_8192x4000_32.7MP", description: "Given: 8192x4000, Then: 成功", w: 8192, h: 4000},
		{name: "異常_8000x5001_40MP超過", description: "Given: 8000x5001=40,008,000, Then: ErrPixelsExceedLimit", w: 8000, h: 5001, wantErr: image_dimensions.ErrPixelsExceedLimit},
		{name: "異常_8192x8192_67MP超過", description: "Given: 8192x8192, Then: ErrPixelsExceedLimit", w: 8192, h: 8192, wantErr: image_dimensions.ErrPixelsExceedLimit},
		{name: "異常_width_0", description: "Given: width=0, Then: ErrDimensionOutOfRange", w: 0, h: 100, wantErr: image_dimensions.ErrDimensionOutOfRange},
		{name: "異常_width_8193", description: "Given: width=8193, Then: ErrDimensionOutOfRange", w: 8193, h: 100, wantErr: image_dimensions.ErrDimensionOutOfRange},
		{name: "異常_height_0", description: "Given: height=0, Then: ErrDimensionOutOfRange", w: 100, h: 0, wantErr: image_dimensions.ErrDimensionOutOfRange},
		{name: "異常_height_8193", description: "Given: height=8193, Then: ErrDimensionOutOfRange", w: 100, h: 8193, wantErr: image_dimensions.ErrDimensionOutOfRange},
		{name: "異常_負値", description: "Given: width=-1, Then: ErrDimensionOutOfRange", w: -1, h: 100, wantErr: image_dimensions.ErrDimensionOutOfRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := image_dimensions.New(tt.w, tt.h)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got.Width() != tt.w || got.Height() != tt.h {
				t.Errorf("got %dx%d want %dx%d", got.Width(), got.Height(), tt.w, tt.h)
			}
		})
	}
}

func TestEqualAndPixels(t *testing.T) {
	t.Parallel()
	a, _ := image_dimensions.New(100, 50)
	b, _ := image_dimensions.New(100, 50)
	c, _ := image_dimensions.New(100, 51)
	if !a.Equal(b) {
		t.Errorf("equal expected")
	}
	if a.Equal(c) {
		t.Errorf("not equal expected")
	}
	if a.Pixels() != 5000 {
		t.Errorf("pixels = %d, want 5000", a.Pixels())
	}
}
