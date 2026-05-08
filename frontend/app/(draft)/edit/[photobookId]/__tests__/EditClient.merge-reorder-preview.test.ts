// EditClient の merge / page reorder / preview 配線 structural guard test (STOP P-6)。
//
// 観点:
//   - import に mergePages / reorderPages / PreviewPane / PreviewToggle が含まれる
//   - mergePages 呼出: source = page.pageId、target = view.pages[idx-1].pageId
//   - reorderPages 呼出: 全 page assignments + 新 displayOrder で送る
//   - 成功時はいずれも setView(...) で EditView 全体反映 (B 方式)
//   - mode state (ViewMode) が edit / preview を切替える
//   - preview mode では PreviewPane を render し、edit UI を出さない
//   - PageBlock に onMergeIntoPrev / onPageMoveUp / onPageMoveDown を渡している
//   - reason 別 UI 文言は出さない (既存 handleApiError 経由のみ)

import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const SRC_PATH = resolve(__dirname, "../EditClient.tsx");

describe("EditClient merge / page reorder / preview 配線 (STOP P-6)", () => {
  const src = readFileSync(SRC_PATH, "utf-8");

  it("正常_lib から mergePages / reorderPages を import", () => {
    expect(src).toMatch(/import\s*{[^}]*\bmergePages\b/s);
    expect(src).toMatch(/import\s*{[^}]*\breorderPages\b/s);
  });

  it("正常_PreviewPane と PreviewToggle を import", () => {
    expect(src).toMatch(/from "@\/components\/Edit\/PreviewPane"/);
    expect(src).toMatch(/from "@\/components\/Edit\/PreviewToggle"/);
  });

  it("正常_mode state を ViewMode 型で持つ", () => {
    expect(src).toMatch(/useState<ViewMode>\("edit"\)/);
  });

  it("正常_preview mode では PreviewPane を render して early return", () => {
    // mode === "preview" の早期 return ブロックに PreviewPane が含まれる
    const block = src.match(/if\s*\(\s*mode\s*===\s*"preview"\s*\)[\s\S]*?<PreviewPane/);
    expect(block).not.toBeNull();
  });

  it("正常_PreviewToggle が header に配置される (edit mode 時)", () => {
    // edit mode の header に <PreviewToggle ... /> が出る
    expect(src).toMatch(/<PreviewToggle\b[\s\S]*?onToggle=\{\(\)\s*=>\s*setMode\("preview"\)\}/);
  });

  it("正常_mergePages 呼出_4 引数 (photobookId, sourcePageId, targetPageId, version)", () => {
    expect(src).toMatch(
      /mergePages\(\s*view\.photobookId\s*,\s*page\.pageId\s*,\s*target\.pageId\s*,\s*view\.version\s*\)/,
    );
  });

  it("正常_merge 成功時_setView(next) で EditView 全体を反映 (B 方式)", () => {
    const block = src.match(/onMergeIntoPrev[\s\S]*?const\s+next\s*=\s*await\s+mergePages[\s\S]*?setView\(next\)/);
    expect(block).not.toBeNull();
  });

  it("正常_reorderPages 呼出_3 引数 (photobookId, assignments, version)", () => {
    expect(src).toMatch(
      /reorderPages\(\s*view\.photobookId\s*,\s*assignments\s*,\s*view\.version\s*\)/,
    );
  });

  it("正常_reorderPages の assignments は全 page から構築 (pageId + displayOrder=i の permutation)", () => {
    // adjacent swap で全 page の assignments を作る
    expect(src).toMatch(/assignments\s*=\s*next\.map\(\(p,\s*i\)\s*=>\s*\(\{\s*pageId:\s*p\.pageId\s*,\s*displayOrder:\s*i\s*\}\)\)/);
  });

  it("正常_reorder 成功時_setView(res) で EditView 全体を反映 (B 方式)", () => {
    const block = src.match(/swapPagesAndReorder[\s\S]*?const\s+res\s*=\s*await\s+reorderPages[\s\S]*?setView\(res\)/);
    expect(block).not.toBeNull();
  });

  it("正常_PageBlock に onMergeIntoPrev / onPageMoveUp / onPageMoveDown を渡している", () => {
    expect(src).toMatch(/<PageBlock\b[\s\S]*?onMergeIntoPrev=/);
    expect(src).toMatch(/<PageBlock\b[\s\S]*?onPageMoveUp=/);
    expect(src).toMatch(/<PageBlock\b[\s\S]*?onPageMoveDown=/);
  });

  it("正常_merge / reorder ともに handleApiError を経由 (汎用エラー = reason 別 UI 文言なし)", () => {
    const mergeBlock = src.match(/onMergeIntoPrev[\s\S]*?handleApiError\(e\)/);
    expect(mergeBlock).not.toBeNull();
    const reorderBlock = src.match(/swapPagesAndReorder[\s\S]*?handleApiError\(e\)/);
    expect(reorderBlock).not.toBeNull();
  });

  it("正常_merge / reorder で reason 別 文言 (merge_into_self 等) を Frontend に持ち込まない", () => {
    // reason key を switch case / 文字列比較として handling 化していないことを確認。
    // コメントでの参照 (UI 防御理由の説明) は許容。
    // 禁止パターン: case "merge_into_self": / e.reason === "merge_into_self" 等
    const forbiddenPatterns: RegExp[] = [
      /case\s+"merge_into_self"/,
      /case\s+"cannot_remove_last_page"/,
      /case\s+"invalid_reorder_assignments"/,
      /e\.reason\s*===\s*"merge_into_self"/,
      /e\.reason\s*===\s*"cannot_remove_last_page"/,
      /e\.reason\s*===\s*"invalid_reorder_assignments"/,
    ];
    for (const re of forbiddenPatterns) {
      expect(src).not.toMatch(re);
    }
  });

  it("正常_SSR 用 fetchEditView を Client Component から呼ばない (.agents/rules/client-vs-ssr-fetch.md)", () => {
    const re = /\bfetchEditView\s*\(/g;
    let m: RegExpExecArray | null;
    while ((m = re.exec(src)) !== null) {
      const after = src.slice(m.index + "fetchEditView".length, m.index + "fetchEditView".length + 8);
      if (after.startsWith("Client(") || after.startsWith("Client ")) continue;
      throw new Error(`EditClient で fetchEditView() を直接呼んでいる: offset ${m.index}`);
    }
    expect(true).toBe(true);
  });
});
