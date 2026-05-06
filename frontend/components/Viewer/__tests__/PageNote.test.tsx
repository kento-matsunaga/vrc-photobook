// PageNote の SSR レンダリング検証。

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PageNote } from "@/components/Viewer/PageNote";

describe("PageNote", () => {
  const cases = [
    {
      name: "正常_undefined_returns_null",
      description: "Given: note undefined, Then: null",
      note: undefined,
      expectEmpty: true,
    },
    {
      name: "正常_空白のみ_returns_null",
      description: "Given: 空白のみ, Then: null",
      note: "   \n   ",
      expectEmpty: true,
    },
    {
      name: "正常_text_renders",
      description: "Given: 任意 text, Then: クリーム色 box に描画",
      note: "またみんなで来ようね",
      expectEmpty: false,
      expectInHTML: ['data-testid="page-note"', "Note", "またみんなで来ようね"],
    },
    {
      name: "正常_改行_preserved",
      description: "Given: 改行入り text, Then: whitespace-pre-line で保持",
      note: "1 行目\n2 行目",
      expectEmpty: false,
      expectInHTML: ["whitespace-pre-line", "1 行目", "2 行目"],
    },
  ] as const;

  for (const tt of cases) {
    it(tt.name, () => {
      const html = renderToStaticMarkup(<PageNote note={tt.note} />);
      if (tt.expectEmpty) {
        expect(html, tt.description).toBe("");
        return;
      }
      for (const s of tt.expectInHTML) {
        expect(html, tt.description).toContain(s);
      }
    });
  }
});
