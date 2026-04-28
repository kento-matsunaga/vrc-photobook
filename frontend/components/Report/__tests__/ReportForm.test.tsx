// ReportForm のレンダリングテスト。
//
// 方針:
//   - SSR 経路（renderToStaticMarkup）で初期 HTML を検証
//   - インタラクション（state 切替・submit）は本テストでは扱わず、別テストや実機で確認
//   - reporter_contact / detail / Turnstile token の値が初期描画に出ないこと

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { ReportForm } from "@/components/Report/ReportForm";

describe("ReportForm 初期描画", () => {
  it("正常_form要素_必要項目を含む", () => {
    const html = renderToStaticMarkup(
      <ReportForm slug="uqfwfti7glarva5saj" turnstileSiteKey="dummy-site-key" />,
    );
    expect(html).toContain('data-testid="report-form"');
    // 必須要素
    expect(html).toContain("通報理由");
    expect(html).toContain("詳細（任意）");
    expect(html).toContain("連絡先（任意）");
    // 注意文
    expect(html).toContain("個人情報・URL・他者の連絡先");
    expect(html).toContain("通報対応のためにのみ使用");
    // submit ボタンは Turnstile 未通過のため disabled
    expect(html).toContain('data-testid="report-submit-button"');
    expect(html).toContain('disabled=""');
    // thanks view ではない
    expect(html).not.toContain('data-testid="report-thanks-view"');
  });

  it("正常_reason_select_に_minor_safety_concern_含む", () => {
    const html = renderToStaticMarkup(
      <ReportForm slug="uqfwfti7glarva5saj" turnstileSiteKey="dummy-site-key" />,
    );
    expect(html).toContain('value="minor_safety_concern"');
    expect(html).toContain('value="harassment_or_doxxing"');
    expect(html).toContain('value="unauthorized_repost"');
    expect(html).toContain('value="subject_removal_request"');
    expect(html).toContain('value="sensitive_flag_missing"');
    expect(html).toContain('value="other"');
  });

  it("正常_report_id_は初期描画に含まれない", () => {
    // PR35a §16 #7: thanks view で report_id を表示しない方針。
    // 初期描画の form にも当然 report_id は含まれない。
    const html = renderToStaticMarkup(
      <ReportForm slug="uqfwfti7glarva5saj" turnstileSiteKey="dummy-site-key" />,
    );
    expect(html).not.toContain("report_id");
    expect(html).not.toContain("reportId");
  });

  it("正常_Turnstile_widget_aria_label_存在", () => {
    const html = renderToStaticMarkup(
      <ReportForm slug="uqfwfti7glarva5saj" turnstileSiteKey="dummy-site-key" />,
    );
    expect(html).toContain('aria-label="Turnstile widget"');
  });
});
