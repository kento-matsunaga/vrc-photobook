// SensitiveGate.tsx の SSR 構造検証。
//
// 観点:
//   - isSensitive=false → children をそのまま render (gate 描画なし)
//   - isSensitive=true → children を render せず、gate UI + 「同意して見る」button
//   - 永続化なし (state のみ) なので reload は SSR 段階で常に gate

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { SensitiveGate } from "@/components/Viewer/SensitiveGate";

describe("SensitiveGate", () => {
  it("正常_isSensitive_false_は_children_そのまま_render", () => {
    const html = renderToStaticMarkup(
      <SensitiveGate isSensitive={false}>
        <div data-testid="gated-children">本文</div>
      </SensitiveGate>,
    );
    expect(html).toContain('data-testid="gated-children"');
    expect(html).toContain("本文");
    expect(html).not.toContain('data-testid="viewer-sensitive-gate"');
  });

  it("正常_isSensitive_true_は_gate_UI_を出して_children_を隠す", () => {
    const html = renderToStaticMarkup(
      <SensitiveGate isSensitive={true}>
        <div data-testid="gated-children">本文</div>
      </SensitiveGate>,
    );
    expect(html).toContain('data-testid="viewer-sensitive-gate"');
    expect(html).toContain('data-testid="viewer-sensitive-agree"');
    expect(html).toContain("センシティブな内容を含む可能性があります");
    expect(html).toContain("同意して見る");
    expect(html).not.toContain('data-testid="gated-children"');
    expect(html).not.toContain("本文");
  });
});
