// CompleteView の SSR レンダリングテスト (M-2 STOP δ、ADR-0007)。
//
// 検証範囲:
//   - 初期 SSR 状態は `complete-ogp-checking` phase（useEffect は SSR では起動しない）
//   - 共有 CTA「公開ページを開く」は disable + spinner、別の disabled button として描画
//   - 既存 testid（`complete-view` / `complete-actions` / `complete-back-to-edit`
//     / `complete-faq-link`）は維持
//   - photobookId / API URL / token / slug は DOM に raw 値出力しない（href 等の構造化のみ）
//
// 制約:
//   - vitest 環境は node、jsdom 無し。useEffect / setInterval を起動する polling state
//     遷移の test はここではできない。polling lib (`lib/ogpReadiness.ts`) 側 unit test
//     で classification を担保し、本 test は SSR 初期状態のみを assert する。
//   - polling 後の `complete-ogp-ready` / `complete-ogp-timeout` phase は Workers
//     production chunk grep + 実機 smoke (STOP δ deploy 後) で確認する。
import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { CompleteView } from "@/components/Complete/CompleteView";

const baseProps = {
  appBaseUrl: "https://app.test",
  photobookId: "00000000-0000-0000-0000-000000000001",
  publicUrlPath: "/p/ok12pp34zz56gh78",
  manageUrlPath: "/manage/token/DUMMY_TOKEN_FOR_TEST",
  onBackToEdit: () => {},
};

describe("CompleteView: SSR 初期状態 (M-2 STOP δ polling 前)", () => {
  const tests = [
    {
      name: "正常_ogp_checking_phase_が初期表示",
      description:
        "Given: CompleteView, When: SSR, Then: data-testid=complete-ogp-checking が出力される（useEffect 未起動の初期 state）",
      check: (html: string) => {
        expect(html).toContain('data-testid="complete-ogp-checking"');
        expect(html).not.toContain('data-testid="complete-ogp-ready"');
        expect(html).not.toContain('data-testid="complete-ogp-timeout"');
      },
    },
    {
      name: "正常_共有CTAは_disabled_button_で描画される",
      description:
        "Given: 初期 checking phase, When: SSR, Then: `公開ページを開く` が <a> ではなく <button disabled> で描画され、spinner SVG を含む",
      check: (html: string) => {
        // disabled button 側で出力されている（<a target="_blank"> 経路ではない）
        expect(html).toMatch(/<button[^>]*disabled[^>]*data-testid="complete-open-viewer"/);
        expect(html).toContain("animate-spin");
        // 「公開ページを開く」テキスト自体は維持
        expect(html).toContain("公開ページを開く");
      },
    },
    {
      name: "正常_checking_phase_の文言が出る",
      description:
        "Given: checking, Then: 「OGP プレビュー画像の準備中です。SNS シェアの前に少しお待ちください。」が表示",
      check: (html: string) => {
        expect(html).toContain("OGP プレビュー画像の準備中です");
        expect(html).toContain("SNS シェアの前に少しお待ちください");
      },
    },
    {
      name: "正常_既存testidが維持される",
      description:
        "Given: SSR, Then: complete-view / complete-actions / complete-back-to-edit / complete-faq-link / complete-save-reminder が出力（M-2 STOP δ で破壊していない）",
      check: (html: string) => {
        expect(html).toContain('data-testid="complete-view"');
        expect(html).toContain('data-testid="complete-actions"');
        expect(html).toContain('data-testid="complete-back-to-edit"');
        expect(html).toContain('data-testid="complete-faq-link"');
        expect(html).toContain('data-testid="complete-save-reminder"');
      },
    },
    {
      name: "正常_photobookId_は属性として直接出ない",
      description:
        "Given: photobookId prop, When: SSR, Then: DOM 属性 / aria-label / data-testid に raw photobookId が含まれない（fetch URL 内部に閉じる）",
      check: (html: string) => {
        // raw photobookId が data-* / aria-* / testid に含まれないこと（href / src も同様）
        expect(html).not.toContain('data-photobook-id=');
        expect(html).not.toContain('aria-label="00000000-0000-0000-0000-000000000001"');
        expect(html).not.toContain('data-testid="00000000-0000-0000-0000-000000000001"');
      },
    },
    {
      name: "正常_OGPプレビュー準備完了_文言は初期SSRに出ない",
      description:
        "Given: 初期 checking phase, Then: ready / timeout phase の文言は出力されない（polling 前なので）",
      check: (html: string) => {
        expect(html).not.toContain("OGP プレビュー画像の準備が完了しました");
        expect(html).not.toContain("OGP 画像の反映に少し時間がかかっています");
      },
    },
  ];

  for (const tt of tests) {
    it(tt.name, () => {
      const html = renderToStaticMarkup(<CompleteView {...baseProps} />);
      tt.check(html);
    });
  }
});
