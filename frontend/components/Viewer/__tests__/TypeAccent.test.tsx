// TypeAccent の SSR 検証。

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { TypeAccent } from "@/components/Viewer/TypeAccent";

describe("TypeAccent", () => {
  const cases = [
    { type: "event", label: "イベント" },
    { type: "daily", label: "おはツイ" },
    { type: "portfolio", label: "作品集" },
    { type: "avatar", label: "アバター紹介" },
    { type: "world", label: "ワールド" },
    { type: "memory", label: "思い出" },
    { type: "free", label: "自由" },
  ];

  for (const tt of cases) {
    it(`正常_${tt.type}_label`, () => {
      const html = renderToStaticMarkup(<TypeAccent type={tt.type} />);
      expect(html).toContain('data-testid="type-accent"');
      expect(html).toContain(`data-photobook-type="${tt.type}"`);
      expect(html).toContain(tt.label);
    });
  }

  it("異常_unknown_type_falls_back_to_free", () => {
    const html = renderToStaticMarkup(<TypeAccent type="unknown_type_xyz" />);
    expect(html).toContain('data-photobook-type="free"');
    expect(html).toContain("自由");
  });
});
