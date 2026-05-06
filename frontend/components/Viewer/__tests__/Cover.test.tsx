// Cover の SSR レンダリング検証。
//
// 観点:
//   - data-testid="viewer-cover" + variant + pattern attribute が出る
//   - cover_first / light の 2 variant
//   - 3 contrast pattern (gradient / panel / fallback)
//   - cover なしのとき fallback に倒れる
//   - 「読む」CTA は cover_first のときだけ出る
//   - 公開日が表示される
//
// セキュリティ:
//   - photobookId が DOM に出ないこと
//   - presigned URL が data-testid / aria-label に出ないこと

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { Cover } from "@/components/Viewer/Cover";
import { sampleSunsetMemories, sampleWithVariant } from "@/lib/__fixtures__/publicPhotobookSample";

describe("Cover", () => {
  const cases = [
    {
      name: "正常_cover_first_gradient",
      description: "Given: cover あり / openingStyle cover_first / type memory, When: render, Then: gradient pattern + 読む CTA",
      photobook: sampleSunsetMemories(),
      variant: "cover_first" as const,
      pattern: undefined,
      expectInHTML: [
        'data-cover-variant="cover_first"',
        'data-cover-pattern="gradient"',
        "Sunset Memories",
        "あの日見た、夕暮れの記憶",
        "by すずきさん",
        "@suzuki_vrc",
        "公開日 2025.05.12",
        'data-testid="viewer-cover-read-cta"',
        "読む",
      ],
      notInHTML: [],
    },
    {
      name: "正常_cover_first_panel_for_portfolio",
      description: "Given: type=portfolio + cover あり, When: pattern 自動判定, Then: panel pattern",
      photobook: sampleWithVariant({ type: "portfolio" }),
      variant: "cover_first" as const,
      pattern: undefined,
      expectInHTML: ['data-cover-pattern="panel"', "読む"],
      notInHTML: [],
    },
    {
      name: "正常_cover_first_fallback_when_no_cover",
      description: "Given: cover なし, When: variant cover_first, Then: fallback pattern + 読む CTA は出さない",
      photobook: sampleWithVariant({ cover: undefined }),
      variant: "cover_first" as const,
      pattern: undefined,
      expectInHTML: ['data-cover-pattern="fallback"', "Sunset Memories"],
      notInHTML: ['data-testid="viewer-cover-read-cta"'],
    },
    {
      name: "正常_light_variant",
      description: "Given: openingStyle light, When: render, Then: light variant + 読む CTA は出ない",
      photobook: sampleSunsetMemories(),
      variant: "light" as const,
      pattern: undefined,
      expectInHTML: [
        'data-cover-variant="light"',
        "Sunset Memories",
        "あの日見た、夕暮れの記憶",
        "公開日 2025.05.12",
      ],
      notInHTML: ['data-testid="viewer-cover-read-cta"'],
    },
    {
      name: "正常_creatorXIdなしでも描画される",
      description: "Given: creatorXId 未設定, Then: @id 行は出ない",
      photobook: sampleWithVariant({ creatorXId: undefined }),
      variant: "cover_first" as const,
      pattern: undefined,
      expectInHTML: ["by すずきさん"],
      notInHTML: ["@suzuki_vrc"],
    },
  ];

  for (const tt of cases) {
    it(tt.name, () => {
      const html = renderToStaticMarkup(
        <Cover photobook={tt.photobook} variant={tt.variant} pattern={tt.pattern} />,
      );
      for (const s of tt.expectInHTML) {
        expect(html, tt.description).toContain(s);
      }
      for (const s of tt.notInHTML) {
        expect(html, tt.description).not.toContain(s);
      }
    });
  }

  it("セキュリティ_photobookIdがDOMに出ない", () => {
    const html = renderToStaticMarkup(
      <Cover photobook={sampleSunsetMemories()} variant="cover_first" />,
    );
    expect(html).not.toContain("00000000-0000-0000-0000-000000000001");
  });
});
