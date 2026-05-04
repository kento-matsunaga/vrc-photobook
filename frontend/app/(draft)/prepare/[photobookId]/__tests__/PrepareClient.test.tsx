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
    images: [],
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

  it("正常_ε-fix_アップロード説明文_自動圧縮を案内し_旧10MB1枚表記は出ない", () => {
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={emptyView("pb-test-redacted")}
      />,
    );
    // 新文言: 「最大 20 枚まで（HEIC / HEIF 未対応）。大きい画像は送信前に自動圧縮されます。」
    expect(html).toContain("最大 20 枚まで");
    expect(html).toContain("HEIC / HEIF 未対応");
    expect(html).toContain("大きい画像は送信前に自動圧縮されます");
    // 旧文言 「最大 10MB / 1 枚」は出ない（自動圧縮で per-file MB 上限の文言は廃止）
    expect(html).not.toContain("最大 10MB / 1 枚");
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

  it("正常_processingCount > 0 のとき通常 1〜2 分案内が出る", () => {
    const view = emptyView("pb-test-redacted");
    view.processingCount = 2;
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={view}
      />,
    );
    expect(html).toContain("通常 1〜2 分ほどで完了します");
    expect(html).toContain('data-testid="prepare-normal-notice"');
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

  it("正常_server images から reload 復元 tile が描画される（processing / available / failed）", () => {
    const SECRET_IMG_PROC = "img-secret-processing-zzz1111";
    const SECRET_IMG_AVAIL = "img-secret-available-zzz2222";
    const SECRET_IMG_FAILED = "img-secret-failed-zzz3333";
    const view: EditView = {
      ...emptyView("pb-test-redacted"),
      version: 7,
      processingCount: 1,
      failedCount: 1,
      images: [
        {
          imageId: SECRET_IMG_PROC,
          status: "processing",
          sourceFormat: "jpeg",
          originalByteSize: 2_500_000,
          createdAt: "2026-05-02T00:00:00Z",
        },
        {
          imageId: SECRET_IMG_AVAIL,
          status: "available",
          sourceFormat: "png",
          originalByteSize: 3_500_000,
          createdAt: "2026-05-02T00:00:01Z",
        },
        {
          imageId: SECRET_IMG_FAILED,
          status: "failed",
          sourceFormat: "jpeg",
          originalByteSize: 4_000_000,
          createdAt: "2026-05-02T00:00:02Z",
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
    expect(html).toContain('data-testid="prepare-tiles"');
    expect(html).toContain("復元された画像");
    expect(html).toContain("処理中");
    expect(html).toContain("完了");
    expect(html).toContain("失敗");
    // raw image_id が UI / DOM / data-testid / aria-label / title に出ない（最重要）
    expect(html).not.toContain(SECRET_IMG_PROC);
    expect(html).not.toContain(SECRET_IMG_AVAIL);
    expect(html).not.toContain(SECRET_IMG_FAILED);
  });

  it("正常_progress UI に n/m 表示が出る", () => {
    const view: EditView = {
      ...emptyView("pb-test-redacted"),
      version: 1,
      processingCount: 1,
      images: [
        {
          imageId: "img-restore-aaa",
          status: "available",
          sourceFormat: "jpeg",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:00Z",
        },
        {
          imageId: "img-restore-bbb",
          status: "processing",
          sourceFormat: "jpeg",
          originalByteSize: 2_000_000,
          createdAt: "2026-05-02T00:00:01Z",
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
    expect(html).toContain('data-testid="prepare-progress"');
    expect(html).toContain("進捗");
  });

  it("正常_processing 中のとき normal 通知（10 分未満）が出る、slow 通知は出ない", () => {
    const view: EditView = {
      ...emptyView("pb-test-redacted"),
      processingCount: 1,
      images: [
        {
          imageId: "img-x",
          status: "processing",
          sourceFormat: "jpeg",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:00Z",
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
    expect(html).toContain('data-testid="prepare-normal-notice"');
    expect(html).not.toContain('data-testid="prepare-slow-notice"');
  });

  it("正常_β-3_eyebrow_Step2_3_と_PC_wf-grid-2-1_layout_class_を持つ", () => {
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={emptyView("pb-test-redacted")}
      />,
    );
    // β-3: design eyebrow「Step 2 / 3」(`wf-screens-a.jsx:338` / `:394`)
    expect(html).toContain("Step 2 / 3");
    // β-3: PC は wf-grid-2-1 (2fr 1fr) layout (`wireframe-styles.css:568`)
    expect(html).toContain("sm:grid-cols-[2fr_1fr]");
  });

  it("正常_β-3_dashed_picker_と_Mobile_bottom_sticky_CTA_wrapper_を持つ", () => {
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={emptyView("pb-test-redacted")}
      />,
    );
    // β-3: dashed picker (`wireframe-styles.css:165-175` `.wf-box.dashed` 風)
    expect(html).toContain("border-dashed");
    // β-3: Mobile bottom sticky CTA (`wireframe-styles.css:513-520` `.wf-m-stick-cta`)
    expect(html).toMatch(
      /class="[^"]*fixed[^"]*inset-x-0[^"]*bottom-0[^"]*sm:hidden[^"]*"/,
    );
  });

  it("正常_β-3_prepare-proceed_button_は_PC_と_Mobile_に_2_箇所描画される", () => {
    const html = renderToStaticMarkup(
      <PrepareClient
        photobookId="pb-test-redacted"
        turnstileSiteKey="dummy-site-key"
        initialView={emptyView("pb-test-redacted")}
      />,
    );
    // β-3: PC sidebar + Mobile sticky の 2 箇所に同 data-testid で render
    const matches = html.match(/<button[^>]*data-testid="prepare-proceed"/g) ?? [];
    expect(matches.length).toBe(2);
    // 両 button が同じ disabled state (queue 空 → 共に disabled)
    const buttons = html.match(/<button[^>]*data-testid="prepare-proceed"[^>]*>/g) ?? [];
    expect(buttons.length).toBe(2);
    for (const btn of buttons) {
      expect(btn).toMatch(/disabled=""/);
    }
  });

  it("正常_β-3_raw_imageId_storage_key_upload_url_は_DOM_に出ない", () => {
    const view: EditView = {
      ...emptyView("pb-test-redacted"),
      processingCount: 1,
      images: [
        {
          imageId: "img-secret-leak-test-zzz9999",
          status: "processing",
          sourceFormat: "jpeg",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:00Z",
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
    // raw imageId は UI / data-testid / aria-label / title に絶対に出ない
    expect(html).not.toContain("img-secret-leak-test-zzz9999");
    // storage_key / upload_url 等の生 token 系名前も出ない (本 SSR では不在を確認)
    expect(html).not.toMatch(/storage_key=/i);
    expect(html).not.toMatch(/upload_url=/i);
    expect(html).not.toMatch(/verification_token=/i);
  });
});
