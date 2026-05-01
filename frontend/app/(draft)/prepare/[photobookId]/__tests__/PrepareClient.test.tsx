// PrepareClient SSR レンダリング検証。
//
// 観点:
//   - 画面 testid / 主要文言 / 7 つの中核要素が描画される
//   - 初期状態で「編集へ進む」ボタンが disabled
//   - Turnstile widget placeholder（"送信前の Bot 検証"）が出る
//   - file input は multiple 属性を持つ
//   - raw token / Cookie / Secret が初期 HTML に出ない
//
// 注意:
//   - SSR では Turnstile callback / fetch / polling は走らない
//   - useEffect は SSR では発火しないため、初期 render の static markup のみ assert

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import type { EditView } from "@/lib/editPhotobook";
import { PrepareClient } from "@/app/(draft)/prepare/[photobookId]/PrepareClient";

function emptyView(photobookId: string): EditView {
  return {
    photobookId,
    status: "draft",
    version: 0,
    settings: {
      type: "memory",
      title: "",
      layout: "simple",
      openingStyle: "light",
      visibility: "unlisted",
    },
    pages: [],
    processingCount: 0,
    failedCount: 0,
  };
}

describe("PrepareClient 初期描画", () => {
  it("正常_主要セクションと CTA が描画される", () => {
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={emptyView("pb-test-redacted")}
      />,
    );
    expect(html).toContain('data-testid="prepare-page"');
    expect(html).toContain('data-testid="prepare-picker"');
    expect(html).toContain('data-testid="prepare-summary"');
    expect(html).toContain('data-testid="prepare-file-input"');
    expect(html).toContain('data-testid="prepare-proceed"');
    expect(html).toContain("写真をまとめて追加");
    expect(html).toContain("送信前の Bot 検証が必要です");
  });

  it("正常_file input が multiple 属性を持つ", () => {
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={emptyView("pb-test-redacted")}
      />,
    );
    expect(html).toMatch(/<input[^>]*type="file"[^>]*multiple/);
    expect(html).toMatch(/accept="image\/jpeg,image\/png,image\/webp"/);
  });

  it("正常_初期状態で「編集へ進む」ボタンが disabled (queue 空 / placed photo 0)", () => {
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={emptyView("pb-test-redacted")}
      />,
    );
    // 属性順序に依らない形で disabled="" を assert（class 内の "disabled:" Tailwind variant と区別）
    const buttonMatch = html.match(
      /<button[^>]*data-testid="prepare-proceed"[^>]*>/,
    );
    expect(buttonMatch).not.toBeNull();
    expect(buttonMatch?.[0] ?? "").toMatch(/disabled=""/);
  });

  it("正常_Bot 検証未完了時は file input が disabled", () => {
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={emptyView("pb-test-redacted")}
      />,
    );
    // React は属性順序を保証しないため、input タグ全体を捕まえてから disabled="" を assert
    const inputMatch = html.match(
      /<input[^>]*data-testid="prepare-file-input"[^>]*\/?>/,
    );
    expect(inputMatch).not.toBeNull();
    expect(inputMatch?.[0] ?? "").toMatch(/disabled=""/);
    expect(html).toContain("まず Bot 検証を完了してください");
  });

  it("正常_既存 photo がある場合の summary に 0 集計が出る (queue 空)", () => {
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={emptyView("pb-test-redacted")}
      />,
    );
    // 0 集計（合計 / 完了 / 処理中 / 失敗）が表示される
    expect(html).toContain("合計");
    expect(html).toContain("完了");
    expect(html).toContain("処理中");
    expect(html).toContain("失敗");
  });

  it("正常_processingCount > 0 のとき待機メッセージが出る", () => {
    const view = emptyView("pb-test-redacted");
    view.processingCount = 2;
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={view}
      />,
    );
    expect(html).toContain("画像処理は最大 5 分ほどかかることがあります");
  });

  it("正常_Cookie / token / Secret 値が HTML に出ない", () => {
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={emptyView("pb-test-redacted")}
      />,
    );
    expect(html).not.toMatch(/draft_edit_token=/i);
    expect(html).not.toMatch(/manage_url_token=/i);
    expect(html).not.toMatch(/session_token=/i);
    expect(html).not.toMatch(/Set-Cookie/i);
    expect(html).not.toMatch(/Bearer\s+[A-Za-z0-9._-]{20,}/);
  });

  it("正常_既に placed photo + processing 0 なら「編集へ進む」が enabled", () => {
    const view: EditView = {
      ...emptyView("pb-test-redacted"),
      pages: [
        {
          pageId: "page-1",
          displayOrder: 0,
          photos: [
            {
              photoId: "photo-1",
              imageId: "img-1",
              displayOrder: 0,
              variants: {
                display: {
                  url: "https://example/d.jpg",
                  expiresAt: "2026-12-31T00:00:00Z",
                  width: 1600,
                  height: 1066,
                },
                thumbnail: {
                  url: "https://example/t.jpg",
                  expiresAt: "2026-12-31T00:00:00Z",
                  width: 480,
                  height: 320,
                },
              },
            },
          ],
        },
      ],
    };
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={view}
      />,
    );
    // disabled 属性がないことを確認（enabled 状態）
    // React は disabled=false の場合 attribute 出力なし、disabled=true の場合 disabled="" を出力。
    // class 文字列内の Tailwind variant ("disabled:cursor-not-allowed" 等) と区別するため
    // disabled="" のみを check する。
    const buttonMatch = html.match(
      /<button[^>]*data-testid="prepare-proceed"[^>]*>/,
    );
    expect(buttonMatch).not.toBeNull();
    expect(buttonMatch?.[0] ?? "").not.toMatch(/disabled=""/);
    expect(html).toContain("編集へ進む");
  });
});
