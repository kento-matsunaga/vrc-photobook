// CreateClient の SSR レンダリング検証。
//
// 観点:
//   - 7 種の type 選択肢が表示される
//   - 既定 type は memory
//   - title / creator_display_name 入力欄 + 文字数 counter
//   - 公開範囲は限定公開既定の説明 (wf-note 視覚)
//   - Turnstile widget placeholder
//   - submit ボタンは Turnstile 未通過で disabled
//   - Cookie / token / Secret が画面に出ない
//   - β-3: design 視覚 (wf-radio active class / wf-counter / wf-input focus / wf-note teal)

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { CreateClient } from "@/app/(public)/create/CreateClient";

describe("CreateClient 初期描画", () => {
  it("正常_form要素_必要項目を含む", () => {
    const html = renderToStaticMarkup(
      <CreateClient turnstileSiteKey="dummy-site-key" />,
    );
    expect(html).toContain('data-testid="create-form"');
    // type 選択肢 7 種
    expect(html).toContain('data-testid="create-type-memory"');
    expect(html).toContain('data-testid="create-type-event"');
    expect(html).toContain('data-testid="create-type-daily"');
    expect(html).toContain('data-testid="create-type-portfolio"');
    expect(html).toContain('data-testid="create-type-avatar"');
    expect(html).toContain('data-testid="create-type-world"');
    expect(html).toContain('data-testid="create-type-free"');
    // title / creator_display_name
    expect(html).toContain('id="create-title"');
    expect(html).toContain('id="create-creator"');
    // 注意文（限定公開既定）
    expect(html).toContain("限定公開");
    expect(html).toContain("公開操作は次のステップ以降");
    // submit ボタン disabled (Turnstile 未通過)
    expect(html).toContain('data-testid="create-submit-button"');
    expect(html).toContain('disabled=""');
  });

  it("正常_既定type_memoryが_selected_になる", () => {
    const html = renderToStaticMarkup(
      <CreateClient turnstileSiteKey="dummy-site-key" />,
    );
    // memory のラジオが checked
    expect(html).toMatch(
      /value="memory"[^>]*checked|checked[^>]*value="memory"/,
    );
  });

  it("正常_Turnstile_widget_placeholder_を含む", () => {
    const html = renderToStaticMarkup(
      <CreateClient turnstileSiteKey="dummy-site-key" />,
    );
    expect(html).toContain("送信前の bot 検証が必要です");
  });

  it("正常_Cookie_token_Secret_が初期描画に出ない", () => {
    const html = renderToStaticMarkup(
      <CreateClient turnstileSiteKey="dummy-site-key" />,
    );
    expect(html).not.toMatch(/draft_edit_token=/i);
    expect(html).not.toMatch(/manage_url_token=/i);
    expect(html).not.toMatch(/session_token=/i);
    expect(html).not.toMatch(/Set-Cookie/i);
  });

  it("正常_β-3_design_wf-radio_active_状態_と_wf-input_focus_と_wf-note_視覚_を持つ", () => {
    const html = renderToStaticMarkup(
      <CreateClient turnstileSiteKey="dummy-site-key" />,
    );
    // memory active radio: 開きタグ全体を抽出して active-only class を assert
    // (React は class / data-testid の attribute 順を保証しないため、
    // ラベルタグ全体を string match してから個別 contain を確認)
    const memoryLabel = html.match(
      /<label[^>]*data-testid="create-type-memory"[^>]*>/,
    );
    expect(memoryLabel).not.toBeNull();
    expect(memoryLabel?.[0] ?? "").toContain("border-teal-500");
    expect(memoryLabel?.[0] ?? "").toContain("bg-teal-50");
    // 他 type は active class を持たない (例: event)
    const eventLabel = html.match(
      /<label[^>]*data-testid="create-type-event"[^>]*>/,
    );
    expect(eventLabel).not.toBeNull();
    expect(eventLabel?.[0] ?? "").not.toContain("border-teal-500");
    // wf-input style の class が出る
    expect(html).toContain("focus:border-teal-400");
    expect(html).toContain("outline-teal-200");
    // wf-note 視覚: border-l teal-300 + bg teal-50 + i icon teal-500
    expect(html).toContain("border-teal-300");
    expect(html).toContain("bg-teal-500");
    // wf-counter (font-num + text-[10.5px])
    expect(html).toContain("0 / 100");
    expect(html).toContain("0 / 50");
  });
});
