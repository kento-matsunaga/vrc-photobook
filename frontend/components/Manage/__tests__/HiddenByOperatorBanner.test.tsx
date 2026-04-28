// HiddenByOperatorBanner のレンダリングテスト。
//
// 方針:
//   - SSR 経路（renderToStaticMarkup）で HTML をレンダーし文字列で検証
//   - ManagePanel が hiddenByOperator の真偽で出し分けることを別 test で確認

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { HiddenByOperatorBanner } from "@/components/Manage/HiddenByOperatorBanner";
import { ManagePanel } from "@/components/Manage/ManagePanel";
import type { ManagePhotobook } from "@/lib/managePhotobook";

const baseManagePhotobook: ManagePhotobook = {
  photobookId: "00000000-0000-0000-0000-000000000001",
  type: "event",
  title: "Test Photobook",
  status: "published",
  visibility: "unlisted",
  hiddenByOperator: false,
  publicUrlSlug: "ok12pp34zz56gh78",
  publicUrlPath: "/p/ok12pp34zz56gh78",
  manageUrlTokenVersion: 1,
  availableImageCount: 5,
};

describe("HiddenByOperatorBanner", () => {
  const tests = [
    {
      name: "正常_主要文言を含む",
      description: "Given: banner レンダー, When: 表示, Then: 公開ページ・SNS 非表示を説明",
      assert: (html: string) => {
        expect(html).toContain("一時的に非公開");
        expect(html).toContain("公開ページと SNS プレビューには表示されません");
        expect(html).toContain("編集を");
      },
    },
    {
      name: "正常_aria_role_status",
      description: "Given: banner レンダー, When: HTML 出力, Then: role=status",
      assert: (html: string) => {
        expect(html).toContain('role="status"');
        expect(html).toContain('data-testid="hidden-by-operator-banner"');
      },
    },
  ];

  for (const tt of tests) {
    it(tt.name, () => {
      const html = renderToStaticMarkup(<HiddenByOperatorBanner />);
      tt.assert(html);
    });
  }
});

describe("ManagePanel hiddenByOperator banner 表示制御", () => {
  it("正常_hiddenByOperator_true_でbanner表示", () => {
    const html = renderToStaticMarkup(
      <ManagePanel
        photobook={{ ...baseManagePhotobook, hiddenByOperator: true }}
        appBaseUrl="https://app.vrc-photobook.com"
      />,
    );
    expect(html).toContain('data-testid="hidden-by-operator-banner"');
    expect(html).toContain("一時的に非公開");
  });

  it("正常_hiddenByOperator_false_でbanner非表示", () => {
    const html = renderToStaticMarkup(
      <ManagePanel
        photobook={{ ...baseManagePhotobook, hiddenByOperator: false }}
        appBaseUrl="https://app.vrc-photobook.com"
      />,
    );
    expect(html).not.toContain('data-testid="hidden-by-operator-banner"');
    expect(html).not.toContain("運営により一時的に非公開");
  });

  it("正常_hiddenByOperator_true_でも編集ブロックや再発行は強制されない", () => {
    // 既存の「再発行（後日対応）」placeholder と公開 URL は引き続き表示される。
    // banner はあくまで状態説明であり、UI を全面ブロックしない。
    const html = renderToStaticMarkup(
      <ManagePanel
        photobook={{ ...baseManagePhotobook, hiddenByOperator: true }}
        appBaseUrl="https://app.vrc-photobook.com"
      />,
    );
    expect(html).toContain("管理リンクの再発行");
    expect(html).toContain("公開 URL");
  });
});
