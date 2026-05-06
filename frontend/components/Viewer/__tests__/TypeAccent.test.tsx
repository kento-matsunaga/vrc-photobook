// TypeAccent.tsx の SSR レンダリング検証 (table 駆動)。
//
// 観点:
//   - 既知 7 type それぞれのラベルが出る
//   - 未知 type は null (空文字)

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { TypeAccent } from "@/components/Viewer/TypeAccent";

describe("TypeAccent", () => {
  type Case = {
    name: string;
    description: string;
    type: string;
    expectInHTML: string[];
    expectIsEmpty?: boolean;
  };

  const cases: Case[] = [
    { name: "正常_event", description: "Given event, Then 'Event' label + testid", type: "event", expectInHTML: ['data-testid="type-accent-event"', "Event"] },
    { name: "正常_portfolio", description: "Given portfolio, Then 'Portfolio'", type: "portfolio", expectInHTML: ['data-testid="type-accent-portfolio"', "Portfolio"] },
    { name: "正常_world", description: "Given world, Then 'World'", type: "world", expectInHTML: ['data-testid="type-accent-world"', "World"] },
    { name: "正常_casual", description: "Given casual, Then 'Casual'", type: "casual", expectInHTML: ['data-testid="type-accent-casual"', "Casual"] },
    { name: "正常_oha_tweet", description: "Given oha_tweet, Then 'おはツイ'", type: "oha_tweet", expectInHTML: ['data-testid="type-accent-oha_tweet"', "おはツイ"] },
    { name: "正常_collection", description: "Given collection, Then 'Collection'", type: "collection", expectInHTML: ['data-testid="type-accent-collection"', "Collection"] },
    { name: "正常_archive", description: "Given archive, Then 'Archive'", type: "archive", expectInHTML: ['data-testid="type-accent-archive"', "Archive"] },
    { name: "正常_memory_alias", description: "Given memory (alias), Then 'Memory'", type: "memory", expectInHTML: ['data-testid="type-accent-memory"', "Memory"] },
    { name: "境界_未知_type_は_null", description: "Given 未知 type, Then 空文字 (silent skip)", type: "totally_unknown_xyz", expectInHTML: [], expectIsEmpty: true },
  ];

  for (const tt of cases) {
    it(tt.name, () => {
      const html = renderToStaticMarkup(<TypeAccent type={tt.type} />);
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
