// EditClient publish 経路 (P0 v2) の構造 guard test。
//
// 観点:
//   - publish 関連 import に publishPhotobook を保持
//   - onPublish が引数 rightsAgreed: boolean を受け取る形に変わっている
//   - publishPhotobook(view.photobookId, view.version, rightsAgreed) 呼び出しが入っている
//   - publish_precondition_failed reason 別文言が source に存在
//   - 「最新を取得」CTA は version_conflict 時のみ
//   - 旧来「公開条件に合致しません。最新を取得して再度確認してください。」固定文言が消えている
//
// frontend に DOM testing library が無いため、interactive click test は組まない
// （実 fetch / state は publishPhotobook.test.ts / PublishSettingsPanel.test.tsx で個別検証済み）。

import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const SRC_PATH = resolve(__dirname, "../EditClient.tsx");

describe("EditClient publish 経路 (P0 v2 hotfix)", () => {
  const src = readFileSync(SRC_PATH, "utf-8");

  it("正常_publishPhotobook を 3 引数呼出に切替えている", () => {
    expect(src).toMatch(
      /publishPhotobook\(\s*view\.photobookId\s*,\s*view\.version\s*,\s*rightsAgreed\s*\)/,
    );
  });

  it("正常_onPublish callback が rightsAgreed: boolean を受ける", () => {
    expect(src).toMatch(/async\s*\(\s*rightsAgreed\s*:\s*boolean\s*\)/);
  });

  it("正常_publish_precondition_failed reason 別文言が出る", () => {
    expect(src).toContain('case "rights_not_agreed":');
    expect(src).toContain('case "not_draft":');
    expect(src).toContain('case "empty_creator":');
    expect(src).toContain('case "empty_title":');
    expect(src).toContain('case "unknown_precondition":');
    // 各 reason の代表文言
    expect(src).toContain("権利・配慮確認への同意が必要です");
    expect(src).toContain("既に公開済み");
    expect(src).toContain("作者名が未設定");
    expect(src).toContain("タイトルを入力してください");
  });

  it("正常_version_conflict のみ「最新を取得」案内、precondition_failed では出さない", () => {
    // version_conflict 経路では「最新を取得して再度公開してください」を出す
    expect(src).toMatch(/最新を取得して再度公開/);
    // 旧固定文言「公開条件に合致しません。最新を取得して再度確認してください。」は撤去
    expect(src).not.toContain("公開条件に合致しません。最新を取得して再度確認してください。");
  });

  it("正常_publish_precondition_failed 経路では setConflict('conflict') にしない", () => {
    // setConflict("conflict") は version_conflict ブランチに限定。
    // publish_precondition_failed ブランチでは setConflict("ok") を使う。
    const preconditionBlock = src.match(
      /publish_precondition_failed[\s\S]*?setConflict\("ok"\)/,
    );
    expect(preconditionBlock).not.toBeNull();
  });
});
