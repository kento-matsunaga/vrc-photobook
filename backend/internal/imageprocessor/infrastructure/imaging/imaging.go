// Package imaging は image-processor の画像変換実装。
//
// 設計参照:
//   - docs/plan/m2-image-processor-plan.md §6 / §7 / §10 / §14
//
// 入力（io.Reader）→ decode → resize → JPEG encode（display + thumbnail）。
// JPEG 再エンコードにより APP1 / EXIF / XMP / IPTC / ICC は自動的に除去される。
//
// 依存:
//   - image/jpeg, image/png（Go 標準）
//   - golang.org/x/image/webp（decode 専用）
//   - github.com/disintegration/imaging（Lanczos resize）
//
// セキュリティ:
//   - decode 失敗 / format 不一致は明示エラーで返す（呼び出し側で failure_reason に変換）
//   - 元画像の長辺が target 未満のときは拡大しない（plan §7.1）
//   - encode 出力に APP1 / EXIF marker が残っていないことを test で検証する
package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // image.Decode に png を登録
	"io"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // image.Decode に webp を登録
)

// エラー。
var (
	ErrDecodeFailed         = errors.New("image decode failed")
	ErrEncodeFailed         = errors.New("image encode failed")
	ErrUnsupportedFormat    = errors.New("unsupported image format")
)

// 仕様値（plan §6.2 / §7.1）。
const (
	DisplayLongSide   = 1600
	ThumbnailLongSide = 480
	DisplayQuality    = 85
	ThumbnailQuality  = 80
)

// SourceFormat は decode 対象として許容する形式。
type SourceFormat string

const (
	SourceJPEG SourceFormat = "jpg"
	SourcePNG  SourceFormat = "png"
	SourceWebP SourceFormat = "webp"
)

// DecodedImage は decode 結果。寸法は集約に渡す（VO 側で範囲制約を再検証）。
type DecodedImage struct {
	Image  image.Image
	Width  int
	Height int
}

// Decode は io.Reader から画像を decode する。
//
// `image.Decode` は image/jpeg / image/png / image/webp（_ import 経由）を判別する。
// SourceFormat は呼び出し側の期待値（DB の source_format）。
// decode 自体は format 自動判別だが、想定外の形式（svg / heic 等）はここで弾く。
func Decode(r io.Reader, expected SourceFormat) (DecodedImage, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return DecodedImage{}, fmt.Errorf("%w: %v", ErrDecodeFailed, err)
	}
	if !matchesFormat(format, expected) {
		return DecodedImage{}, fmt.Errorf("%w: decoded=%s expected=%s",
			ErrUnsupportedFormat, format, expected)
	}
	b := img.Bounds()
	return DecodedImage{Image: img, Width: b.Dx(), Height: b.Dy()}, nil
}

// matchesFormat は image.Decode の format 名（"jpeg" / "png" / "webp"）と SourceFormat を比較。
func matchesFormat(decoded string, expected SourceFormat) bool {
	switch expected {
	case SourceJPEG:
		return decoded == "jpeg"
	case SourcePNG:
		return decoded == "png"
	case SourceWebP:
		return decoded == "webp"
	default:
		return false
	}
}

// EncodedVariant は resize + encode 結果。
type EncodedVariant struct {
	Body   []byte
	Width  int
	Height int
}

// EncodeJPEG は src を長辺 longSide に Lanczos リサイズし、quality で JPEG エンコードする。
//
// 元画像が longSide 以下のときは拡大せず元サイズを維持する（plan §7.1）。
func EncodeJPEG(src image.Image, longSide, quality int) (EncodedVariant, error) {
	resized := resizeIfLarger(src, longSide)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: quality}); err != nil {
		return EncodedVariant{}, fmt.Errorf("%w: %v", ErrEncodeFailed, err)
	}
	b := resized.Bounds()
	return EncodedVariant{
		Body:   buf.Bytes(),
		Width:  b.Dx(),
		Height: b.Dy(),
	}, nil
}

// resizeIfLarger は長辺が longSide を超えている場合のみ Lanczos リサイズする。
func resizeIfLarger(src image.Image, longSide int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= longSide && h <= longSide {
		return src
	}
	if w >= h {
		return imaging.Resize(src, longSide, 0, imaging.Lanczos)
	}
	return imaging.Resize(src, 0, longSide, imaging.Lanczos)
}

