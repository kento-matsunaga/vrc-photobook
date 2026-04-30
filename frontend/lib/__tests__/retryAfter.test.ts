// formatRetryAfterDisplay の単体テスト。
import { describe, expect, it } from "vitest";
import { formatRetryAfterDisplay } from "@/lib/retryAfter";

describe("formatRetryAfterDisplay", () => {
  const cases: Array<{ name: string; in: number; want: string }> = [
    { name: "0以下は1分ほど", in: 0, want: "1 分ほど" },
    { name: "負も1分ほど", in: -10, want: "1 分ほど" },
    { name: "NaN_は1分ほど", in: Number.NaN, want: "1 分ほど" },
    { name: "59秒は1分ほど", in: 59, want: "1 分ほど" },
    { name: "60秒は1分", in: 60, want: "1 分ほど" },
    { name: "120秒は2分", in: 120, want: "2 分ほど" },
    { name: "121秒は3分_切り上げ", in: 121, want: "3 分ほど" },
    { name: "1時間は60分", in: 3600, want: "60 分ほど" },
    { name: "61分は1時間1分", in: 61 * 60, want: "1 時間 1 分ほど" },
    { name: "2時間ぴったり", in: 2 * 3600, want: "2 時間ほど" },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(formatRetryAfterDisplay(c.in)).toBe(c.want);
    });
  }
});
