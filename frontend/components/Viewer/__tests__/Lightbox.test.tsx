// Lightbox.tsx の SSR 構造検証。
//
// 観点:
//   - isOpen=false → null (DOM 描画なし)
//   - isOpen=true + photos.length>0 → role="dialog" + close button + thumb strip
//   - photos.length===0 → 安全に null
//   - activeIndex がカウンター "n / m" に反映
//
// 注意:
//   - vitest は environment: "node" 設定のため、pointer event / Esc keydown / click は
//     jsdom 必要で本ファイルでは未対応。状態遷移と DOM 構造のみ assert
//   - pinch zoom 等 gesture は実機 (Safari) で確認 (落とし穴 #5 認知)

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { Lightbox } from "@/components/Viewer/Lightbox";
import type { PublicPhoto } from "@/lib/publicPhotobook";

function dummyPhoto(slug: string): PublicPhoto {
  return {
    caption: undefined,
    variants: {
      display: {
        url: `https://images.example.invalid/${slug}.jpg`,
        width: 1200,
        height: 1600,
        expiresAt: "2099-12-31T23:59:59Z",
      },
      thumbnail: {
        url: `https://images.example.invalid/${slug}.thumb.jpg`,
        width: 300,
        height: 400,
        expiresAt: "2099-12-31T23:59:59Z",
      },
    },
  };
}

describe("Lightbox", () => {
  const noop = () => {
    /* noop */
  };

  it("正常_isOpen_false_は_null_でDOM出力なし", () => {
    const html = renderToStaticMarkup(
      <Lightbox
        photos={[dummyPhoto("a")]}
        isOpen={false}
        activeIndex={0}
        onClose={noop}
        onSelect={noop}
      />,
    );
    expect(html).toBe("");
  });

  it("正常_photos_空配列_は_null", () => {
    const html = renderToStaticMarkup(
      <Lightbox
        photos={[]}
        isOpen={true}
        activeIndex={0}
        onClose={noop}
        onSelect={noop}
      />,
    );
    expect(html).toBe("");
  });

  it("正常_isOpen_true_+_photos_あり_は_dialog_+_close_button_+_thumbs_を描画", () => {
    const photos = [dummyPhoto("a"), dummyPhoto("b"), dummyPhoto("c")];
    const html = renderToStaticMarkup(
      <Lightbox
        photos={photos}
        isOpen={true}
        activeIndex={1}
        onClose={noop}
        onSelect={noop}
      />,
    );
    expect(html).toContain('role="dialog"');
    expect(html).toContain('aria-modal="true"');
    expect(html).toContain('data-testid="lightbox"');
    expect(html).toContain('data-testid="lightbox-close"');
    expect(html).toContain('data-testid="lightbox-thumbs"');
    expect(html).toContain('data-testid="lightbox-thumb-0"');
    expect(html).toContain('data-testid="lightbox-thumb-1"');
    expect(html).toContain('data-testid="lightbox-thumb-2"');
    // counter "2 / 3" (activeIndex=1 → 1+1=2)
    expect(html).toContain("2 / 3");
  });

  it("正常_photos_1枚_の場合_thumb_strip_と_prev_next_を出さない", () => {
    const html = renderToStaticMarkup(
      <Lightbox
        photos={[dummyPhoto("a")]}
        isOpen={true}
        activeIndex={0}
        onClose={noop}
        onSelect={noop}
      />,
    );
    expect(html).toContain('data-testid="lightbox"');
    expect(html).not.toContain('data-testid="lightbox-thumbs"');
    expect(html).not.toContain('data-testid="lightbox-prev"');
    expect(html).not.toContain('data-testid="lightbox-next"');
  });

  it("正常_activeIndex_範囲外_は_clamp_され安全に描画", () => {
    const photos = [dummyPhoto("a"), dummyPhoto("b")];
    const html = renderToStaticMarkup(
      <Lightbox
        photos={photos}
        isOpen={true}
        activeIndex={99}
        onClose={noop}
        onSelect={noop}
      />,
    );
    // clamp で b (index 1) が表示、counter は "2 / 2"
    expect(html).toContain("2 / 2");
    expect(html).toContain("images.example.invalid/b.jpg");
  });
});
