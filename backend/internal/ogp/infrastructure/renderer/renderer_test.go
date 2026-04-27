package renderer_test

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"vrcpb/backend/internal/ogp/infrastructure/renderer"
)

func newRendererForTest(t *testing.T) *renderer.Renderer {
	t.Helper()
	r, err := renderer.New()
	if err != nil {
		t.Fatalf("renderer.New: %v", err)
	}
	return r
}

func decodePNG(t *testing.T, b []byte) (w, h int) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	return img.Bounds().Dx(), img.Bounds().Dy()
}

func TestRender_TableDriven(t *testing.T) {
	r := newRendererForTest(t)
	tests := []struct {
		name        string
		description string
		input       renderer.Input
	}{
		{
			name:        "正常_短いASCIItitle",
			description: "Given: short ascii title, When: render, Then: 1200x630 PNG",
			input: renderer.Input{
				Title: "Hello PhotoBook", TypeLabel: "memory",
				CreatorDisplayName: "alice",
			},
		},
		{
			name:        "正常_日本語title",
			description: "Given: 日本語 title, When: render, Then: 成功",
			input: renderer.Input{
				Title: "VRChat の素敵な思い出フォトブック", TypeLabel: "memory",
				CreatorDisplayName: "ありす",
			},
		},
		{
			name:        "異常_長文title_折返し+省略",
			description: "Given: 80字の長文, When: render, Then: panic せず PNG 出力",
			input: renderer.Input{
				Title: strings.Repeat("あ", 80), TypeLabel: "event",
				CreatorDisplayName: "ボブ",
			},
		},
		{
			name:        "正常_creator長文",
			description: "Given: 50字 creator, When: render, Then: 成功",
			input: renderer.Input{
				Title: "Title", TypeLabel: "world",
				CreatorDisplayName: strings.Repeat("a", 50),
			},
		},
		{
			name:        "正常_emojiは欠落許容",
			description: "Given: emoji含み, When: render, Then: panic せず PNG",
			input: renderer.Input{
				Title: "Hello 🎉 World", TypeLabel: "memory",
				CreatorDisplayName: "🐱 cat",
			},
		},
		{
			name:        "正常_coverPNG_nilでfallback描画",
			description: "Given: cover nil, When: render, Then: 成功",
			input: renderer.Input{
				Title: "no cover", TypeLabel: "memory",
				CreatorDisplayName: "alice",
				CoverPNG:           nil,
			},
		},
		{
			name:        "正常_coverPNG_decode失敗でもfallback",
			description: "Given: invalid bytes, When: render, Then: 成功（fallback 描画）",
			input: renderer.Input{
				Title: "broken cover", TypeLabel: "memory",
				CreatorDisplayName: "alice",
				CoverPNG:           []byte{0x00, 0x01, 0x02},
			},
		},
		{
			name:        "正常_空title",
			description: "Given: 空 title, When: render, Then: 成功",
			input: renderer.Input{
				Title: "", TypeLabel: "", CreatorDisplayName: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := r.Render(tt.input)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if res.Width != 1200 || res.Height != 630 {
				t.Errorf("size mismatch: %dx%d", res.Width, res.Height)
			}
			w, h := decodePNG(t, res.Bytes)
			if w != 1200 || h != 630 {
				t.Errorf("decoded size mismatch: %dx%d", w, h)
			}
			if len(res.Bytes) < 1024 {
				t.Errorf("png bytes too small: %d", len(res.Bytes))
			}
		})
	}
}

// TestRender_NoSecretInBytes は PNG バイナリに input から渡した文字列以外の
// 余計な metadata（例: storage_key / token / Cookie）が出ないことを検証する簡易テスト。
//
// PNG バイナリ自体は文字列を含まないので、本テストは render が input 文字列を
// テキストチャンク等に書き出していないことのみ確認する（Go image/png の
// 標準 encoder は tEXt チャンクを書き出さないため、ここでは ASCII 列としての
// 危険語のみ確認）。
func TestRender_NoSecretInBytes(t *testing.T) {
	r := newRendererForTest(t)
	res, err := r.Render(renderer.Input{
		Title: "Title", TypeLabel: "memory", CreatorDisplayName: "alice",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, banned := range []string{
		"DATABASE_URL", "Bearer ", "Set-Cookie", "presigned",
		"manage_url_token", "draft_edit_token", "session_token",
	} {
		if bytes.Contains(res.Bytes, []byte(banned)) {
			t.Errorf("banned token leaked into PNG bytes: %q", banned)
		}
	}
}
