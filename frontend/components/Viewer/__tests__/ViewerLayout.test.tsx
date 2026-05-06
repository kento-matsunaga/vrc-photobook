// ViewerLayout.tsx の SSR レンダリング検証 (v2 redesign)。
//
// 観点:
//   - flat photos / pageBases 計算が正しく PageHero に伝播する (落とし穴 #4)
//   - 必須セクション: Cover / 各 PageHero / RightPanel (PC) / PublicTopBar / Footer
//   - **業務違反 3 機能** (いいね / ブックマーク / 画像ダウンロード) が混入しない (要件 A)
//   - raw photobookId が DOM / data-* に出ない
//   - openingStyle に応じて Cover variant が切替

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { ViewerLayout } from "@/components/Viewer/ViewerLayout";
import { sampleSunsetMemories, sampleCoverlessCasual } from "@/lib/__fixtures__/publicPhotobookSample";

describe("ViewerLayout v2", () => {
  it("正常_sampleSunsetMemories_主要セクション_と_5_PageHero_を描画", () => {
    const photobook = sampleSunsetMemories();
    const html = renderToStaticMarkup(<ViewerLayout photobook={photobook} />);

    // 必須セクション
    expect(html).toContain('data-testid="public-topbar"');
    expect(html).toContain('data-testid="viewer-cover"');
    expect(html).toContain('data-testid="viewer-main"');
    expect(html).toContain('data-testid="viewer-meta-strip"');
    expect(html).toContain('data-testid="viewer-page-nav"');
    expect(html).toContain('data-testid="viewer-right-panel"');
    expect(html).toContain('data-testid="public-page-footer"');
    expect(html).toContain('data-testid="viewer-report-link"');

    // 5 page 全部に PageHero
    for (let i = 1; i <= 5; i++) {
      expect(html).toContain(`data-testid="viewer-page-${i}"`);
      expect(html).toContain(`id="page-${i}"`);
    }

    // Cover (cover_first variant)
    expect(html).toContain('data-cover-variant="cover_first"');
    // sample type=event → pattern A
    expect(html).toContain('data-cover-pattern="A"');

    // ShareActions (Mobile + PC) は片方ずつ存在
    expect(html).toContain('data-testid="viewer-share-actions"');
  });

  it("正常_coverless_casual_は_pattern_C_と_light_variant", () => {
    const photobook = sampleCoverlessCasual();
    const html = renderToStaticMarkup(<ViewerLayout photobook={photobook} />);
    expect(html).toContain('data-cover-pattern="C"');
    expect(html).toContain('data-cover-variant="light"');
    // 「読む」CTA は cover_first 限定なので light では無い
    expect(html).not.toContain('data-testid="viewer-cover-cta"');
  });

  it("ガード_業務違反_3機能_いいね_ブックマーク_画像ダウンロード_が混入しない", () => {
    const photobook = sampleSunsetMemories();
    const html = renderToStaticMarkup(<ViewerLayout photobook={photobook} />);
    // 要件 A (cbabbe6 と同等の guard、将来 regression 防止)
    expect(html).not.toMatch(/いいね/);
    expect(html).not.toMatch(/data-testid="like/);
    expect(html).not.toMatch(/ブックマーク/);
    expect(html).not.toMatch(/data-testid="bookmark/);
    expect(html).not.toMatch(/data-testid="download/);
    expect(html).not.toMatch(/画像をダウンロード/);
    expect(html).not.toMatch(/save\s+image/i);
  });

  it("ガード_raw_photobookId_は_DOM_に出ない", () => {
    const photobook = sampleSunsetMemories();
    // photobookId は "sample-sunset-redacted" だが DOM 露出禁止
    const html = renderToStaticMarkup(<ViewerLayout photobook={photobook} />);
    expect(html).not.toContain("sample-sunset-redacted");
  });

  it("正常_flat_photo_index_が_PageHero_の_aria_label_に正しく反映 (sampleSunsetMemories)", () => {
    const photobook = sampleSunsetMemories();
    const html = renderToStaticMarkup(<ViewerLayout photobook={photobook} />);
    // page 1 photos: 3 → photoIndexBase=0 (写真 1〜3)
    expect(html).toContain('aria-label="Page 1 の写真 1 を全画面で開く"');
    expect(html).toContain('aria-label="Page 1 の写真 2 を全画面で開く"');
    // page 2 photos: 2 → photoIndexBase=3 (写真 4, 5)
    expect(html).toContain('aria-label="Page 2 の写真 1 を全画面で開く"');
    // page 3 photos: 4 → photoIndexBase=5 (写真 6〜9)
    expect(html).toContain('aria-label="Page 3 の写真 1 を全画面で開く"');
  });

  it("正常_PublicTopBar_PublicPageFooter_の共有コンポを使う", () => {
    const photobook = sampleSunsetMemories();
    const html = renderToStaticMarkup(<ViewerLayout photobook={photobook} />);
    expect(html).toContain('data-testid="public-topbar"');
    expect(html).toContain('data-testid="public-page-footer"');
  });

  it("正常_creator_inline_と_creator_card_が両方出る (Mobile + PC 重複表示)", () => {
    const photobook = sampleSunsetMemories();
    const html = renderToStaticMarkup(<ViewerLayout photobook={photobook} />);
    expect(html).toContain('data-testid="viewer-creator-inline"');
    expect(html).toContain('data-testid="viewer-creator-card"');
  });
});
