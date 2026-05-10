// ManageUrlNoticeBanner SSR レンダリングテスト (M-1a)。
import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { ManageUrlNoticeBanner } from "@/components/Manage/ManageUrlNoticeBanner";

describe("ManageUrlNoticeBanner", () => {
  const tests = [
    {
      name: "正常_主要文言を含む",
      description:
        "Given: ManageUrlNoticeBanner, When: renderToStaticMarkup, Then: 共有禁止 / SNS / この端末の管理権限を削除 を含む",
      check: (html: string) => {
        expect(html).toContain("管理 URL の取り扱いについて");
        expect(html).toContain("他人と共有しないでください");
        expect(html).toContain("SNS や公開チャットに貼らない");
        expect(html).toContain("この端末の管理権限を削除");
      },
    },
    {
      name: "正常_data-testidが付与される",
      description: "Given: 既定 testId, When: render, Then: data-testid=manage-url-notice-banner",
      check: (html: string) => {
        expect(html).toContain('data-testid="manage-url-notice-banner"');
      },
    },
    {
      name: "正常_role_noteが付与される",
      description: "Given: render, Then: role=note でアクセシビリティ意図を表す",
      check: (html: string) => {
        expect(html).toContain('role="note"');
      },
    },
  ];

  for (const tt of tests) {
    it(tt.name, () => {
      const html = renderToStaticMarkup(<ManageUrlNoticeBanner />);
      tt.check(html);
    });
  }
});
