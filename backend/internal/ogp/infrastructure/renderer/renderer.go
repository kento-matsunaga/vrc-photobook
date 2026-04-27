// Package renderer は OGP 画像（1200×630 PNG）を生成する。
//
// 設計参照:
//   - docs/plan/m2-ogp-generation-plan.md §3 / §4 採用案 A（Go image/draw + freetype）
//   - docs/design/cross-cutting/ogp-generation.md §6
//
// 表示要素:
//   - cover thumbnail（左、480×480 中央切抜）/ cover が無ければ fallback ブランド面
//   - title（右上、Bold、最大 2 行で折返し、3 行目以降は ... に切る）
//   - type badge（タイトル下）
//   - creator（type の下、Regular）
//   - service wordmark（footer 中央、Bold）
//
// セキュリティ:
//   - 入力に管理 URL / token / hash / storage_key を含めない（呼び出し側で除外）
//   - panic / フォント load 失敗は recover して error として返す（renderer の panic で
//     呼び出し側が落ちないようにする）
package renderer

import (
	_ "embed"
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	canvasW = 1200
	canvasH = 630

	coverSize = 460 // cover thumbnail 460x460（描画位置 padding 分余裕）
	coverX    = 60
	coverY    = (canvasH - coverSize) / 2

	textX     = coverX + coverSize + 60 // 580
	titleY    = 110
	maxTitleW = canvasW - textX - 60 // ~560

	titleSize        = 56
	titleLineHeight  = 70
	maxTitleLines    = 2
	bodySize         = 28
	wordmarkSize     = 32
	bottomMargin     = 50
)

//go:embed fonts/NotoSansJP-Regular.otf
var notoRegular []byte

//go:embed fonts/NotoSansJP-Bold.otf
var notoBold []byte

// Input は renderer に渡すデータ（公開可能な情報のみ）。
//
// CoverPNG が nil なら fallback デザイン（cover 領域はブランド色のフラット）に切替。
type Input struct {
	Title              string // 80 char 制限内（domain VO で保証済）
	TypeLabel          string // 例: "memory" / "event" / "world"（日本語ラベルでも可）
	CreatorDisplayName string
	CoverPNG           []byte // 任意。decode 失敗 / nil → fallback
}

// Result は生成結果。Bytes は PNG。
type Result struct {
	Bytes  []byte
	Width  int
	Height int
}

// Renderer は再利用可能な OGP 画像 renderer。
//
// font face は cold start に時間がかかるため一度だけ load する。Renderer 自体は
// stateless なので複数 worker が共有して良い。
type Renderer struct {
	titleFace    font.Face
	bodyFace     font.Face
	wordmarkFace font.Face
}

// New は Renderer を組み立てる。フォント load 失敗は致命的エラーで返す。
func New() (*Renderer, error) {
	titleFace, err := newFace(notoBold, titleSize)
	if err != nil {
		return nil, fmt.Errorf("title face: %w", err)
	}
	bodyFace, err := newFace(notoRegular, bodySize)
	if err != nil {
		return nil, fmt.Errorf("body face: %w", err)
	}
	wordmarkFace, err := newFace(notoBold, wordmarkSize)
	if err != nil {
		return nil, fmt.Errorf("wordmark face: %w", err)
	}
	return &Renderer{
		titleFace:    titleFace,
		bodyFace:     bodyFace,
		wordmarkFace: wordmarkFace,
	}, nil
}

func newFace(raw []byte, size float64) (font.Face, error) {
	f, err := opentype.Parse(raw)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// Render は 1200×630 PNG を生成する。panic は recover して error に変換する。
func (r *Renderer) Render(in Input) (result Result, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("renderer panic: %v", rec)
			result = Result{}
		}
	}()

	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	bg := color.RGBA{R: 0xF7, G: 0xF9, B: 0xFA, A: 0xFF} // surface-soft 相当
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	r.drawCover(canvas, in.CoverPNG)
	r.drawTitle(canvas, in.Title)
	r.drawTypeAndCreator(canvas, in.TypeLabel, in.CreatorDisplayName)
	r.drawWordmark(canvas)

	var buf bytes.Buffer
	enc := &png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, canvas); err != nil {
		return Result{}, fmt.Errorf("png encode: %w", err)
	}
	return Result{Bytes: buf.Bytes(), Width: canvasW, Height: canvasH}, nil
}

