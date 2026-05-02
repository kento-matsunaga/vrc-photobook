// EditClient.tsx の reload() 経路に対する構造 guard test。
//
// 観点:
//   - 2026-05-03 STOP α P0-β hotfix: reload() は browser からの polling /
//     「最新を取得」ボタン経路。credentials:"include" を使う fetchEditViewClient
//     が必須で、SSR 用 fetchEditView(..., "") を使うと cross-origin で Cookie が
//     送られず 401 になる。
//
// frontend には DOM testing library が無いため、useEffect / Click を実際に
// 走らせる integration test は組まない。代わりに source file 上で
//   - fetchEditViewClient が import されている
//   - SSR 用 fetchEditView(view.photobookId, "") パターンが存在しない
// ことを regex 検証する（regression guard）。
//
// fetchEditViewClient 自体の credentials:"include" 動作は
// lib/__tests__/editPhotobook.test.ts で fetch spy ベースで検証済み。

import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const SRC_PATH = resolve(__dirname, "../EditClient.tsx");

describe("EditClient.reload (P0-β hotfix)", () => {
  const src = readFileSync(SRC_PATH, "utf-8");

  it("正常_fetchEditViewClient を import している", () => {
    expect(src).toMatch(/import\s*{[^}]*fetchEditViewClient[^}]*}\s*from\s*"@\/lib\/editPhotobook"/);
  });

  it("正常_SSR 用 fetchEditView(空 Cookie) パターンを使っていない", () => {
    // 旧パターン: await fetchEditView(view.photobookId, "")
    // これが残っていると cross-origin で Cookie が送られず polling が常に 401 になる。
    expect(src).not.toMatch(/fetchEditView\(\s*[a-zA-Z_.]+\s*,\s*""\s*\)/);
  });

  it("正常_reload() 内で fetchEditViewClient(...) が呼ばれている", () => {
    // reload の本体に fetchEditViewClient(view.photobookId) 呼び出しがあること。
    expect(src).toMatch(/fetchEditViewClient\(\s*view\.photobookId\s*\)/);
  });
});
