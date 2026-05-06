// PageNote.tsx の SSR レンダリング検証。
//
// 観点:
//   - note 空 / undefined / 空白のみ → null
//   - 改行 \n が whitespace-pre-line で保持される

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PageNote } from "@/components/Viewer/PageNote";

describe("PageNote", () => {
  type Case = {
    name: string;
    description: string;
    note: string | undefined;
    expectInHTML: string[];
    expectIsEmpty?: boolean;
  };

  const cases: Case[] = [
    {
      name: "正常_note_あり",
      description: "Given note 文字列, When render, Then blockquote + 文字列が出る",
      note: "ふと撮った 1 枚が、その日の空気をぜんぶ覚えていた。",
      expectInHTML: [
        '<blockquote data-testid="page-note"',
        "ふと撮った 1 枚が、その日の空気をぜんぶ覚えていた。",
      ],
    },
    {
      name: "正常_改行付き_note",
      description: "Given note に \\n が含まれる, When render, Then 文字列がそのまま出る (whitespace-pre-line で表示時に保持)",
      note: "1 行目\n2 行目",
      expectInHTML: ["1 行目\n2 行目", "whitespace-pre-line"],
    },
    {
      name: "境界_note_undefined_は_null",
      description: "Given note undefined, When render, Then html 空文字",
      note: undefined,
      expectInHTML: [],
      expectIsEmpty: true,
    },
    {
      name: "境界_note_空文字_は_null",
      description: "Given note 空文字, When render, Then html 空文字",
      note: "",
      expectInHTML: [],
      expectIsEmpty: true,
    },
    {
      name: "境界_note_空白のみ_は_null",
      description: "Given note 空白のみ, When render, Then html 空文字",
      note: "   \n  \t  ",
      expectInHTML: [],
      expectIsEmpty: true,
    },
  ];

  for (const tt of cases) {
    it(tt.name, () => {
      const html = renderToStaticMarkup(<PageNote note={tt.note} />);
      if (tt.expectIsEmpty) {
        expect(html).toBe("");
      } else {
        for (const s of tt.expectInHTML) {
          expect(html).toContain(s);
        }
      }
    });
  }
});