// drawCover は左側の cover thumbnail を描画する。
//
// CoverPNG が nil / decode 失敗時は fallback 色面（brand teal のフラット）。
func (r *Renderer) drawCover(canvas draw.Image, coverPNG []byte) {
	rect := image.Rect(coverX, coverY, coverX+coverSize, coverY+coverSize)
	if len(coverPNG) > 0 {
		src, err := png.Decode(bytes.NewReader(coverPNG))
		if err == nil {
			thumb := imaging.Fill(src, coverSize, coverSize, imaging.Center, imaging.Lanczos)
			draw.Draw(canvas, rect, thumb, image.Point{}, draw.Src)
			return
		}
	}
	// fallback: brand teal フラット
	teal := color.RGBA{R: 0x2C, G: 0xA0, B: 0x9D, A: 0xFF}
	draw.Draw(canvas, rect, &image.Uniform{C: teal}, image.Point{}, draw.Src)
}

// drawTitle は title を 2 行までで折返す。3 行目以降は省略記号で切る。
func (r *Renderer) drawTitle(canvas draw.Image, title string) {
	col := color.RGBA{R: 0x10, G: 0x16, B: 0x1B, A: 0xFF} // ink
	lines := wrapTextByWidth(title, r.titleFace, maxTitleW, maxTitleLines)
	for i, line := range lines {
		drawString(canvas, r.titleFace, col, textX, titleY+i*titleLineHeight, line)
	}
}

// drawTypeAndCreator は title 下の補助情報を描く。
func (r *Renderer) drawTypeAndCreator(canvas draw.Image, typeLabel, creator string) {
	colMid := color.RGBA{R: 0x60, G: 0x70, B: 0x7C, A: 0xFF} // ink-medium
	colSoft := color.RGBA{R: 0x94, G: 0xA3, B: 0xB8, A: 0xFF} // ink-soft

	yType := titleY + maxTitleLines*titleLineHeight + 30
	if typeLabel != "" {
		drawString(canvas, r.bodyFace, colSoft, textX, yType, "Type: "+typeLabel)
	}

	yCreator := yType + 50
	if creator != "" {
		// 50 char 制限は domain VO 側で保証済。fallback で 60 文字 truncate（防衛）。
		if l := []rune(creator); len(l) > 60 {
			creator = string(l[:60])
		}
		drawString(canvas, r.bodyFace, colMid, textX, yCreator, "by "+creator)
	}
}

// drawWordmark は footer 中央にサービス名を描画する。
func (r *Renderer) drawWordmark(canvas draw.Image) {
	col := color.RGBA{R: 0x2C, G: 0xA0, B: 0x9D, A: 0xFF} // brand-teal
	const wordmark = "VRC PhotoBook"
	w := stringWidth(r.wordmarkFace, wordmark)
	x := (canvasW - w) / 2
	y := canvasH - bottomMargin
	drawString(canvas, r.wordmarkFace, col, x, y, wordmark)
}

// wrapTextByWidth は与えた face / max width で文字列を行分割する。
//
// 日本語の場合は文字単位（rune）で分割する（簡易実装、word boundary は考慮しない）。
// max lines を超える分は最終行末尾に … を付けて切り捨てる。
func wrapTextByWidth(s string, face font.Face, maxW int, maxLines int) []string {
	if s == "" {
		return nil
	}
	out := make([]string, 0, maxLines)
	var current strings.Builder
	for _, r := range s {
		probe := current.String() + string(r)
		if stringWidth(face, probe) > maxW && current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
			if len(out) >= maxLines {
				break
			}
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 && len(out) < maxLines {
		out = append(out, current.String())
	}
	if len(out) == maxLines {
		// 残りがあれば末尾を ... で打ち切る
		consumed := 0
		for _, line := range out {
			consumed += len([]rune(line))
		}
		if consumed < len([]rune(s)) {
			last := out[maxLines-1]
			// 末尾を…に置換できるところまで切る
			for stringWidth(face, last+"…") > maxW && len([]rune(last)) > 1 {
				rs := []rune(last)
				last = string(rs[:len(rs)-1])
			}
			out[maxLines-1] = last + "…"
		}
	}
	return out
}

// drawString は指定座標に文字列を 1 行描画する。
func drawString(canvas draw.Image, face font.Face, col color.Color, x, y int, s string) {
	if s == "" {
		return
	}
	d := &font.Drawer{
		Dst:  canvas,
		Src:  &image.Uniform{C: col},
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

// stringWidth は face で render したときの文字列幅（px）を返す。
func stringWidth(face font.Face, s string) int {
	if s == "" {
		return 0
	}
	d := &font.Drawer{Face: face}
	w := d.MeasureString(s)
	return w.Round()
}

// ErrFontLoad は外部 test から判定できるよう公開しておく（現時点で main code では未使用）。
var ErrFontLoad = errors.New("renderer: font load failed")
