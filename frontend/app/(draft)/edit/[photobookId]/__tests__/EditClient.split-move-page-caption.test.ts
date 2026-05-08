// EditClient の split / move / page caption 配線の structural guard test (STOP P-5)。
//
// 観点:
//   - import に updatePageCaption / splitPage / movePhoto が含まれる
//   - PageBlock を render する (古い PhotoGrid 直接 render は撤去)
//   - page caption: A 方式 = res.version を view.version に反映 (bumpVersion ではなく
//     setView({...v, version: res.version}))
//   - split / move: B 方式 = setView(next) で EditView 全体を反映
//   - reason 別 UI 文言は P-5 で出さない (kind ベースのみ、handleApiError 経由)
//   - splitDisabledReasonOf: 30 page 上限 / 末尾 photo の reason を出している
//   - .agents/rules/client-vs-ssr-fetch.md 準拠: SSR 用 fetchEditView を Client から呼ばない

import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const SRC_PATH = resolve(__dirname, "../EditClient.tsx");

describe("EditClient split / move / page caption 配線 (STOP P-5)", () => {
  const src = readFileSync(SRC_PATH, "utf-8");

  it("正常_lib から updatePageCaption / splitPage / movePhoto を import", () => {
    expect(src).toMatch(/import\s*{[^}]*\bupdatePageCaption\b/s);
    expect(src).toMatch(/import\s*{[^}]*\bsplitPage\b/s);
    expect(src).toMatch(/import\s*{[^}]*\bmovePhoto\b/s);
  });

  it("正常_PageBlock を import しており、PhotoGrid 直接 import は撤去", () => {
    expect(src).toMatch(/from "@\/components\/Edit\/PageBlock"/);
    // EditClient 内では PageBlock 経由なので PhotoGrid を直接 import しない
    expect(src).not.toMatch(/from "@\/components\/Edit\/PhotoGrid"/);
  });

  it("正常_updatePageCaption 呼出_4 引数 (photobookId, pageId, caption, version)", () => {
    expect(src).toMatch(
      /updatePageCaption\(\s*view\.photobookId\s*,\s*pageId\s*,\s*caption\s*,\s*view\.version\s*\)/,
    );
  });

  it("正常_page caption A 方式: setView で res.version と pages を更新", () => {
    // res.version を view.version に反映
    expect(src).toMatch(/version:\s*res\.version/);
    // 該当 page の caption を反映 (caption ?? undefined)
    expect(src).toMatch(/caption:\s*caption\s*\?\?\s*undefined/);
  });

  it("正常_splitPage 呼出_4 引数 (photobookId, pageId, splitAtPhotoId, version)", () => {
    expect(src).toMatch(
      /splitPage\(\s*view\.photobookId\s*,\s*pageId\s*,\s*splitAtPhotoId\s*,\s*view\.version\s*\)/,
    );
  });

  it("正常_movePhoto 呼出_5 引数 (photobookId, photoId, targetPageId, position, version)", () => {
    expect(src).toMatch(
      /movePhoto\(\s*view\.photobookId\s*,\s*photoId\s*,\s*targetPageId\s*,\s*position\s*,\s*view\.version\s*\)/,
    );
  });

  it("正常_split / move 成功時_setView(next) で EditView 全体を反映 (B 方式)", () => {
    // splitPage / movePhoto の戻り値を直接 setView に渡している
    expect(src).toMatch(/const\s+next\s*=\s*await\s+splitPage\([\s\S]*?\)/);
    expect(src).toMatch(/const\s+next\s*=\s*await\s+movePhoto\([\s\S]*?\)/);
    // setView(next) が両方の handler に存在
    const splitBlock = src.match(/onSplitPage[\s\S]*?setView\(next\)/);
    expect(splitBlock).not.toBeNull();
    const moveBlock = src.match(/onMovePhoto[\s\S]*?setView\(next\)/);
    expect(moveBlock).not.toBeNull();
  });

  it("正常_PageBlock に splitDisabledReasonOf / onSplit / onMovePhoto を渡している", () => {
    expect(src).toMatch(/<PageBlock\b[\s\S]*?splitDisabledReasonOf=/);
    expect(src).toMatch(/<PageBlock\b[\s\S]*?onSplit=/);
    expect(src).toMatch(/<PageBlock\b[\s\S]*?onMovePhoto=/);
  });

  it("正常_splitDisabledReasonOf_30 page 上限 / 末尾 photo の reason を返す", () => {
    expect(src).toContain("ページ数が上限 (30) に達しています");
    expect(src).toContain("末尾の写真ではページを分けられません");
  });

  it("正常_3 mutation handler とも handleApiError を経由する (汎用エラー)", () => {
    // reason 別 UI 文言は P-5 で出さない方針 (kind ベースのみ)
    const onPageCaptionBlock = src.match(/onPageCaptionSave[\s\S]*?handleApiError\(e\)/);
    expect(onPageCaptionBlock).not.toBeNull();
    const onSplitBlock = src.match(/onSplitPage[\s\S]*?handleApiError\(e\)/);
    expect(onSplitBlock).not.toBeNull();
    const onMoveBlock = src.match(/onMovePhoto\s*=[\s\S]*?handleApiError\(e\)/);
    expect(onMoveBlock).not.toBeNull();
  });

  it("正常_SSR 用 fetchEditView を Client Component から呼ばない (.agents/rules/client-vs-ssr-fetch.md)", () => {
    // Client Component (use client) で fetchEditView( を呼ぶのは禁止 (Client は fetchEditViewClient のみ)。
    // fetchEditView と fetchEditViewClient を区別するため "fetchEditView(" だけを単純検査して、
    // 直後が "Client(" でないか確認する。
    const re = /\bfetchEditView\s*\(/g;
    let m: RegExpExecArray | null;
    while ((m = re.exec(src)) !== null) {
      const after = src.slice(m.index + "fetchEditView".length, m.index + "fetchEditView".length + 8);
      // "fetchEditViewClient(" は OK
      if (after.startsWith("Client(") || after.startsWith("Client ")) continue;
      // それ以外で fetchEditView( を呼ぶのは違反
      throw new Error(`EditClient で fetchEditView() を直接呼んでいる: offset ${m.index}`);
    }
    // ここに到達 = 違反なし。
    expect(true).toBe(true);
  });
});
