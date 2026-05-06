// ShareActions.tsx の SSR 構造検証。
//
// 注意:
//   - vitest environment: "node" のため navigator.share / navigator.clipboard.writeText の
//     呼出 mock は jsdom 必要。本ファイルは初期 render の DOM 構造のみ assert
//   - 実機での挙動 (Safari ネイティブシート / Chrome clipboard) は手動 smoke で確認

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { ShareActions } from "@/components/Viewer/ShareActions";

describe("ShareActions", () => {
  it("正常_X共有_と_URLコピー_の_2button_が描画される", () => {
    const html = renderToStaticMarkup(
      <ShareActions
        shareUrl="https://app.vrc-photobook.com/p/sample"
        shareText="Sunset Memories | VRC PhotoBook"
      />,
    );
    expect(html).toContain('data-testid="viewer-share-actions"');
    expect(html).toContain('data-testid="viewer-share-x"');
    expect(html).toContain('data-testid="viewer-share-copy"');
    expect(html).toContain("X で共有する");
    expect(html).toContain("URL をコピー");
  });

  it("正常_初期状態_の_コピーボタン_は_idle_文言", () => {
    const html = renderToStaticMarkup(
      <ShareActions shareUrl="https://example.invalid/" shareText="t" />,
    );
    expect(html).toContain("URL をコピー");
    expect(html).not.toContain("コピーしました");
    expect(html).not.toContain("コピー失敗");
  });

  it("正常_shareUrl_は_DOM_に直接埋まらない (button onClick で参照)", () => {
    const html = renderToStaticMarkup(
      <ShareActions shareUrl="https://app.vrc-photobook.com/p/sample" shareText="t" />,
    );
    // shareUrl は state / closure に保持され、初期 DOM の data-* / href には出ない
    expect(html).not.toContain('href="https://app.vrc-photobook.com/p/sample"');
    expect(html).not.toContain('data-share-url');
  });
});
