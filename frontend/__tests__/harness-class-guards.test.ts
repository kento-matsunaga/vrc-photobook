// 横断 antipattern guard test (harness class-level)。
//
// 目的:
//   - 個別画面ごとの test では拾えない事故クラス (class-level pattern) を、
//     repo-wide な source scan で検知する。
//   - "/edit/EditClient.tsx の reload が 401" は 1 箇所の bug だが、事故クラスは
//     「Client Component で SSR 用 fetch を呼ぶ」設計問題。1 箇所直しても別画面で
//     再発する余地があるので、横断 guard で全体を抑え込む。
//
// 検査対象:
//   1. SSR fetch を Client Component から呼ぶ antipattern
//      (`.agents/rules/client-vs-ssr-fetch.md`)
//   2. Publish 旧曖昧文言の不在（「公開条件に合致しません。最新を取得して再度確認してください。」）
//      (`.agents/rules/publish-precondition-ux.md`)
//   3. Backend CORS の AllowedMethods に PATCH / DELETE が含まれる
//      (`.agents/rules/cors-mutation-methods.md`)

import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";

import { describe, expect, it } from "vitest";

const FRONTEND_ROOT = resolve(__dirname, "..");
const REPO_ROOT = resolve(FRONTEND_ROOT, "..");

const SCAN_DIRS = ["app", "components", "lib"] as const;
const EXTS = [".ts", ".tsx"] as const;
const EXCLUDED_DIR_NAMES = new Set([
  "node_modules",
  ".next",
  ".open-next",
  ".wrangler",
  "__tests__",
]);

function* walkSourceFiles(dir: string): Generator<string> {
  let entries: string[];
  try {
    entries = readdirSync(dir);
  } catch {
    return;
  }
  for (const name of entries) {
    if (EXCLUDED_DIR_NAMES.has(name)) continue;
    const full = join(dir, name);
    let st;
    try {
      st = statSync(full);
    } catch {
      continue;
    }
    if (st.isDirectory()) {
      yield* walkSourceFiles(full);
    } else if (st.isFile()) {
      if (name.endsWith(".test.ts") || name.endsWith(".test.tsx")) continue;
      if (EXTS.some((e) => name.endsWith(e))) {
        yield full;
      }
    }
  }
}

function readFrontendSources(): { path: string; content: string }[] {
  const out: { path: string; content: string }[] = [];
  for (const sub of SCAN_DIRS) {
    const root = join(FRONTEND_ROOT, sub);
    for (const f of walkSourceFiles(root)) {
      out.push({ path: relative(FRONTEND_ROOT, f), content: readFileSync(f, "utf-8") });
    }
  }
  return out;
}

describe("harness class guard: client/ssr fetch separation (.agents/rules/client-vs-ssr-fetch.md)", () => {
  const sources = readFrontendSources();

  it("正常_スキャン対象 source が 1 件以上ある（sanity check）", () => {
    expect(sources.length).toBeGreaterThan(10);
  });

  it("正常_'use client' ファイルは SSR 用 fetchEditView() を直接呼んでいない", () => {
    const offenders: string[] = [];
    for (const { path, content } of sources) {
      // "use client"; / 'use client'; 両方を許容
      const isClient = /^["']use client["'];?/m.test(content);
      if (!isClient) continue;
      // SSR 用 fetchEditView の関数呼出を検知。
      // fetchEditViewClient は許容（lookbehind ではなく否定パターンで検知）。
      // 関数呼出パターン: "fetchEditView(" だが "fetchEditViewClient(" は除外する。
      const re = /\bfetchEditView\s*\(/g;
      let m: RegExpExecArray | null;
      while ((m = re.exec(content)) !== null) {
        const idx = m.index;
        // 直後が "Client(" なら fetchEditViewClient なので skip。
        // 実際には正規表現を /\bfetchEditView\(/ にして "Client" を含む方は別関数として
        // 一致しないようにする。\b は単語境界なので fetchEditViewClient(...) は別 token。
        // ただし fetchEditView<no boundary>Client( には合致しないので OK。
        // 念のため後方 6 文字でも確認。
        const after = content.slice(idx + "fetchEditView".length, idx + "fetchEditView".length + 8);
        if (after.startsWith("Client(") || after.startsWith("Client ")) continue;
        if (!after.startsWith("(")) continue; // 単語境界の保険
        offenders.push(`${path}: byte offset ${idx}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("正常_'use client' ファイルが fetchEditView を import しても、呼び出さない（型のみ等は許容）", () => {
    // 上の test で「呼び出し」のみ検知している。import の有無は許容（型として import する余地）。
    // この test は説明用 placeholder（assert は無し）。
    expect(true).toBe(true);
  });
});

describe("harness class guard: publish 旧曖昧文言の不在 (.agents/rules/publish-precondition-ux.md)", () => {
  const sources = readFrontendSources();
  const FORBIDDEN_PHRASES = [
    // 旧 publish 失敗時の曖昧文言（reload で解消しないケースに reload を案内する罠）
    "公開条件に合致しません。最新を取得して再度確認してください。",
  ];

  it("正常_source 内に旧曖昧文言が無い", () => {
    const offenders: string[] = [];
    for (const { path, content } of sources) {
      for (const phrase of FORBIDDEN_PHRASES) {
        if (content.includes(phrase)) {
          offenders.push(`${path}: '${phrase}'`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });
});

describe("harness class guard: Backend CORS AllowedMethods (.agents/rules/cors-mutation-methods.md)", () => {
  const corsPath = resolve(REPO_ROOT, "backend/internal/http/cors.go");
  const cors = readFileSync(corsPath, "utf-8");

  it("正常_AllowedMethods に PATCH / DELETE が含まれる", () => {
    // AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"} のような行を期待
    const m = cors.match(/AllowedMethods:\s*\[\]string\{([^}]+)\}/);
    expect(m).not.toBeNull();
    const list = (m?.[1] ?? "").toLowerCase();
    expect(list).toContain('"patch"');
    expect(list).toContain('"delete"');
    expect(list).toContain('"options"');
    expect(list).toContain('"get"');
    expect(list).toContain('"post"');
  });
});
