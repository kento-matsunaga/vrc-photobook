// PublishSettingsPanel SSR rendering test。
//
// 観点 (2026-05-03 STOP α P0 v2):
//   - 権利・配慮確認 checkbox が描画される
//   - 初期状態（未チェック）で「公開へ進む」が disabled
//   - publish-rights-required-hint が出る
//   - dirty / publishDisabledReason 既存条件は維持
//
// React Testing Library を入れていないので click test は EditClient.publish.test.ts
// 側の structural guard で代替。本ファイルは SSR markup の構造検証のみ。

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PublishSettingsPanel } from "@/components/Edit/PublishSettingsPanel";
import type { EditSettings } from "@/lib/editPhotobook";

const baseSettings: EditSettings = {
  type: "memory",
  title: "テスト",
  layout: "simple",
  openingStyle: "light",
  visibility: "unlisted",
};

const noopSave = async () => undefined;

describe("PublishSettingsPanel 同意 checkbox (P0 v2)", () => {
  it("正常_publish-rights-agreed checkbox が描画される", () => {
    const html = renderToStaticMarkup(
      <PublishSettingsPanel
        initial={baseSettings}
        onSave={noopSave}
        onPublish={async () => undefined}
      />,
    );
    expect(html).toContain('data-testid="publish-rights-agreed"');
    expect(html).toContain("写っている人やアバター、");
    expect(html).toContain("配慮した内容であることを確認しました");
  });

  it("正常_初期状態で「公開へ進む」が disabled、rights-required-hint が出る", () => {
    const html = renderToStaticMarkup(
      <PublishSettingsPanel
        initial={baseSettings}
        onSave={noopSave}
        onPublish={async () => undefined}
      />,
    );
    const buttonMatch = html.match(/<button[^>]*data-testid="publish-button"[^>]*>/);
    expect(buttonMatch).not.toBeNull();
    expect(buttonMatch?.[0] ?? "").toMatch(/disabled=""/);
    expect(html).toContain('data-testid="publish-rights-required-hint"');
    expect(html).toContain("権利・配慮確認への同意が必要です");
  });

  it("正常_publishDisabledReason 既存条件 + rights checkbox は両立して disabled", () => {
    const html = renderToStaticMarkup(
      <PublishSettingsPanel
        initial={baseSettings}
        publishDisabledReason="公開には最低 1 枚の写真が必要です。"
        onSave={noopSave}
        onPublish={async () => undefined}
      />,
    );
    const buttonMatch = html.match(/<button[^>]*data-testid="publish-button"[^>]*>/);
    expect(buttonMatch?.[0] ?? "").toMatch(/disabled=""/);
    expect(html).toContain("公開には最低 1 枚の写真が必要です");
  });

  it("正常_onPublish 不在時は placeholder ボタン", () => {
    const html = renderToStaticMarkup(
      <PublishSettingsPanel initial={baseSettings} onSave={noopSave} />,
    );
    // onPublish 不在時は data-testid="publish-button" は出ない（placeholder）。
    expect(html).not.toContain('data-testid="publish-button"');
    expect(html).not.toContain('data-testid="publish-rights-agreed"');
  });

  it("正常_β-4_Q-A_rights_main_label_と_helper_text_両方表示", () => {
    const html = renderToStaticMarkup(
      <PublishSettingsPanel
        initial={baseSettings}
        onSave={noopSave}
        onPublish={async () => undefined}
      />,
    );
    // main label (design 短文)
    expect(html).toContain("権利・配慮について確認しました");
    // helper text (production 長文、既存既存維持)
    expect(html).toContain("投稿する画像について必要な権利・許可を確認");
    expect(html).toContain("写っている人やアバター、");
    expect(html).toContain("配慮した内容であることを確認しました");
  });

  it("正常_β-4_Q-B_settings-save_label_は_下書き保存", () => {
    const html = renderToStaticMarkup(
      <PublishSettingsPanel
        initial={baseSettings}
        onSave={noopSave}
        onPublish={async () => undefined}
      />,
    );
    const saveBtn = html.match(/<button[^>]*data-testid="settings-save"[^>]*>([^<]+)</);
    expect(saveBtn).not.toBeNull();
    // dirty=false 初期は「下書き保存」、saving 中は「保存中…」
    expect(saveBtn?.[1] ?? "").toBe("下書き保存");
  });

  it("正常_β-4_publish_button_label_は_公開へ進む_を維持", () => {
    const html = renderToStaticMarkup(
      <PublishSettingsPanel
        initial={baseSettings}
        onSave={noopSave}
        onPublish={async () => undefined}
      />,
    );
    const publishBtn = html.match(/<button[^>]*data-testid="publish-button"[^>]*>([^<]+)</);
    expect(publishBtn).not.toBeNull();
    expect(publishBtn?.[1] ?? "").toBe("公開へ進む");
  });
});
