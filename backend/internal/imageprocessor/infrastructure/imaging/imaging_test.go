// imaging package のテーブル駆動テスト。
//
// 観点:
//   - Decode: jpg / png / webp 正常 + 不正バイトでエラー + format 不一致でエラー
//   - EncodeJPEG: 縮小 / そのまま / 拡大なし
//   - EncodeJPEG 出力に APP1（0xFF 0xE1） / "Exif" マーカーが含まれない（plan §6.2）
package imaging_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"vrcpb/backend/internal/imageprocessor/infrastructure/imaging"
)

func newRGBA(t *testing.T, w, h int) image.Image {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0x80, A: 0xFF})
		}
	}
	return img
}

func encodeJPEGBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}

func encodePNGBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestDecode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		description string
		body        func(*testing.T) []byte
		expected    imaging.SourceFormat
		wantErrIs   error
	}{
		{
			name:        "正常_jpeg",
			description: "Given: valid JPEG, When: Decode(JPEG), Then: 成功",
			body:        func(t *testing.T) []byte { return encodeJPEGBytes(t, newRGBA(t, 64, 32)) },
			expected:    imaging.SourceJPEG,
		},
		{
			name:        "正常_png",
			description: "Given: valid PNG, When: Decode(PNG), Then: 成功",
			body:        func(t *testing.T) []byte { return encodePNGBytes(t, newRGBA(t, 16, 16)) },
			expected:    imaging.SourcePNG,
		},
		{
			name:        "異常_format不一致",
			description: "Given: PNG body, When: Decode 期待=JPEG, Then: ErrUnsupportedFormat",
			body:        func(t *testing.T) []byte { return encodePNGBytes(t, newRGBA(t, 16, 16)) },
			expected:    imaging.SourceJPEG,
			wantErrIs:   imaging.ErrUnsupportedFormat,
		},
		{
			name:        "異常_decode失敗",
			description: "Given: 壊れたバイト, When: Decode, Then: ErrDecodeFailed",
			body:        func(t *testing.T) []byte { return []byte("not an image") },
			expected:    imaging.SourceJPEG,
			wantErrIs:   imaging.ErrDecodeFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := tt.body(t)
			got, err := imaging.Decode(bytes.NewReader(body), tt.expected)
			if tt.wantErrIs != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErrIs)
				}
				if !errorsIs(err, tt.wantErrIs) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.Image == nil {
				t.Fatal("Image is nil")
			}
			if got.Width <= 0 || got.Height <= 0 {
				t.Errorf("dims w=%d h=%d", got.Width, got.Height)
			}
		})
	}
}

func TestEncodeJPEG_Resize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		description string
		srcW, srcH  int
		longSide    int
		wantW       int
		wantH       int
	}{
		{
			name:        "縮小_横長",
			description: "Given: 3200x1600, When: EncodeJPEG(longSide=1600), Then: 1600x800",
			srcW:        3200, srcH: 1600, longSide: 1600,
			wantW: 1600, wantH: 800,
		},
		{
			name:        "縮小_縦長",
			description: "Given: 1000x2000, When: EncodeJPEG(longSide=480), Then: 240x480",
			srcW:        1000, srcH: 2000, longSide: 480,
			wantW: 240, wantH: 480,
		},
		{
			name:        "拡大なし",
			description: "Given: 100x100, When: EncodeJPEG(longSide=1600), Then: 100x100 のまま",
			srcW:        100, srcH: 100, longSide: 1600,
			wantW: 100, wantH: 100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := imaging.EncodeJPEG(newRGBA(t, tt.srcW, tt.srcH), tt.longSide, 80)
			if err != nil {
				t.Fatalf("EncodeJPEG: %v", err)
			}
			if out.Width != tt.wantW || out.Height != tt.wantH {
				t.Errorf("dims = %dx%d, want %dx%d", out.Width, out.Height, tt.wantW, tt.wantH)
			}
			if len(out.Body) == 0 {
				t.Error("empty body")
			}
		})
	}
}

// TestEncodeJPEG_NoEXIFMarker は plan §6.2 受け入れ条件:
// JPEG 再エンコードで APP1 / "Exif" / "XMP" マーカーが残らない。
//
// Go 標準 image/jpeg の encoder は APP0 (JFIF) のみを書き出し、APP1 (EXIF) は出さない。
func TestEncodeJPEG_NoEXIFMarker(t *testing.T) {
	t.Parallel()
	out, err := imaging.EncodeJPEG(newRGBA(t, 200, 200), 1600, 85)
	if err != nil {
		t.Fatalf("EncodeJPEG: %v", err)
	}
	body := out.Body
	// APP1 marker = 0xFF 0xE1
	if bytes.Contains(body, []byte{0xFF, 0xE1}) {
		t.Error("APP1 (EXIF) marker found in JPEG output")
	}
	if bytes.Contains(body, []byte("Exif\x00\x00")) {
		t.Error("Exif identifier found in JPEG output")
	}
	if bytes.Contains(body, []byte("http://ns.adobe.com/xap/1.0/")) {
		t.Error("XMP namespace found in JPEG output")
	}
	// APP13 (IPTC/Photoshop) = 0xFF 0xED
	if bytes.Contains(body, []byte{0xFF, 0xED}) {
		t.Error("APP13 (IPTC) marker found in JPEG output")
	}
}

// errorsIs は errors.Is の薄いラッパ（test 内 import を簡潔にするため）。
func errorsIs(err, target error) bool {
	for {
		if err == target {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
		if err == nil {
			return false
		}
	}
}
