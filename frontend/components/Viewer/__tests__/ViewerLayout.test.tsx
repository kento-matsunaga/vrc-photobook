// ViewerLayout の SSR 統合検証。
//
// 観点:
//   - 業務違反機能（いいね / ブックマーク / 画像ダウンロード）が混入しない
//   - 共有 / URL コピー / 通報 / 作る CTA が業務要件として描画される
//   - photobookId が DOM に出ない
//   - cover_first / light で適切に分岐
//
// セキュリティ:
//   - 削除した機能の文言が混入しないことを assert で守る

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { ViewerLayout } from "@/components/Viewer/ViewerLayout";
import {
  sampleSunsetMemories,
  sampleWithVariant,
} from "@/lib/__fixtures__/publicPhotobookSample";

describe("ViewerLayout", () => {
  it("正常_必須機能_共有_コピー_通報_作るCTA", () => {
    const html = renderToStaticMarkup(<ViewerLayout photobook={sampleSunsetMemories()} />);
    // 共有
    expect(html).toContain('data-testid="share-action-x"');
    expect(html).toContain('data-testid="share-action-copy"');
    // 通報（PC sidebar / Mobile actions）
    expect(html).toMatch(/data-testid="viewer-report-link/);
    // 作る CTA
    expect(html).toMatch(/このフォトブックを作る/);
  });

  it("禁止_削除した機能の文言が混入しない", () => {
    const html = renderToStaticMarkup(<ViewerLayout photobook={sampleSunsetMemories()} />);
    // P0 で削除した 3 機能の文言・data-testid が一切ないこと
    expect(html).not.toMatch(/いいね/);
    expect(html).not.toMatch(/data-testid="like/);
    expect(html).not.toMatch(/ブックマーク/);
    expect(html).not.toMatch(/data-testid="bookmark/);
    expect(html).not.toMatch(/data-testid="download/);
    expect(html).not.toMatch(/画像をダウンロード/);
  });

  it("セキュリティ_photobookIdがDOMに出ない", () => {
    const html = renderToStaticMarkup(<ViewerLayout photobook={sampleSunsetMemories()} />);
    expect(html).not.toContain("00000000-0000-0000-0000-000000000001");
  });

  it("正常_cover_firstで読むCTA有り", () => {
    const html = renderToStaticMarkup(
      <ViewerLayout photobook={sampleWithVariant({})} />,
    );
    expect(html).toContain('data-testid="viewer-cover-read-cta"');
  });

  it("正常_lightで読むCTA無し", () => {
    const html = renderToStaticMarkup(
      <ViewerLayout photobook={sampleWithVariant({})} />,
    );
    // sampleSunsetMemories は cover_first 既定だが、light variant も健全に動くこと
    const lightSample = { ...sampleSunsetMemories(), openingStyle: "light" };
    const html2 = renderToStaticMarkup(<ViewerLayout photobook={lightSample} />);
    expect(html2).not.toContain('data-testid="viewer-cover-read-cta"');
    // 元 cover_first sample と区別できること
    expect(html).not.toBe(html2);
  });

  it("正常_全ページが id付きでレンダーされる_anchor_jump用", () => {
    const html = renderToStaticMarkup(<ViewerLayout photobook={sampleSunsetMemories()} />);
    expect(html).toContain('id="page-1"');
    expect(html).toContain('id="page-2"');
    expect(html).toContain('id="page-3"');
  });

  it("正常_PageNavSidebarがPC用に出る", () => {
    const html = renderToStaticMarkup(<ViewerLayout photobook={sampleSunsetMemories()} />);
    expect(html).toContain('data-testid="page-nav-sidebar"');
  });

  it("正常_RightPanelがPC用に出る", () => {
    const html = renderToStaticMarkup(<ViewerLayout photobook={sampleSunsetMemories()} />);
    expect(html).toContain('data-testid="viewer-right-panel"');
  });

  it("正常_TypeAccentが描画される", () => {
    const html = renderToStaticMarkup(<ViewerLayout photobook={sampleSunsetMemories()} />);
    expect(html).toContain('data-testid="type-accent"');
  });
});
